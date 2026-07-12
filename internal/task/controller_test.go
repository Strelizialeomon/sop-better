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
	controller := Controller{
		Profile: profile, Leases: leases, Reconciler: reconciler,
		Workspaces: WorkspaceManager{Root: filepath.Join(t.TempDir(), "workspaces")},
	}

	started, err := controller.Start(context.Background(), StartRequest{
		Snapshot: snapshot, Attestation: attestation, CurrentActorID: 123456,
		RunID: "run-1", MachineID: "mac-1", StateRevision: 7, BaseSHA: "abc123",
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
	})
	if err != nil {
		t.Fatal(err)
	}
	if continued.Claim.OID == started.Claim.OID {
		t.Fatal("Continue() did not renew the lease with CAS")
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

func TestControllerCompletesOnlyWithChecksReviewPRAndFinalVerification(t *testing.T) {
	profile := loopProfile()
	now := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)
	claims := newMemoryClaimStore(now)
	issues := &memoryIssueStateStore{state: StateRunning, revision: 8}
	leases := newTestLeaseService(claims)
	claim := mustClaim(t, leases, "run-1")
	reconciler := Reconciler{Claims: claims, Issues: issues, Leases: leases, NewEventID: func() (string, error) { return "evt-done", nil }}
	controller := Controller{Profile: profile, Leases: leases, Reconciler: reconciler}

	_, err := controller.Close(context.Background(), CloseRequest{
		Claim: claim, Target: StateDone, Reason: "implemented",
		Evidence: CompletionEvidence{AcceptanceVerified: true, ChecksPassed: map[string]bool{"test": true}},
	})
	if err == nil {
		t.Fatal("Close(done) accepted incomplete evidence")
	}
	result, err := controller.Close(context.Background(), CloseRequest{
		Claim: claim, Target: StateDone, Reason: "implemented",
		Evidence: CompletionEvidence{
			AcceptanceVerified: true, ChecksPassed: map[string]bool{"test": true},
			ReviewCompleted: true, BlockingFindings: 0,
			PullRequestURL: "https://github.com/acme/repo/pull/31", FinalVerification: "go test ./... passed after PR head",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateDone {
		t.Fatalf("state = %s", result.State)
	}
	if _, exists, _ := claims.Read(context.Background(), "R_repo", 31); exists {
		t.Fatal("done task retained its claim")
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
	if _, err := controller.Close(context.Background(), CloseRequest{Claim: claim, Target: StateWaiting}); err == nil {
		t.Fatal("Close(waiting) accepted an empty recovery reason")
	}
	if _, err := controller.Close(context.Background(), CloseRequest{Claim: claim, Target: StateWaiting, Reason: "need owner API decision; resume after contract approval"}); err != nil {
		t.Fatal(err)
	}
	if _, exists, _ := claims.Read(context.Background(), "R_repo", 31); exists {
		t.Fatal("waiting task retained its claim")
	}
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
