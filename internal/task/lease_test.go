package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLeaseServiceAllowsOnlyOneConcurrentClaim(t *testing.T) {
	store := newMemoryClaimStore(time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC))
	var tokenSequence atomic.Int64
	service := LeaseService{
		Store: store,
		NewToken: func() (string, error) {
			return fmt.Sprintf("token-%d", tokenSequence.Add(1)), nil
		},
		TTL: 10 * time.Minute,
	}

	const contenders = 100
	var successes atomic.Int64
	var conflicts atomic.Int64
	var wait sync.WaitGroup
	start := make(chan struct{})
	for contender := 0; contender < contenders; contender++ {
		wait.Add(1)
		go func(id int) {
			defer wait.Done()
			<-start
			_, err := service.Claim(context.Background(), ClaimRequest{
				RepoNodeID:    "R_repo",
				IssueNumber:   31,
				RunID:         fmt.Sprintf("run-%d", id),
				MachineID:     fmt.Sprintf("machine-%d", id),
				ActorID:       int64(id + 1),
				StateRevision: 7,
				BaseSHA:       "abc123",
			})
			switch {
			case err == nil:
				successes.Add(1)
			case errors.Is(err, ErrAlreadyClaimed):
				conflicts.Add(1)
			default:
				t.Errorf("Claim(%d) error = %v", id, err)
			}
		}(contender)
	}
	close(start)
	wait.Wait()

	if got := successes.Load(); got != 1 {
		t.Fatalf("successful claims = %d, want 1", got)
	}
	if got := conflicts.Load(); got != contenders-1 {
		t.Fatalf("claim conflicts = %d, want %d", got, contenders-1)
	}
}

func TestLeaseServiceFencesPreviousOwnerAfterTakeover(t *testing.T) {
	started := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)
	store := newMemoryClaimStore(started)
	var tokenSequence atomic.Int64
	service := LeaseService{
		Store: store,
		NewToken: func() (string, error) {
			return fmt.Sprintf("token-%d", tokenSequence.Add(1)), nil
		},
		TTL: 10 * time.Minute,
	}
	first, err := service.Claim(context.Background(), ClaimRequest{
		RepoNodeID: "R_repo", IssueNumber: 31, RunID: "run-old", MachineID: "mac-old", ActorID: 1, StateRevision: 7, BaseSHA: "abc123",
	})
	if err != nil {
		t.Fatal(err)
	}
	store.setNow(started.Add(11 * time.Minute))
	replacement, err := service.Takeover(context.Background(), ClaimRequest{
		RepoNodeID: "R_repo", IssueNumber: 31, RunID: "run-new", MachineID: "win-new", ActorID: 2, StateRevision: 7, BaseSHA: "abc123",
	})
	if err != nil {
		t.Fatalf("Takeover() error = %v", err)
	}
	if got, want := replacement.Claim.LeaseEpoch, int64(2); got != want {
		t.Fatalf("replacement epoch = %d, want %d", got, want)
	}
	if replacement.Claim.FencingToken == first.Claim.FencingToken {
		t.Fatal("replacement reused the previous fencing token")
	}
	if _, err := service.Renew(context.Background(), first); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("old Renew() error = %v, want ErrLeaseLost", err)
	}
}

func TestLeaseServiceReleaseRequiresCurrentClaimOID(t *testing.T) {
	started := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)
	store := newMemoryClaimStore(started)
	service := newTestLeaseService(store)
	first := mustClaim(t, service, "run-old")
	store.setNow(started.Add(11 * time.Minute))
	replacement, err := service.Takeover(context.Background(), ClaimRequest{
		RepoNodeID: "R_repo", IssueNumber: 31, RunID: "run-new", MachineID: "machine-run-new", ActorID: 1, StateRevision: 7, BaseSHA: "abc123",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := service.Release(context.Background(), first); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("old Release() error = %v, want ErrLeaseLost", err)
	}
	if err := service.Release(context.Background(), replacement); err != nil {
		t.Fatalf("current Release() error = %v", err)
	}
	if _, exists, err := store.Read(context.Background(), "R_repo", 31); err != nil || exists {
		t.Fatalf("claim after release: exists=%v error=%v", exists, err)
	}
}

func TestLeaseServiceFailsClosedNearExpiryWhenServerTimeIsUncertain(t *testing.T) {
	started := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)
	store := newMemoryClaimStore(started)
	store.uncertainty = time.Minute
	service := newTestLeaseService(store)
	claim := mustClaim(t, service, "run-old")

	store.setNow(started.Add(8*time.Minute + time.Second))
	if _, err := service.Renew(context.Background(), claim); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("Renew() error = %v, want ErrLeaseLost", err)
	}
	if _, err := service.Takeover(context.Background(), ClaimRequest{
		RepoNodeID: "R_repo", IssueNumber: 31, RunID: "run-new", MachineID: "machine-run-new", ActorID: 1, StateRevision: 7, BaseSHA: "abc123",
	}); !errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("early Takeover() error = %v, want ErrAlreadyClaimed", err)
	}

	store.setNow(started.Add(10*time.Minute + time.Second))
	if _, err := service.Takeover(context.Background(), ClaimRequest{
		RepoNodeID: "R_repo", IssueNumber: 31, RunID: "run-new", MachineID: "machine-run-new", ActorID: 1, StateRevision: 7, BaseSHA: "abc123",
	}); err != nil {
		t.Fatalf("safe Takeover() error = %v", err)
	}
}

func newTestLeaseService(store ClaimStore) LeaseService {
	var tokenSequence atomic.Int64
	return LeaseService{
		Store: store,
		NewToken: func() (string, error) {
			return fmt.Sprintf("token-%d", tokenSequence.Add(1)), nil
		},
		TTL: 10 * time.Minute,
	}
}

func mustClaim(t *testing.T, service LeaseService, runID string) StoredClaim {
	t.Helper()
	claim, err := service.Claim(context.Background(), ClaimRequest{
		RepoNodeID: "R_repo", IssueNumber: 31, RunID: runID, MachineID: "machine-" + runID, ActorID: 1, StateRevision: 7, BaseSHA: "abc123",
	})
	if err != nil {
		t.Fatal(err)
	}
	return claim
}

type memoryClaimStore struct {
	mu          sync.Mutex
	now         time.Time
	uncertainty time.Duration
	claim       Claim
	oid         string
	exists      bool
	nextOID     int
}

func newMemoryClaimStore(now time.Time) *memoryClaimStore {
	return &memoryClaimStore{now: now}
}

func (store *memoryClaimStore) setNow(now time.Time) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.now = now
}

func (store *memoryClaimStore) ServerNow(context.Context) (time.Time, time.Duration, error) {
	return store.now, store.uncertainty, nil
}

func (store *memoryClaimStore) Read(context.Context, string, int) (StoredClaim, bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.exists {
		return StoredClaim{}, false, nil
	}
	return StoredClaim{OID: store.oid, Claim: store.claim}, true, nil
}

func (store *memoryClaimStore) Create(_ context.Context, claim Claim) (StoredClaim, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.exists {
		return StoredClaim{}, ErrRefConflict
	}
	store.nextOID++
	store.oid = fmt.Sprintf("oid-%d", store.nextOID)
	store.claim = claim
	store.exists = true
	return StoredClaim{OID: store.oid, Claim: claim}, nil
}

func (store *memoryClaimStore) CompareAndSwap(_ context.Context, expectedOID string, claim Claim) (StoredClaim, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.exists || store.oid != expectedOID {
		return StoredClaim{}, ErrRefConflict
	}
	store.nextOID++
	store.oid = fmt.Sprintf("oid-%d", store.nextOID)
	store.claim = claim
	return StoredClaim{OID: store.oid, Claim: claim}, nil
}

func (store *memoryClaimStore) Delete(_ context.Context, _ string, _ int, expectedOID string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.exists || store.oid != expectedOID {
		return ErrRefConflict
	}
	store.exists = false
	return nil
}
