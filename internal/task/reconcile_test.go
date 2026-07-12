package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestReconcileRepairsReadyIssueAfterClaimWasCreated(t *testing.T) {
	fixture := newReconcileFixture(t, StateReady)
	claim := mustClaim(t, fixture.leases, "run-1")

	result, err := fixture.reconciler.Reconcile(context.Background(), ReconcileRequest{
		RepoNodeID: "R_repo", IssueNumber: 31, SnapshotHash: "sha256:snapshot",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateRunning || result.Action != ReconcileAppendedState {
		t.Fatalf("result = %#v, want appended running state", result)
	}
	event := fixture.issues.lastEvent(t)
	if event.From != StateReady || event.To != StateRunning || event.Reason != "claim-created" {
		t.Fatalf("event = %#v", event)
	}
	if event.RunID != claim.Claim.RunID || event.LeaseEpoch != claim.Claim.LeaseEpoch || event.FencingToken != claim.Claim.FencingToken {
		t.Fatalf("event does not carry current lease fence: %#v", event)
	}
}

func TestReconcileReleasesClaimAfterPermanentStateWasWritten(t *testing.T) {
	for _, state := range []BusinessState{StateWaiting, StateDone} {
		t.Run(string(state), func(t *testing.T) {
			fixture := newReconcileFixture(t, state)
			mustClaim(t, fixture.leases, "run-1")

			result, err := fixture.reconciler.Reconcile(context.Background(), ReconcileRequest{RepoNodeID: "R_repo", IssueNumber: 31})
			if err != nil {
				t.Fatal(err)
			}
			if result.State != state || result.Action != ReconcileReleasedClaim {
				t.Fatalf("result = %#v", result)
			}
			if _, exists, _ := fixture.claims.Read(context.Background(), "R_repo", 31); exists {
				t.Fatal("claim still exists after terminal reconciliation")
			}
		})
	}
}

func TestReconcileMovesOrphanedRunningIssueToWaiting(t *testing.T) {
	fixture := newReconcileFixture(t, StateRunning)

	result, err := fixture.reconciler.Reconcile(context.Background(), ReconcileRequest{
		RepoNodeID: "R_repo", IssueNumber: 31,
		RecoveryClaim: &ClaimRequest{RunID: "run-reconcile", MachineID: "reconciler", ActorID: 1, BaseSHA: "abc123"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateWaiting || result.Action != ReconcileAppendedState {
		t.Fatalf("result = %#v", result)
	}
	event := fixture.issues.lastEvent(t)
	if event.From != StateRunning || event.To != StateWaiting || event.Reason != "missing-active-claim" {
		t.Fatalf("event = %#v", event)
	}
}

func TestReconcileDoesNotOverwriteNewerOwnerDecision(t *testing.T) {
	fixture := newReconcileFixture(t, StateReady)
	mustClaim(t, fixture.leases, "run-1")
	fixture.issues.beforeAppend = func(store *memoryIssueStateStore) {
		store.beforeAppend = nil
		store.state = StateWaiting
		store.revision++
	}

	_, err := fixture.reconciler.Reconcile(context.Background(), ReconcileRequest{RepoNodeID: "R_repo", IssueNumber: 31})
	if !errors.Is(err, ErrStateConflict) {
		t.Fatalf("Reconcile() error = %v, want ErrStateConflict", err)
	}
	if got := fixture.issues.snapshot().State; got != StateWaiting {
		t.Fatalf("state = %q, newer owner waiting decision was overwritten", got)
	}
}

func TestReconcileReleasesRecoveryClaimWhenStateAppendConflicts(t *testing.T) {
	fixture := newReconcileFixture(t, StateRunning)
	fixture.issues.beforeAppend = func(store *memoryIssueStateStore) {
		store.beforeAppend = nil
		store.state = StateWaiting
		store.revision++
	}
	_, err := fixture.reconciler.Reconcile(context.Background(), ReconcileRequest{
		RepoNodeID: "R_repo", IssueNumber: 31,
		RecoveryClaim: &ClaimRequest{RunID: "run-reconcile", MachineID: "reconciler", ActorID: 1, BaseSHA: "abc123"},
	})
	if !errors.Is(err, ErrStateConflict) {
		t.Fatalf("Reconcile() error = %v, want ErrStateConflict", err)
	}
	if _, exists, _ := fixture.claims.Read(context.Background(), "R_repo", 31); exists {
		t.Fatal("recovery claim leaked after append conflict")
	}
}

type reconcileFixture struct {
	claims     *memoryClaimStore
	issues     *memoryIssueStateStore
	leases     LeaseService
	reconciler Reconciler
}

func newReconcileFixture(t *testing.T, state BusinessState) reconcileFixture {
	t.Helper()
	now := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)
	claims := newMemoryClaimStore(now)
	issues := &memoryIssueStateStore{state: state, revision: 7}
	leases := newTestLeaseService(claims)
	var sequence int
	reconciler := Reconciler{
		Claims: claims,
		Issues: issues,
		Leases: leases,
		NewEventID: func() (string, error) {
			sequence++
			return fmt.Sprintf("evt-%d", sequence), nil
		},
	}
	return reconcileFixture{claims: claims, issues: issues, leases: leases, reconciler: reconciler}
}

type memoryIssueStateStore struct {
	mu           sync.Mutex
	state        BusinessState
	revision     int64
	events       []StateEvent
	beforeAppend func(*memoryIssueStateStore)
}

func (store *memoryIssueStateStore) ReadState(context.Context, string, int) (IssueState, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return IssueState{State: store.state, Revision: store.revision}, nil
}

func (store *memoryIssueStateStore) AppendStateEvent(_ context.Context, _ string, _ int, event StateEvent) (IssueState, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.beforeAppend != nil {
		store.beforeAppend(store)
	}
	if event.ExpectedPreviousRevision != store.revision || event.From != store.state {
		return IssueState{}, ErrStateConflict
	}
	store.state = event.To
	store.revision = event.StateRevision
	store.events = append(store.events, event)
	return IssueState{State: store.state, Revision: store.revision}, nil
}

func (store *memoryIssueStateStore) lastEvent(t *testing.T) StateEvent {
	t.Helper()
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.events) == 0 {
		t.Fatal("no state event was appended")
	}
	return store.events[len(store.events)-1]
}

func (store *memoryIssueStateStore) snapshot() IssueState {
	store.mu.Lock()
	defer store.mu.Unlock()
	return IssueState{State: store.state, Revision: store.revision}
}
