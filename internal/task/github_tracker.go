package task

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	readyMarkerPrefix = "<!-- sop-ready-v1\n"
	stateMarkerPrefix = "<!-- sop-state-v1\n"
	markerSuffix      = "\n-->"
)

var ErrNoReadyAttestation = errors.New("no trusted sop-ready-v1 attestation was found on the issue")

type ReadyEnvelope struct {
	Snapshot      Snapshot    `json:"snapshot"`
	Attestation   Attestation `json:"attestation"`
	StateRevision int64       `json:"state_revision"`
}

type GitHubRepository struct {
	NodeID        string `json:"node_id"`
	DefaultBranch string `json:"default_branch"`
	FullName      string `json:"full_name"`
	Permissions   struct {
		Push bool `json:"push"`
	} `json:"permissions"`
}

func (tracker GitHubTracker) PreflightWrite(ctx context.Context, repository GitHubRepository) error {
	if !repository.Permissions.Push {
		return errors.New("GitHub preflight cannot prove contents write permission")
	}
	var rulesets []json.RawMessage
	if err := tracker.apiJSON(ctx, nil, &rulesets, "repos/{owner}/{repo}/rulesets?includes_parents=true"); err != nil {
		return fmt.Errorf("GitHub preflight cannot inspect repository rulesets: %w", err)
	}
	return nil
}

func (tracker GitHubTracker) VerifyMergedPullRequest(ctx context.Context, repoNodeID, fullName, pullRequestURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(pullRequestURL))
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Host, "github.com") {
		return "", errors.New("completion pull request URL must be an https://github.com URL")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 || parts[0]+"/"+parts[1] != fullName || parts[2] != "pull" {
		return "", errors.New("completion pull request must belong to the current repository")
	}
	number, err := strconv.Atoi(parts[3])
	if err != nil || number <= 0 {
		return "", errors.New("completion pull request URL has an invalid number")
	}
	var pull struct {
		MergedAt *time.Time `json:"merged_at"`
		Head     struct {
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Repo struct {
				NodeID string `json:"node_id"`
			} `json:"repo"`
		} `json:"base"`
	}
	if err := tracker.apiJSON(ctx, nil, &pull, "repos/{owner}/{repo}/pulls/"+strconv.Itoa(number)); err != nil {
		return "", err
	}
	if pull.Base.Repo.NodeID != repoNodeID || pull.MergedAt == nil || strings.TrimSpace(pull.Head.SHA) == "" {
		return "", errors.New("completion pull request is not a merged PR in the current repository")
	}
	return pull.Head.SHA, nil
}

type GitHubActor struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

type githubIssue struct {
	Number  int    `json:"number"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
}

type GitHubTracker struct {
	RepositoryPath  string
	GHBinary        string
	TrustedActorIDs []int64
	Command         GitHubCommand
}

type GitHubCommand func(context.Context, []byte, ...string) (string, error)

func (tracker GitHubTracker) Repository(ctx context.Context) (GitHubRepository, error) {
	var repository GitHubRepository
	if err := tracker.apiJSON(ctx, nil, &repository, "repos/{owner}/{repo}"); err != nil {
		return GitHubRepository{}, err
	}
	if repository.NodeID == "" || repository.FullName == "" {
		return GitHubRepository{}, errors.New("GitHub repository identity is incomplete")
	}
	return repository, nil
}

func (tracker GitHubTracker) CurrentActor(ctx context.Context) (GitHubActor, error) {
	var actor GitHubActor
	if err := tracker.apiJSON(ctx, nil, &actor, "user"); err != nil {
		return GitHubActor{}, err
	}
	if actor.ID <= 0 {
		return GitHubActor{}, errors.New("GitHub actor identity is incomplete")
	}
	return actor, nil
}

func (tracker GitHubTracker) ApprovedTask(ctx context.Context, repoNodeID string, issueNumber int) (ReadyEnvelope, error) {
	comments, err := tracker.comments(ctx, issueNumber)
	if err != nil {
		return ReadyEnvelope{}, err
	}
	for index := len(comments) - 1; index >= 0; index-- {
		comment := comments[index]
		if !tracker.actorTrusted(comment.User.ID) {
			continue
		}
		var envelope ReadyEnvelope
		if !decodeMarkedJSON(comment.Body, readyMarkerPrefix, &envelope) {
			continue
		}
		createdAt, err := time.Parse(time.RFC3339, comment.CreatedAt)
		if err != nil {
			return ReadyEnvelope{}, fmt.Errorf("ready attestation comment has invalid GitHub time: %w", err)
		}
		if envelope.Attestation.ActorID != comment.User.ID || envelope.Attestation.ServerTime.IsZero() ||
			envelope.Attestation.ServerTime.Sub(createdAt).Abs() > 5*time.Minute {
			return ReadyEnvelope{}, errors.New("ready attestation identity or server time does not match its GitHub comment")
		}
		if envelope.Snapshot.RepoNodeID != repoNodeID || envelope.Snapshot.IssueNumber != issueNumber || envelope.StateRevision < 0 {
			return ReadyEnvelope{}, errors.New("ready attestation does not match the current repository, issue, or state revision")
		}
		return envelope, nil
	}
	return ReadyEnvelope{}, ErrNoReadyAttestation
}

func (tracker GitHubTracker) AttestCurrentIssue(
	ctx context.Context,
	repoNodeID string,
	issueNumber int,
	actorID int64,
	sopVersion string,
	stateRevision int64,
) (ReadyEnvelope, error) {
	if !tracker.actorTrusted(actorID) {
		return ReadyEnvelope{}, fmt.Errorf("current GitHub actor %d is not trusted by the project profile", actorID)
	}
	var issue githubIssue
	if err := tracker.apiJSON(ctx, nil, &issue, "repos/{owner}/{repo}/issues/"+strconv.Itoa(issueNumber)); err != nil {
		return ReadyEnvelope{}, err
	}
	snapshot, err := snapshotFromIssue(repoNodeID, issue)
	if err != nil {
		return ReadyEnvelope{}, err
	}
	hash, err := SnapshotHash(snapshot)
	if err != nil {
		return ReadyEnvelope{}, err
	}
	serverTime, _, err := tracker.ServerClock()(ctx)
	if err != nil {
		return ReadyEnvelope{}, err
	}
	envelope := ReadyEnvelope{
		Snapshot: snapshot,
		Attestation: Attestation{
			RepoNodeID: repoNodeID, IssueNumber: issueNumber, SnapshotHash: hash,
			ActorID: actorID, SOPVersion: sopVersion, ServerTime: serverTime,
		},
		StateRevision: stateRevision,
	}
	body, err := encodeMarkedJSON(readyMarkerPrefix, envelope)
	if err != nil {
		return ReadyEnvelope{}, err
	}
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return ReadyEnvelope{}, err
	}
	var comment githubComment
	if err := tracker.apiJSON(ctx, payload, &comment, "repos/{owner}/{repo}/issues/"+strconv.Itoa(issueNumber)+"/comments"); err != nil {
		return ReadyEnvelope{}, err
	}
	if comment.User.ID != actorID {
		return ReadyEnvelope{}, errors.New("GitHub created ready attestation under an unexpected actor")
	}
	return envelope, nil
}

func (tracker GitHubTracker) ReadState(ctx context.Context, repoNodeID string, issueNumber int) (IssueState, error) {
	ready, err := tracker.ApprovedTask(ctx, repoNodeID, issueNumber)
	if err != nil {
		return IssueState{}, err
	}
	state := IssueState{State: StateReady, Revision: ready.StateRevision}
	comments, err := tracker.comments(ctx, issueNumber)
	if err != nil {
		return IssueState{}, err
	}
	for _, comment := range comments {
		if !tracker.actorTrusted(comment.User.ID) {
			continue
		}
		var event StateEvent
		if !decodeMarkedJSON(comment.Body, stateMarkerPrefix, &event) {
			continue
		}
		if event.ExpectedPreviousRevision < state.Revision {
			continue
		}
		if event.ExpectedPreviousRevision != state.Revision || event.StateRevision != state.Revision+1 || event.From != state.State {
			return IssueState{}, ErrStateConflict
		}
		state = IssueState{State: event.To, Revision: event.StateRevision}
	}
	return state, nil
}

func (tracker GitHubTracker) AppendStateEvent(ctx context.Context, repoNodeID string, issueNumber int, event StateEvent) (IssueState, error) {
	current, err := tracker.ReadState(ctx, repoNodeID, issueNumber)
	if err != nil {
		return IssueState{}, err
	}
	if current.Revision != event.ExpectedPreviousRevision || current.State != event.From {
		return IssueState{}, ErrStateConflict
	}
	body, err := encodeMarkedJSON(stateMarkerPrefix, event)
	if err != nil {
		return IssueState{}, err
	}
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return IssueState{}, err
	}
	var response any
	if err := tracker.apiJSON(ctx, payload, &response, "repos/{owner}/{repo}/issues/"+strconv.Itoa(issueNumber)+"/comments"); err != nil {
		return IssueState{}, err
	}
	updated, err := tracker.ReadState(ctx, repoNodeID, issueNumber)
	if err != nil {
		return IssueState{}, err
	}
	if updated.Revision != event.StateRevision || updated.State != event.To {
		return IssueState{}, ErrStateConflict
	}
	return updated, nil
}

func (tracker GitHubTracker) ProjectState(ctx context.Context, _ string, issueNumber int, state BusinessState) error {
	payload := map[string]string{"state": "open"}
	if state == StateDone {
		payload["state"] = "closed"
		payload["state_reason"] = "completed"
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := "repos/{owner}/{repo}/issues/" + strconv.Itoa(issueNumber)
	output, err := tracker.runGH(ctx, data, "api", endpoint, "--method", http.MethodPatch, "--input", "-")
	if err != nil {
		return err
	}
	var response githubIssue
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return fmt.Errorf("decode GitHub issue projection response: %w", err)
	}
	return nil
}

func (tracker GitHubTracker) ServerClock() ServerClock {
	return func(ctx context.Context) (time.Time, time.Duration, error) {
		started := time.Now()
		output, err := tracker.runGH(ctx, nil, "api", "-i", "rate_limit")
		roundTrip := time.Since(started)
		if err != nil {
			return time.Time{}, 0, err
		}
		serverTime, err := parseGitHubDate(output)
		if err != nil {
			return time.Time{}, 0, err
		}
		return serverTime, roundTrip + time.Second, nil
	}
}

type githubComment struct {
	ID        int64       `json:"id"`
	Body      string      `json:"body"`
	CreatedAt string      `json:"created_at"`
	User      GitHubActor `json:"user"`
}

func (tracker GitHubTracker) comments(ctx context.Context, issueNumber int) ([]githubComment, error) {
	var pages [][]githubComment
	endpoint := "repos/{owner}/{repo}/issues/" + strconv.Itoa(issueNumber) + "/comments?per_page=100"
	if err := tracker.apiJSONWithArgs(ctx, nil, &pages, endpoint, "--paginate", "--slurp"); err != nil {
		return nil, err
	}
	var comments []githubComment
	for _, page := range pages {
		comments = append(comments, page...)
	}
	sort.SliceStable(comments, func(i, j int) bool { return comments[i].ID < comments[j].ID })
	return comments, nil
}

func (tracker GitHubTracker) actorTrusted(actorID int64) bool {
	for _, trusted := range tracker.TrustedActorIDs {
		if trusted == actorID {
			return true
		}
	}
	return false
}

func (tracker GitHubTracker) apiJSON(ctx context.Context, input []byte, output any, endpoint string) error {
	return tracker.apiJSONWithArgs(ctx, input, output, endpoint)
}

func (tracker GitHubTracker) apiJSONWithArgs(ctx context.Context, input []byte, output any, endpoint string, extra ...string) error {
	args := []string{"api", endpoint}
	if input != nil {
		args = append(args, "--method", http.MethodPost, "--input", "-")
	}
	args = append(args, extra...)
	data, err := tracker.runGH(ctx, input, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(data), output); err != nil {
		return fmt.Errorf("decode GitHub API response: %w", err)
	}
	return nil
}

func (tracker GitHubTracker) runGH(ctx context.Context, stdin []byte, args ...string) (string, error) {
	if tracker.Command != nil {
		return tracker.Command(ctx, stdin, args...)
	}
	binary := tracker.GHBinary
	if binary == "" {
		binary = "gh"
	}
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = tracker.RepositoryPath
	command.Stdin = bytes.NewReader(stdin)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func encodeMarkedJSON(prefix string, value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return prefix + string(data) + markerSuffix, nil
}

func decodeMarkedJSON(body, prefix string, output any) bool {
	start := strings.Index(body, prefix)
	if start < 0 {
		return false
	}
	dataStart := start + len(prefix)
	end := strings.Index(body[dataStart:], markerSuffix)
	if end < 0 {
		return false
	}
	return json.Unmarshal([]byte(body[dataStart:dataStart+end]), output) == nil
}

func parseGitHubDate(response string) (time.Time, error) {
	for _, line := range strings.Split(strings.ReplaceAll(response, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.ToLower(line), "date:") {
			value := strings.TrimSpace(strings.TrimPrefix(line, line[:5]))
			parsed, err := http.ParseTime(value)
			if err != nil {
				return time.Time{}, fmt.Errorf("parse GitHub Date header: %w", err)
			}
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("GitHub response is missing Date header")
}

var markdownURLPattern = regexp.MustCompile(`https?://[^\s)>]+`)

func snapshotFromIssue(repoNodeID string, issue githubIssue) (Snapshot, error) {
	sections := make(map[string][]string)
	current := ""
	for _, rawLine := range strings.Split(strings.ReplaceAll(issue.Body, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "## ") {
			heading := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			switch strings.ToLower(heading) {
			case "目标", "goal":
				current = "goal"
			case "验收", "acceptance":
				current = "acceptance"
			case "正文凭据", "evidence":
				current = "evidence"
			default:
				current = ""
			}
			continue
		}
		if current != "" && line != "" {
			sections[current] = append(sections[current], line)
		}
	}
	goal := strings.TrimSpace(strings.Join(sections["goal"], " "))
	if goal == "" || goal == "一句话说明要解决什么。" {
		return Snapshot{}, errors.New("Issue 目标 section is missing or still contains the template placeholder")
	}
	var acceptance []string
	for _, line := range sections["acceptance"] {
		item := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(line, "- [ ]"), "- [x]"), "-"))
		if item != "" && item != "写出可以检查的通过条件。" {
			acceptance = append(acceptance, item)
		}
	}
	if len(acceptance) == 0 {
		return Snapshot{}, errors.New("Issue 验收 section is missing or still contains the template placeholder")
	}
	documentURL := ""
	if match := markdownURLPattern.FindString(strings.Join(sections["evidence"], " ")); match != "" {
		documentURL = match
	}
	return Snapshot{
		RepoNodeID: repoNodeID, IssueNumber: issue.Number, Goal: goal, Acceptance: acceptance,
		DocumentURL: documentURL, UntrustedBody: issue.Body,
	}, nil
}
