package task

import (
	"strings"
	"testing"
)

func TestReviewEventsCreateContinuousCoverageAndStableFindings(t *testing.T) {
	firstInput := reviewInput("review-1", "base", "head-1", ReviewFull)
	firstInput.Findings = []ReviewFinding{reviewFinding(FindingOpen)}
	first := mustBuildReview(t, ReviewChain{}, firstInput)
	if first.Findings[0].ID != "F-001" {
		t.Fatalf("finding = %+v", first.Findings[0])
	}
	chain := mustApplyReview(t, ReviewChain{}, first)
	closed := reviewFinding(FindingResolved)
	closed.ID = "F-001"
	newBlocker := reviewFinding(FindingOpen)
	newBlocker.Invariant = "new regression"
	secondInput := reviewInput("review-2", "head-1", "head-2", ReviewDelta)
	secondInput.Findings = []ReviewFinding{closed, newBlocker}
	second := mustBuildReview(t, chain, secondInput)
	chain = mustApplyReview(t, chain, second)
	if len(chain.Segments) != 2 || second.Findings[1].ID != "F-002" || chain.Findings["F-001"].Status != FindingResolved || chain.Findings["F-002"].Status != FindingOpen {
		t.Fatalf("chain = %+v", chain)
	}
	if err := ValidateReviewCoverage(chain, "base", "head-2", "snapshot", "scope"); err == nil || !strings.Contains(err.Error(), "F-002") {
		t.Fatalf("new blocker gate error = %v", err)
	}
}

func TestReviewEventRejectsInvalidFindingChanges(t *testing.T) {
	rewrite := reviewFinding(FindingResolved)
	rewrite.ID, rewrite.Severity, rewrite.Paths, rewrite.Invariant = "F-001", FindingNonBlocking, []string{"other.go"}, "rewritten"
	selfClosed := reviewFinding(FindingResolved)
	absolute := reviewFinding(FindingOpen)
	absolute.Paths = []string{"/tmp/"}
	for _, test := range []struct {
		name  string
		chain ReviewChain
		item  ReviewFinding
		want  string
	}{
		{name: "identity rewrite", chain: reviewedChain(t, FindingOpen), item: rewrite, want: "identity"},
		{name: "self closed new", item: selfClosed, want: "start open"},
		{name: "absolute path", item: absolute, want: "portable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := reviewInput("review-bad", "base", "head-2", ReviewFull)
			input.Findings = []ReviewFinding{test.item}
			_, err := BuildReviewEvent(test.chain, input)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestValidateReviewCoverageRejectsBlockerHeadAndBasisMismatch(t *testing.T) {
	chain := reviewedChain(t, FindingOpen)
	if err := ValidateReviewCoverage(chain, "base", "head-1", "snapshot", "scope"); err == nil || !strings.Contains(err.Error(), "F-001") {
		t.Fatalf("blocker error = %v", err)
	}
	chain.Findings["F-001"] = withFindingID(reviewFinding(FindingResolved), "F-001")
	if err := ValidateReviewCoverage(chain, "base", "other", "snapshot", "scope"); err == nil || !strings.Contains(err.Error(), "final HEAD") {
		t.Fatalf("head error = %v", err)
	}
	basis := reviewBasis("base")
	chain.Segments[0].ReviewBasis = basis
	second := chain.Segments[0]
	second.Mode, second.BaseSHA, second.HeadSHA = ReviewDelta, "head-1", "head-2"
	second.ReviewBasis.PullRequestNumber++
	chain.Segments = append(chain.Segments, second)
	if err := ValidateReviewCoverage(chain, "base", "head-2", "snapshot", "scope"); err == nil || !strings.Contains(err.Error(), "basis") {
		t.Fatalf("basis error = %v", err)
	}
}

func TestPlanReviewUsesDeltaAndResetsOnlyForChangedContract(t *testing.T) {
	chain := reviewedChain(t, FindingOpen)
	plan := mustPlanReview(t, ReviewPlanInput{Chain: chain, TaskBaseSHA: "base", HeadSHA: "head-2", SnapshotHash: "snapshot", ScopeHash: "scope"})
	if plan.Mode != ReviewDelta || plan.BaseSHA != "head-1" || len(plan.OpenFindings) != 1 {
		t.Fatalf("delta plan = %+v", plan)
	}
	if _, err := PlanReview(ReviewPlanInput{Chain: chain, TaskBaseSHA: "base", HeadSHA: "head-1", SnapshotHash: "snapshot", ScopeHash: "scope"}); err == nil {
		t.Fatal("already reviewed HEAD was accepted")
	}
	for _, input := range []ReviewPlanInput{
		{Chain: chain, TaskBaseSHA: "new-base", HeadSHA: "head-2", SnapshotHash: "snapshot", ScopeHash: "scope"},
		{Chain: chain, TaskBaseSHA: "base", HeadSHA: "head-2", SnapshotHash: "new-snapshot", ScopeHash: "scope"},
		{Chain: chain, TaskBaseSHA: "base", HeadSHA: "head-2", SnapshotHash: "snapshot", ScopeHash: "new-scope"},
	} {
		if got := mustPlanReview(t, input); got.Mode != ReviewFull || len(got.OpenFindings) != 1 {
			t.Fatalf("reset plan = %+v", got)
		}
	}
	chain.Segments[0].ReviewBasis = reviewBasis("base")
	input := ReviewPlanInput{Chain: chain, TaskBaseSHA: "base", HeadSHA: "head-2", SnapshotHash: "snapshot", ScopeHash: "scope", ReviewBasis: reviewBasis("other-base")}
	if got := mustPlanReview(t, input); got.Mode != ReviewFull || got.BaseSHA != "other-base" {
		t.Fatalf("basis reset plan = %+v", got)
	}
}

func TestDeltaEventCannotCrossPullRequestBasis(t *testing.T) {
	chain := reviewedChain(t, FindingOpen)
	chain.Segments[0].ReviewBasis = reviewBasis("base")
	input := reviewInput("review-2", "head-1", "head-2", ReviewDelta)
	input.ReviewBasis = reviewBasis("other-base")
	if _, err := BuildReviewEvent(chain, input); err == nil || !strings.Contains(err.Error(), "basis") {
		t.Fatalf("basis error = %v", err)
	}
}

func TestFullResetDispositionsOpenAndStartsNewFindingGeneration(t *testing.T) {
	open := reviewedChain(t, FindingOpen)
	if _, err := BuildReviewEvent(open, reviewInput("review-reset", "new-base", "head-2", ReviewFull)); err == nil || !strings.Contains(err.Error(), "F-001") {
		t.Fatalf("dropped open finding error = %v", err)
	}
	closed := reviewFinding(FindingResolved)
	closed.ID = "F-001"
	reset := reviewInput("review-reset", "new-base", "head-2", ReviewFull)
	reset.SnapshotHash = "new-snapshot"
	reset.Findings = []ReviewFinding{closed}
	chain := mustApplyReview(t, open, mustBuildReview(t, open, reset))
	if len(chain.Segments) != 1 || chain.Findings["F-001"].Status != FindingResolved {
		t.Fatalf("reset chain = %+v", chain)
	}
	archive := reviewInput("review-archive", "head-2", "head-3", ReviewFull)
	archive.SnapshotHash = "new-snapshot"
	chain = mustApplyReview(t, chain, mustBuildReview(t, chain, archive))
	next := reviewInput("review-next", "head-3", "head-4", ReviewDelta)
	next.SnapshotHash = "new-snapshot"
	next.Findings = []ReviewFinding{reviewFinding(FindingOpen)}
	if event := mustBuildReview(t, chain, next); event.Findings[0].ID != "F-002" {
		t.Fatalf("finding ID was reused after reset: %+v", event.Findings)
	}
	next.Findings[0].ID = "F-001"
	if _, err := BuildReviewEvent(chain, next); err == nil || !strings.Contains(err.Error(), "empty ID") {
		t.Fatalf("explicit retired finding ID error = %v", err)
	}
	replayInput := reviewInput("review-replay", "head-3", "head-4", ReviewDelta)
	replayInput.SnapshotHash = "new-snapshot"
	replay := mustBuildReview(t, chain, replayInput)
	replay.Findings = []ReviewFinding{withFindingID(reviewFinding(FindingOpen), "F-001")}
	if _, err := ApplyReviewEvent(chain, replay); err == nil || !strings.Contains(err.Error(), "next") {
		t.Fatalf("retired replay finding ID error = %v", err)
	}
}

func reviewInput(id, base, head string, mode ReviewMode) ReviewEventInput {
	return ReviewEventInput{EventID: id, RunID: "run-1", BaseSHA: base, HeadSHA: head, Mode: mode, SnapshotHash: "snapshot", ScopeHash: "scope", ReviewReference: "codex-review://" + id}
}

func reviewFinding(status FindingStatus) ReviewFinding {
	finding := ReviewFinding{Severity: FindingBlocking, Status: status, Paths: []string{"internal/task/lease.go"}, Invariant: "one active owner", Evidence: "evidence"}
	if status != FindingOpen {
		finding.Disposition = "fixed"
	}
	return finding
}

func withFindingID(finding ReviewFinding, id string) ReviewFinding { finding.ID = id; return finding }

func reviewBasis(mergeBase string) ReviewBasis {
	return ReviewBasis{PullRequestNumber: 18, BaseRef: "main", MergeBaseSHA: mergeBase}
}

func mustBuildReview(t *testing.T, chain ReviewChain, input ReviewEventInput) ReviewEvent {
	t.Helper()
	event, err := BuildReviewEvent(chain, input)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func mustApplyReview(t *testing.T, chain ReviewChain, event ReviewEvent) ReviewChain {
	t.Helper()
	chain, err := ApplyReviewEvent(chain, event)
	if err != nil {
		t.Fatal(err)
	}
	return chain
}

func mustPlanReview(t *testing.T, input ReviewPlanInput) ReviewPlan {
	t.Helper()
	plan, err := PlanReview(input)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func reviewedChain(t *testing.T, status FindingStatus) ReviewChain {
	t.Helper()
	input := reviewInput("review-1", "base", "head-1", ReviewFull)
	input.Findings = []ReviewFinding{reviewFinding(status)}
	return mustApplyReview(t, ReviewChain{}, mustBuildReview(t, ReviewChain{}, input))
}
