package task

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ReviewMode string

const (
	ReviewFull  ReviewMode = "full"
	ReviewDelta ReviewMode = "delta"
)

type FindingSeverity string

const (
	FindingBlocking    FindingSeverity = "blocking"
	FindingNonBlocking FindingSeverity = "non_blocking"
)

type FindingStatus string

const (
	FindingOpen     FindingStatus = "open"
	FindingResolved FindingStatus = "resolved"
	FindingInvalid  FindingStatus = "invalid"
)

type ReviewFinding struct {
	ID          string          `json:"id"`
	Severity    FindingSeverity `json:"severity"`
	Status      FindingStatus   `json:"status"`
	Paths       []string        `json:"paths"`
	Invariant   string          `json:"invariant"`
	Evidence    string          `json:"evidence"`
	Disposition string          `json:"disposition"`
}

type ReviewBasis struct {
	PullRequestNumber int    `json:"pull_request_number"`
	BaseRef           string `json:"base_ref"`
	MergeBaseSHA      string `json:"merge_base_sha"`
}

type ReviewSegment struct {
	BaseSHA         string      `json:"base_sha"`
	HeadSHA         string      `json:"head_sha"`
	Mode            ReviewMode  `json:"mode"`
	SnapshotHash    string      `json:"snapshot_hash"`
	ScopeHash       string      `json:"scope_hash"`
	ReviewBasis     ReviewBasis `json:"review_basis"`
	ReviewReference string      `json:"review_reference"`
	CheckRuns       []CheckRun  `json:"check_runs"`
	InputBytes      int         `json:"input_bytes"`
	DurationMillis  int64       `json:"duration_millis"`
}

type ReviewEvent struct {
	EventID                 string          `json:"event_id"`
	ExpectedPreviousHeadSHA string          `json:"expected_previous_head_sha,omitempty"`
	RunID                   string          `json:"run_id"`
	LeaseEpoch              int64           `json:"lease_epoch,omitempty"`
	FencingToken            string          `json:"fencing_token,omitempty"`
	SourceActor             string          `json:"source_actor"`
	SourceServerTime        time.Time       `json:"source_server_time"`
	Segment                 ReviewSegment   `json:"segment"`
	Findings                []ReviewFinding `json:"findings"`
}

type ReviewChain struct {
	Segments          []ReviewSegment          `json:"segments"`
	Findings          map[string]ReviewFinding `json:"findings"`
	LastFindingNumber int                      `json:"last_finding_number,omitempty"`
	LastEventID       string                   `json:"last_event_id,omitempty"`
}

type ReviewStore interface {
	ReadReviewChain(context.Context, string, int) (ReviewChain, error)
	AppendReviewEvent(context.Context, string, int, ReviewEvent) (ReviewChain, error)
}

type ReviewEventInput struct {
	EventID            string
	RunID              string
	LeaseEpoch         int64
	FencingToken       string
	SourceServerTime   time.Time
	BaseSHA            string
	HeadSHA            string
	Mode               ReviewMode
	SnapshotHash       string
	ScopeHash          string
	ReviewBasis        ReviewBasis
	ReviewReference    string
	CheckRuns          []CheckRun
	InputBytes         int
	DurationMillis     int64
	Findings           []ReviewFinding
	assignedFindingIDs bool
}

var findingIDPattern = regexp.MustCompile(`^F-([0-9]{3,})$`)

func BuildReviewEvent(chain ReviewChain, input ReviewEventInput) (ReviewEvent, error) {
	if strings.TrimSpace(input.EventID) == "" || strings.TrimSpace(input.RunID) == "" {
		return ReviewEvent{}, errors.New("review event requires event_id and run_id")
	}
	if input.Mode != ReviewFull && input.Mode != ReviewDelta {
		return ReviewEvent{}, errors.New("review event mode must be full or delta")
	}
	for field, value := range map[string]string{
		"base SHA": input.BaseSHA, "head SHA": input.HeadSHA, "snapshot hash": input.SnapshotHash,
		"scope hash": input.ScopeHash, "review reference": input.ReviewReference,
	} {
		if strings.TrimSpace(value) == "" {
			return ReviewEvent{}, fmt.Errorf("review event requires %s", field)
		}
	}
	previousHead := reviewChainHead(chain)
	if input.HeadSHA == input.BaseSHA {
		return ReviewEvent{}, errors.New("review event must cover a changed HEAD")
	}
	if input.Mode == ReviewDelta && (previousHead == "" || input.BaseSHA != previousHead) {
		return ReviewEvent{}, errors.New("delta review must start at the previous reviewed HEAD")
	}
	if input.Mode == ReviewDelta {
		last := chain.Segments[len(chain.Segments)-1]
		if last.ReviewBasis != input.ReviewBasis || last.SnapshotHash != input.SnapshotHash || last.ScopeHash != input.ScopeHash {
			return ReviewEvent{}, errors.New("delta review basis, snapshot, and scope must match the previous segment")
		}
	}

	findings := make([]ReviewFinding, len(input.Findings))
	copy(findings, input.Findings)
	nextID := nextFindingID(chain)
	seen := make(map[string]struct{}, len(findings))
	for index := range findings {
		finding := &findings[index]
		providedID := strings.TrimSpace(finding.ID) != ""
		finding.Paths = append([]string(nil), finding.Paths...)
		sort.Strings(finding.Paths)
		if strings.TrimSpace(finding.ID) == "" {
			finding.ID = fmt.Sprintf("F-%03d", nextID)
			nextID++
		}
		if _, exists := seen[finding.ID]; exists {
			return ReviewEvent{}, fmt.Errorf("review finding %s is duplicated", finding.ID)
		}
		seen[finding.ID] = struct{}{}
		prior, exists := chain.Findings[finding.ID]
		if providedID && !exists {
			if !input.assignedFindingIDs || finding.ID != fmt.Sprintf("F-%03d", nextID) {
				return ReviewEvent{}, errors.New("new review finding must have an empty ID or the next controller-assigned ID during replay")
			}
			nextID++
		}
		if err := validateReviewFinding(*finding, prior, exists); err != nil {
			return ReviewEvent{}, err
		}
	}
	if input.Mode == ReviewFull && len(chain.Segments) > 0 {
		for id, finding := range chain.Findings {
			if finding.Status == FindingOpen {
				if _, exists := seen[id]; !exists {
					return ReviewEvent{}, fmt.Errorf("full review reset must disposition open finding %s", id)
				}
			}
		}
	}
	return ReviewEvent{
		EventID: input.EventID, ExpectedPreviousHeadSHA: previousHead, RunID: input.RunID,
		LeaseEpoch: input.LeaseEpoch, FencingToken: input.FencingToken,
		SourceActor: "runtime-review-controller", SourceServerTime: input.SourceServerTime,
		Segment: ReviewSegment{
			BaseSHA: input.BaseSHA, HeadSHA: input.HeadSHA, Mode: input.Mode,
			SnapshotHash: input.SnapshotHash, ScopeHash: input.ScopeHash, ReviewBasis: input.ReviewBasis,
			ReviewReference: input.ReviewReference, CheckRuns: append([]CheckRun(nil), input.CheckRuns...),
			InputBytes: input.InputBytes, DurationMillis: input.DurationMillis,
		},
		Findings: findings,
	}, nil
}

func ApplyReviewEvent(chain ReviewChain, event ReviewEvent) (ReviewChain, error) {
	input := ReviewEventInput{
		EventID: event.EventID, RunID: event.RunID, LeaseEpoch: event.LeaseEpoch, FencingToken: event.FencingToken,
		SourceServerTime: event.SourceServerTime, BaseSHA: event.Segment.BaseSHA, HeadSHA: event.Segment.HeadSHA,
		Mode: event.Segment.Mode, SnapshotHash: event.Segment.SnapshotHash, ScopeHash: event.Segment.ScopeHash,
		ReviewBasis: event.Segment.ReviewBasis, ReviewReference: event.Segment.ReviewReference, CheckRuns: event.Segment.CheckRuns,
		InputBytes: event.Segment.InputBytes, DurationMillis: event.Segment.DurationMillis, Findings: event.Findings, assignedFindingIDs: true,
	}
	rebuilt, err := BuildReviewEvent(chain, input)
	if err != nil {
		return ReviewChain{}, err
	}
	if event.ExpectedPreviousHeadSHA != rebuilt.ExpectedPreviousHeadSHA {
		return ReviewChain{}, ErrStateConflict
	}
	chain.LastFindingNumber = nextFindingID(chain) - 1
	if rebuilt.Segment.Mode == ReviewFull {
		chain.Findings = make(map[string]ReviewFinding)
	} else if chain.Findings == nil {
		chain.Findings = make(map[string]ReviewFinding)
	} else {
		cloned := make(map[string]ReviewFinding, len(chain.Findings))
		for id, finding := range chain.Findings {
			cloned[id] = finding
		}
		chain.Findings = cloned
	}
	if rebuilt.Segment.Mode == ReviewFull {
		chain.Segments = []ReviewSegment{rebuilt.Segment}
	} else {
		chain.Segments = append(append([]ReviewSegment(nil), chain.Segments...), rebuilt.Segment)
	}
	for _, finding := range rebuilt.Findings {
		chain.Findings[finding.ID] = finding
	}
	chain.LastFindingNumber = nextFindingID(chain) - 1
	chain.LastEventID = rebuilt.EventID
	return chain, nil
}

func ValidateReviewCoverage(chain ReviewChain, taskBaseSHA, finalHeadSHA, snapshotHash, scopeHash string) error {
	if len(chain.Segments) == 0 {
		return errors.New("done requires a canonical full review")
	}
	first := chain.Segments[0]
	if first.Mode != ReviewFull || first.BaseSHA != taskBaseSHA || (first.ReviewBasis.MergeBaseSHA != "" && first.ReviewBasis.MergeBaseSHA != taskBaseSHA) {
		return errors.New("review coverage must start with a full review from the task base")
	}
	previous := taskBaseSHA
	for index, segment := range chain.Segments {
		expectedMode := ReviewDelta
		if index == 0 {
			expectedMode = ReviewFull
		}
		if segment.BaseSHA != previous {
			return fmt.Errorf("review coverage has a gap before segment %d", index+1)
		}
		if segment.Mode != expectedMode || segment.ReviewBasis != first.ReviewBasis {
			return fmt.Errorf("review coverage segment %d has a different mode or review basis", index+1)
		}
		if segment.SnapshotHash != snapshotHash || segment.ScopeHash != scopeHash {
			return fmt.Errorf("review coverage segment %d is bound to a different snapshot or scope", index+1)
		}
		if strings.TrimSpace(segment.ReviewReference) == "" {
			return fmt.Errorf("review coverage segment %d has no reviewer run reference", index+1)
		}
		previous = segment.HeadSHA
	}
	if previous != finalHeadSHA {
		return fmt.Errorf("review coverage final HEAD %s does not match %s", previous, finalHeadSHA)
	}
	ids := make([]string, 0, len(chain.Findings))
	for id := range chain.Findings {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		finding := chain.Findings[id]
		if finding.Severity == FindingBlocking && finding.Status == FindingOpen {
			return fmt.Errorf("review finding %s is still blocking", id)
		}
	}
	return nil
}

type ReviewPlanInput struct {
	Chain        ReviewChain
	TaskBaseSHA  string
	HeadSHA      string
	SnapshotHash string
	ScopeHash    string
	ReviewBasis  ReviewBasis
}

type ReviewPlan struct {
	Mode         ReviewMode      `json:"mode"`
	BaseSHA      string          `json:"base_sha"`
	Reason       string          `json:"reason"`
	OpenFindings []ReviewFinding `json:"open_findings"`
}

func PlanReview(input ReviewPlanInput) (ReviewPlan, error) {
	for field, value := range map[string]string{"task base SHA": input.TaskBaseSHA, "HEAD SHA": input.HeadSHA, "snapshot hash": input.SnapshotHash, "scope hash": input.ScopeHash} {
		if strings.TrimSpace(value) == "" {
			return ReviewPlan{}, fmt.Errorf("review plan requires %s", field)
		}
	}
	reviewBase := input.TaskBaseSHA
	if input.ReviewBasis.MergeBaseSHA != "" {
		reviewBase = input.ReviewBasis.MergeBaseSHA
	}
	valid := len(input.Chain.Segments) > 0
	previous := reviewBase
	for index, segment := range input.Chain.Segments {
		expectedMode := ReviewDelta
		if index == 0 {
			expectedMode = ReviewFull
		}
		if segment.Mode != expectedMode || segment.BaseSHA != previous || segment.SnapshotHash != input.SnapshotHash || segment.ScopeHash != input.ScopeHash || segment.ReviewBasis != input.ReviewBasis {
			valid = false
			break
		}
		previous = segment.HeadSHA
	}
	if valid && input.HeadSHA == previous {
		return ReviewPlan{}, errors.New("current HEAD has already been reviewed")
	}
	if valid {
		return ReviewPlan{Mode: ReviewDelta, BaseSHA: previous, Reason: "review all changes since the previous reviewed HEAD", OpenFindings: openReviewFindings(input.Chain)}, nil
	}
	return ReviewPlan{Mode: ReviewFull, BaseSHA: reviewBase, Reason: "review basis or coverage changed; review the complete pull request change", OpenFindings: openReviewFindings(input.Chain)}, nil
}

func validateReviewFinding(finding, prior ReviewFinding, exists bool) error {
	if !findingIDPattern.MatchString(finding.ID) {
		return fmt.Errorf("review finding %q has an invalid stable ID", finding.ID)
	}
	if finding.Severity != FindingBlocking && finding.Severity != FindingNonBlocking {
		return fmt.Errorf("review finding %s has invalid severity", finding.ID)
	}
	if finding.Status != FindingOpen && finding.Status != FindingResolved && finding.Status != FindingInvalid {
		return fmt.Errorf("review finding %s has invalid status", finding.ID)
	}
	if len(finding.Paths) == 0 || strings.TrimSpace(finding.Invariant) == "" || strings.TrimSpace(finding.Evidence) == "" {
		return fmt.Errorf("review finding %s requires paths, invariant, and evidence", finding.ID)
	}
	for _, findingPath := range finding.Paths {
		candidate := strings.TrimSuffix(findingPath, "/")
		if !normalizedChangedPath(candidate) {
			return fmt.Errorf("review finding %s has a non-portable path", finding.ID)
		}
	}
	if !exists && finding.Status != FindingOpen {
		return fmt.Errorf("new review finding %s must start open", finding.ID)
	}
	if finding.Status != FindingOpen && strings.TrimSpace(finding.Disposition) == "" {
		return fmt.Errorf("review finding %s requires a disposition when closed", finding.ID)
	}
	if exists && (finding.Severity != prior.Severity || !reflect.DeepEqual(finding.Paths, prior.Paths) || finding.Invariant != prior.Invariant) {
		return fmt.Errorf("review finding %s identity cannot be rewritten", finding.ID)
	}
	return nil
}

func nextFindingID(chain ReviewChain) int {
	max := chain.LastFindingNumber
	for id := range chain.Findings {
		match := findingIDPattern.FindStringSubmatch(id)
		if len(match) != 2 {
			continue
		}
		value, _ := strconv.Atoi(match[1])
		if value > max {
			max = value
		}
	}
	return max + 1
}

func reviewChainHead(chain ReviewChain) string {
	if len(chain.Segments) == 0 {
		return ""
	}
	return chain.Segments[len(chain.Segments)-1].HeadSHA
}

func openReviewFindings(chain ReviewChain) []ReviewFinding {
	ids := make([]string, 0, len(chain.Findings))
	for id, finding := range chain.Findings {
		if finding.Status == FindingOpen {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	findings := make([]ReviewFinding, 0, len(ids))
	for _, id := range ids {
		findings = append(findings, chain.Findings[id])
	}
	return findings
}
