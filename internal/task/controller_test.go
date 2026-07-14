package task

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Strelizialeomon/sop-better/internal/config"
)

func TestControllerRejectsUntrustedSnapshotBeforeCreatingClaim(t *testing.T) {
	profile := loopProfile()
	snapshot, attestation := approvedTask(t, profile)
	attestation.SnapshotHash = "sha256:tampered"
	claims := newMemoryClaimStore(time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC))
	controller := Controller{Profile: profile, Leases: newTestLeaseService(claims)}

	_, err := controller.Start(context.Background(), StartRequest{Snapshot: snapshot, Attestation: attestation, CurrentActorID: 123456})
	if err == nil || !strings.Contains(err.Error(), "ready attestation does not match") {
		t.Fatalf("Start() error = %v", err)
	}
	if _, exists, _ := claims.Read(context.Background(), snapshot.RepoNodeID, snapshot.IssueNumber); exists {
		t.Fatal("untrusted snapshot created a remote claim")
	}
}

func TestControllerStartsAndContinuesSameRun(t *testing.T) {
	profile := loopProfile()
	snapshot, attestation := approvedTask(t, profile)
	now := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)
	claims := newMemoryClaimStore(now)
	issues := &memoryIssueStateStore{state: StateReady, revision: 7}
	leases := newTestLeaseService(claims)
	reconciler := Reconciler{Claims: claims, Issues: issues, Leases: leases, NewEventID: func() (string, error) { return "evt-1", nil }}
	repository := initWorkspaceTestRepository(t)
	baseSHA := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
	controller := Controller{
		Profile: profile, Leases: leases, Reconciler: reconciler,
		Workspaces: WorkspaceManager{Root: filepath.Join(t.TempDir(), "workspaces")},
	}

	started, err := controller.Start(context.Background(), StartRequest{
		Snapshot: snapshot, Attestation: attestation, CurrentActorID: 123456,
		RunID: "run-1", MachineID: "mac-1", StateRevision: 7, BaseSHA: baseSHA,
		RepositoryPath: repository,
	})
	if err != nil {
		t.Fatal(err)
	}
	if started.Capsule.Task != 31 || started.Claim.Claim.RunID != "run-1" || started.Workspace.Resumed {
		t.Fatalf("started = %#v", started)
	}
	claims.setNow(now.Add(time.Minute))
	continued, err := controller.Continue(context.Background(), ContinueRequest{
		RepoNodeID: "R_repo", IssueNumber: 31, RunID: "run-1", MachineID: "mac-1",
		Snapshot: snapshot, Attestation: attestation, CurrentActorID: 123456, RepositoryPath: repository,
	})
	if err != nil {
		t.Fatal(err)
	}
	if continued.Claim.OID == started.Claim.OID {
		t.Fatal("Continue() did not renew the lease with CAS")
	}
	current, exists, err := claims.Read(context.Background(), "R_repo", 31)
	if err != nil || !exists || current.OID != continued.Claim.OID {
		t.Fatalf("Continue() returned stale claim: current=%#v result=%#v err=%v", current, continued.Claim, err)
	}
	if !continued.Workspace.Resumed || continued.Capsule.SnapshotHash != started.Capsule.SnapshotHash {
		t.Fatalf("Continue() did not restore capsule/workspace: %#v", continued)
	}
}

func TestControllerRefusesContinueFromDifferentMachine(t *testing.T) {
	profile := loopProfile()
	claims := newMemoryClaimStore(time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC))
	leases := newTestLeaseService(claims)
	mustClaim(t, leases, "run-1")
	controller := Controller{Profile: profile, Leases: leases}
	_, err := controller.Continue(context.Background(), ContinueRequest{RepoNodeID: "R_repo", IssueNumber: 31, RunID: "run-1", MachineID: "other-machine"})
	if !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("Continue() error = %v, want ErrLeaseLost", err)
	}
}

func TestControllerRefusesTerminalTransitionFromDifferentMachine(t *testing.T) {
	profile := loopProfile()
	claims := newMemoryClaimStore(time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC))
	leases := newTestLeaseService(claims)
	claim := mustClaim(t, leases, "run-1")
	issues := &memoryIssueStateStore{state: StateRunning, revision: 8}
	controller := Controller{
		Profile: profile, Leases: leases,
		Reconciler: Reconciler{Claims: claims, Issues: issues, Leases: leases, NewEventID: func() (string, error) { return "evt", nil }},
	}

	_, err := controller.Close(context.Background(), CloseRequest{
		Claim: claim, RunID: "run-1", MachineID: "other-machine",
		Target: StateWaiting, Reason: "attempted from another trusted machine",
	})
	if !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("Close() error = %v, want ErrLeaseLost", err)
	}
	if _, exists, _ := claims.Read(context.Background(), "R_repo", 31); !exists {
		t.Fatal("foreign machine released the active owner's claim")
	}
	if got := issues.snapshot().State; got != StateRunning {
		t.Fatalf("state = %q, foreign machine changed the task", got)
	}
}

func TestControllerCompletesOnlyWithChecksReviewPRAndFinalVerification(t *testing.T) {
	profile := loopProfile()
	now := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)
	claims := newMemoryClaimStore(now)
	issues := &memoryIssueStateStore{state: StateRunning, revision: 8}
	leases := newTestLeaseService(claims)
	claim := mustClaim(t, leases, "run-1")
	reconciler := Reconciler{Claims: claims, Issues: issues, Leases: leases, NewEventID: func() (string, error) { return "evt-done", nil }}
	reviews := &memoryReviewStore{chain: cleanReviewChain(t, profile, claim.Claim.BaseSHA, "def123", "snapshot")}
	reviews.chain.Segments[0].ReviewBasis = reviewBasis(claim.Claim.BaseSHA)
	checks := &fakeCheckExecutor{}
	controller := Controller{Profile: profile, Leases: leases, Reconciler: reconciler, Reviews: reviews, Checks: checks}

	_, err := controller.Close(context.Background(), CloseRequest{
		Claim: claim, RunID: "run-1", MachineID: "machine-run-1", Target: StateDone, Reason: "implemented",
		Evidence: CompletionEvidence{AcceptanceVerified: true, ChecksPassed: map[string]bool{"test": true}},
	})
	if err == nil {
		t.Fatal("Close(done) accepted incomplete evidence")
	}
	done := CloseRequest{
		Claim: claim, RunID: "run-1", MachineID: "machine-run-1", Target: StateDone, Reason: "implemented", SnapshotHash: "snapshot", Workspace: t.TempDir(),
		Evidence: CompletionEvidence{
			AcceptanceVerified: true, ChecksPassed: map[string]bool{"test": true},
			ReviewBasis: reviewBasis(claim.Claim.BaseSHA), PullRequestURL: "https://github.com/acme/repo/pull/18", PullRequestHeadSHA: "def123", MergedCommitSHA: "merged-def123",
		},
	}
	if _, err := controller.Close(context.Background(), done); err == nil || !strings.Contains(err.Error(), "waiting") {
		t.Fatalf("running-to-done error = %v", err)
	}
	issues.state, issues.revision = StateWaiting, claim.Claim.StateRevision
	done.Evidence.ReviewBasis = reviewBasis("other-base")
	if _, err := controller.Close(context.Background(), done); err == nil || !strings.Contains(err.Error(), "basis") {
		t.Fatalf("different PR basis error = %v", err)
	}
	done.Claim = mustClaim(t, leases, "run-1")
	done.Evidence.ReviewBasis = reviewBasis(done.Claim.Claim.BaseSHA)
	result, err := controller.Close(context.Background(), done)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateDone {
		t.Fatalf("state = %s", result.State)
	}
	if checks.headSHA != "merged-def123" {
		t.Fatalf("final checks ran at %q, want merged commit", checks.headSHA)
	}
	if _, exists, _ := claims.Read(context.Background(), "R_repo", 31); exists {
		t.Fatal("done task retained its claim")
	}
}

func TestControllerRejectsAgentWrittenReviewEvidenceWithoutCanonicalChain(t *testing.T) {
	profile := loopProfile()
	claims := newMemoryClaimStore(time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC))
	leases := newTestLeaseService(claims)
	claim := mustClaim(t, leases, "run-1")
	issues := &memoryIssueStateStore{state: StateWaiting, revision: claim.Claim.StateRevision}
	controller := Controller{
		Profile: profile, Leases: leases, Checks: &fakeCheckExecutor{}, Reviews: &memoryReviewStore{},
		Reconciler: Reconciler{Claims: claims, Issues: issues, Leases: leases, NewEventID: func() (string, error) { return "evt", nil }},
	}
	_, err := controller.Close(context.Background(), CloseRequest{
		Claim: claim, RunID: "run-1", MachineID: "machine-run-1", Target: StateDone, Reason: "done", SnapshotHash: "snapshot", Workspace: t.TempDir(),
		Evidence: CompletionEvidence{
			AcceptanceVerified: true, PullRequestURL: "https://github.com/acme/repo/pull/31", PullRequestHeadSHA: "head-1", MergedCommitSHA: "merged-head-1",
			ReviewCompleted: true, BlockingFindings: 0, ReviewReference: "agent-written://fake", ReviewedHeadSHA: "head-1",
			ChecksPassed: map[string]bool{"test": true}, FinalVerification: "agent says passed",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "canonical full review") {
		t.Fatalf("Close() error = %v, want canonical review rejection", err)
	}
}

func TestControllerWaitingRequiresReasonAndReleasesClaim(t *testing.T) {
	profile := loopProfile()
	now := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)
	claims := newMemoryClaimStore(now)
	issues := &memoryIssueStateStore{state: StateRunning, revision: 8}
	leases := newTestLeaseService(claims)
	claim := mustClaim(t, leases, "run-1")
	reconciler := Reconciler{Claims: claims, Issues: issues, Leases: leases, NewEventID: func() (string, error) { return "evt-wait", nil }}
	controller := Controller{Profile: profile, Leases: leases, Reconciler: reconciler}
	if _, err := controller.Close(context.Background(), CloseRequest{Claim: claim, RunID: "run-1", MachineID: "machine-run-1", Target: StateWaiting}); err == nil {
		t.Fatal("Close(waiting) accepted an empty recovery reason")
	}
	if _, err := controller.Close(context.Background(), CloseRequest{Claim: claim, RunID: "run-1", MachineID: "machine-run-1", Target: StateWaiting, Reason: "need owner API decision; resume after contract approval"}); err != nil {
		t.Fatal(err)
	}
	if _, exists, _ := claims.Read(context.Background(), "R_repo", 31); exists {
		t.Fatal("waiting task retained its claim")
	}
}

func TestControllerClaimsTerminalVerificationFromWaitingRevision(t *testing.T) {
	profile := loopProfile()
	claims := newMemoryClaimStore(time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC))
	issues := &memoryIssueStateStore{state: StateWaiting, revision: 9}
	leases := newTestLeaseService(claims)
	controller := Controller{
		Profile: profile, Leases: leases,
		Reconciler: Reconciler{Claims: claims, Issues: issues, Leases: leases, NewEventID: func() (string, error) { return "evt-terminal", nil }},
		Reviews:    &memoryReviewStore{chain: cleanReviewChain(t, profile, "task-base", "pr-head", "snapshot")},
	}
	claim, err := controller.ClaimTerminalVerification(context.Background(), TerminalVerificationClaimRequest{
		RepoNodeID: "R_repo", IssueNumber: 31, RunID: "terminal-1", MachineID: "machine-terminal", ActorID: 123456,
	})
	if err != nil {
		t.Fatal(err)
	}
	if claim.Claim.StateRevision != 9 || claim.Claim.BaseSHA != "task-base" {
		t.Fatalf("terminal claim = %+v", claim.Claim)
	}
}

func TestControllerReleasesTerminalVerificationClaimWhenFinalEvidenceFails(t *testing.T) {
	profile := loopProfile()
	claims := newMemoryClaimStore(time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC))
	issues := &memoryIssueStateStore{state: StateWaiting, revision: 9}
	leases := newTestLeaseService(claims)
	controller := Controller{
		Profile: profile, Leases: leases,
		Reconciler: Reconciler{Claims: claims, Issues: issues, Leases: leases, NewEventID: func() (string, error) { return "evt-terminal", nil }},
		Reviews:    &memoryReviewStore{chain: cleanReviewChain(t, profile, "task-base", "pr-head", "snapshot")},
		Checks:     &fakeCheckExecutor{},
	}
	claim, err := controller.ClaimTerminalVerification(context.Background(), TerminalVerificationClaimRequest{
		RepoNodeID: "R_repo", IssueNumber: 31, RunID: "terminal-1", MachineID: "machine-terminal", ActorID: 123456,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = controller.Close(context.Background(), CloseRequest{
		Claim: claim, RunID: claim.Claim.RunID, MachineID: claim.Claim.MachineID,
		Target: StateDone, Reason: "verify merged commit", SnapshotHash: "snapshot", Workspace: t.TempDir(),
		Evidence: CompletionEvidence{AcceptanceVerified: false, PullRequestURL: "https://github.com/acme/repo/pull/31", PullRequestHeadSHA: "pr-head", MergedCommitSHA: "merged"},
	})
	if err == nil {
		t.Fatal("invalid final evidence unexpectedly completed")
	}
	if _, exists, _ := claims.Read(context.Background(), "R_repo", 31); exists {
		t.Fatal("failed terminal verification retained its claim")
	}
	if state := issues.snapshot(); state.State != StateWaiting || state.Revision != 9 {
		t.Fatalf("failed terminal verification changed state: %+v", state)
	}
}

func TestControllerReleasesTerminalClaimWhenDoneProjectionFailsAndReconcileRepairs(t *testing.T) {
	profile := loopProfile()
	now := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)
	claims := newMemoryClaimStore(now)
	baseIssues := &memoryIssueStateStore{state: StateWaiting, revision: 7}
	issues := &projectingIssueStore{memoryIssueStateStore: baseIssues, fail: true}
	leases := newTestLeaseService(claims)
	claim := mustClaim(t, leases, "run-1")
	reconciler := Reconciler{Claims: claims, Issues: issues, Leases: leases, NewEventID: func() (string, error) { return "evt-done", nil }}
	controller := Controller{
		Profile: profile, Leases: leases, Reconciler: reconciler,
		Reviews: &memoryReviewStore{chain: cleanReviewChain(t, profile, claim.Claim.BaseSHA, "abc", "snapshot")},
		Checks:  &fakeCheckExecutor{},
	}
	evidence := CompletionEvidence{
		AcceptanceVerified: true, ChecksPassed: map[string]bool{"test": true}, PullRequestURL: "https://github.com/acme/repo/pull/31",
		PullRequestHeadSHA: "abc", MergedCommitSHA: "merged-abc",
	}
	if _, err := controller.Close(context.Background(), CloseRequest{Claim: claim, RunID: "run-1", MachineID: "machine-run-1", Target: StateDone, Reason: "done", SnapshotHash: "snapshot", Workspace: t.TempDir(), Evidence: evidence}); err == nil {
		t.Fatal("projection failure unexpectedly completed")
	}
	if _, exists, _ := claims.Read(context.Background(), "R_repo", 31); exists {
		t.Fatal("failed terminal projection retained its claim")
	}
	issues.fail = false
	if _, err := reconciler.Reconcile(context.Background(), ReconcileRequest{RepoNodeID: "R_repo", IssueNumber: 31}); err != nil {
		t.Fatal(err)
	}
	if _, exists, _ := claims.Read(context.Background(), "R_repo", 31); exists {
		t.Fatal("reconcile did not release claim after projection repair")
	}
}

func TestControllerEscalatesDeltaToFullBeforeAppendingCanonicalReview(t *testing.T) {
	profile := loopProfile()
	profile.Runtime.Checks["build"] = []string{"go build ./..."}
	claims := newMemoryClaimStore(time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC))
	leases := newTestLeaseService(claims)
	claim := mustClaim(t, leases, "run-1")
	capsule := Capsule{Goal: "fix", Acceptance: []string{"tests pass"}, Role: "backend", AllowedPaths: []string{"backend/"}, SnapshotHash: "snapshot", Risk: Risk{Class: "low"}}
	basis := ReviewBasis{PullRequestNumber: 18, BaseRef: "main", MergeBaseSHA: claim.Claim.BaseSHA}
	prior := reviewedChain(t, FindingOpen)
	prior.Findings["F-001"] = withFindingID(reviewFinding(FindingResolved), "F-001")
	prior.Segments[0].ScopeHash, _ = CapsuleScopeHash(capsule)
	prior.Segments[0].BaseSHA, prior.Segments[0].ReviewBasis = claim.Claim.BaseSHA, basis
	reviews := &memoryReviewStore{chain: prior}
	checks := &fakeCheckExecutor{}
	reviewer := &fakeReviewExecutor{results: []ReviewExecutionResult{{RequiresFullReview: true, FullReviewReason: "previous SHA unavailable"}, {Reference: "codex-review://thread-1", Summary: "clean"}}}
	controller := Controller{
		Profile: profile, Leases: leases, Reviews: reviews, Checks: checks, Reviewer: reviewer,
		NewReviewEventID: func() (string, error) { return "review-1", nil },
	}
	result, err := controller.Review(context.Background(), ReviewRequest{
		Claim: claim, RunID: "run-1", MachineID: "machine-run-1", Snapshot: Snapshot{Goal: "fix", Acceptance: []string{"tests pass"}}, Capsule: capsule,
		ChecksWorkspace: t.TempDir(), ReviewerWorkspace: t.TempDir(),
		HeadSHA: "head-2", TaskChangedPaths: []string{"backend/service.go"}, ReviewBasis: basis,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Plan.Mode != ReviewFull || !result.Selection.Full || len(result.CheckRuns) != 2 || len(result.Chain.Segments) != 1 || result.Chain.Segments[0].ReviewBasis != basis || result.Chain.Segments[0].ReviewReference != "codex-review://thread-1" {
		t.Fatalf("result = %+v", result)
	}
	if checks.calls != 1 || reviewer.calls != 2 || reviewer.request.BaseSHA != claim.Claim.BaseSHA || reviewer.request.Mode != ReviewFull {
		t.Fatalf("checks=%d reviewer=%d request=%+v", checks.calls, reviewer.calls, reviewer.request)
	}
}

func TestControllerRenewsLeaseDuringLongReview(t *testing.T) {
	profile := loopProfile()
	claims := newMemoryClaimStore(time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC))
	leases := newTestLeaseService(claims)
	claim := mustClaim(t, leases, "run-1")
	controller := Controller{
		Profile: profile, Leases: leases, Reviews: &memoryReviewStore{}, Checks: &fakeCheckExecutor{},
		Reviewer:         &fakeReviewExecutor{delay: 20 * time.Millisecond, results: []ReviewExecutionResult{{Reference: "codex-review://thread-1", Summary: "clean"}}},
		NewReviewEventID: func() (string, error) { return "review-1", nil }, HeartbeatInterval: 2 * time.Millisecond,
	}
	_, err := controller.Review(context.Background(), ReviewRequest{
		Claim: claim, RunID: "run-1", MachineID: "machine-run-1", Snapshot: Snapshot{Goal: "fix", Acceptance: []string{"tests pass"}},
		Capsule:         Capsule{Role: "backend", AllowedPaths: []string{"backend/"}, SnapshotHash: "snapshot", Risk: Risk{Class: "low"}},
		ChecksWorkspace: t.TempDir(), ReviewerWorkspace: t.TempDir(),
		HeadSHA: "head-1", TaskChangedPaths: []string{"backend/service.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if claims.nextOID < 4 {
		t.Fatalf("claim CAS count = %d, want heartbeat renewals during review", claims.nextOID)
	}
}

type projectingIssueStore struct {
	*memoryIssueStateStore
	fail bool
}

type memoryReviewStore struct {
	chain ReviewChain
}

func (store *memoryReviewStore) ReadReviewChain(context.Context, string, int) (ReviewChain, error) {
	return store.chain, nil
}

func (store *memoryReviewStore) AppendReviewEvent(_ context.Context, _ string, _ int, event ReviewEvent) (ReviewChain, error) {
	updated, err := ApplyReviewEvent(store.chain, event)
	if err != nil {
		return ReviewChain{}, err
	}
	store.chain = updated
	return updated, nil
}

type fakeCheckExecutor struct {
	calls   int
	headSHA string
}

func (executor *fakeCheckExecutor) Run(_ context.Context, _ string, headSHA string, _ map[string][]string, groups []string) ([]CheckRun, error) {
	executor.calls++
	executor.headSHA = headSHA
	runs := make([]CheckRun, 0, len(groups))
	for _, group := range groups {
		runs = append(runs, CheckRun{Group: group, HeadSHA: headSHA, Passed: true})
	}
	return runs, nil
}

type fakeReviewExecutor struct {
	calls   int
	request ReviewExecutionRequest
	results []ReviewExecutionResult
	delay   time.Duration
}

func (executor *fakeReviewExecutor) Execute(ctx context.Context, request ReviewExecutionRequest) (ReviewExecutionResult, error) {
	executor.calls++
	executor.request = request
	if executor.delay > 0 {
		select {
		case <-ctx.Done():
			return ReviewExecutionResult{}, ctx.Err()
		case <-time.After(executor.delay):
		}
	}
	return executor.results[min(executor.calls, len(executor.results))-1], nil
}

func cleanReviewChain(t *testing.T, profile config.Profile, baseSHA, headSHA, snapshotHash string) ReviewChain {
	t.Helper()
	scopeHash, err := ProfileScopeHash(profile)
	if err != nil {
		t.Fatal(err)
	}
	event, err := BuildReviewEvent(ReviewChain{}, ReviewEventInput{
		EventID: "review-clean", RunID: "run-1", BaseSHA: baseSHA, HeadSHA: headSHA, Mode: ReviewFull,
		SnapshotHash: snapshotHash, ScopeHash: scopeHash, ReviewReference: "codex-review://thread-clean",
	})
	if err != nil {
		t.Fatal(err)
	}
	chain, err := ApplyReviewEvent(ReviewChain{}, event)
	if err != nil {
		t.Fatal(err)
	}
	return chain
}

func (store *projectingIssueStore) ProjectState(context.Context, string, int, BusinessState) error {
	if store.fail {
		return errors.New("projection failed")
	}
	return nil
}

func approvedTask(t *testing.T, profile config.Profile) (Snapshot, Attestation) {
	t.Helper()
	snapshot := Snapshot{RepoNodeID: "R_repo", IssueNumber: 31, Goal: "fix recovery", Acceptance: []string{"tests pass"}}
	hash, err := SnapshotHash(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot, Attestation{RepoNodeID: "R_repo", IssueNumber: 31, SnapshotHash: hash, ActorID: 123456, SOPVersion: profile.SOPVersion, ServerTime: time.Date(2026, 7, 12, 2, 0, 0, 0, time.UTC)}
}
