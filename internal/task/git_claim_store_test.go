package task

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestGitClaimStoreMakesConcurrentRemoteClaimAtomic(t *testing.T) {
	remote := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, t.TempDir(), "init", "--bare", remote)
	cloneOne := cloneClaimRepository(t, remote)
	cloneTwo := cloneClaimRepository(t, remote)
	now := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)
	stores := []*GitClaimStore{
		{RepositoryPath: cloneOne, Remote: "origin", Clock: fixedServerClock(now)},
		{RepositoryPath: cloneTwo, Remote: "origin", Clock: fixedServerClock(now)},
	}

	var wait sync.WaitGroup
	errorsByStore := make([]error, len(stores))
	for index, store := range stores {
		wait.Add(1)
		go func(index int, store *GitClaimStore) {
			defer wait.Done()
			_, errorsByStore[index] = store.Create(context.Background(), Claim{
				RepoNodeID: "R_repo", IssueNumber: 31, RunID: "run", MachineID: "machine", ActorID: int64(index + 1),
				LeaseEpoch: 1, FencingToken: "fence", ServerObserved: now, LeaseExpiresAt: now.Add(time.Minute), BaseSHA: "abc123",
			})
		}(index, store)
	}
	wait.Wait()
	successes, conflicts := 0, 0
	for _, err := range errorsByStore {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrRefConflict):
			conflicts++
		default:
			t.Fatalf("Create() error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d errors=%v", successes, conflicts, errorsByStore)
	}
}

func TestGitClaimStoreDeleteUsesExpectedOID(t *testing.T) {
	remote := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, t.TempDir(), "init", "--bare", remote)
	repo := cloneClaimRepository(t, remote)
	now := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)
	store := &GitClaimStore{RepositoryPath: repo, Remote: "origin", Clock: fixedServerClock(now)}
	first, err := store.Create(context.Background(), Claim{RepoNodeID: "R_repo", IssueNumber: 31, RunID: "run-1", MachineID: "mac", ActorID: 1, LeaseEpoch: 1, FencingToken: "one", ServerObserved: now, LeaseExpiresAt: now.Add(time.Minute), BaseSHA: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CompareAndSwap(context.Background(), first.OID, Claim{RepoNodeID: "R_repo", IssueNumber: 31, RunID: "run-2", MachineID: "win", ActorID: 2, LeaseEpoch: 2, FencingToken: "two", ServerObserved: now, LeaseExpiresAt: now.Add(time.Minute), BaseSHA: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), "R_repo", 31, first.OID); !errors.Is(err, ErrRefConflict) {
		t.Fatalf("stale Delete() error = %v, want ErrRefConflict", err)
	}
	if err := store.Delete(context.Background(), "R_repo", 31, second.OID); err != nil {
		t.Fatalf("current Delete() error = %v", err)
	}
}

func TestGitClaimStoreCASCannotRecreateAReleasedClaim(t *testing.T) {
	remote := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, t.TempDir(), "init", "--bare", remote)
	repo := cloneClaimRepository(t, remote)
	now := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)
	store := &GitClaimStore{RepositoryPath: repo, Remote: "origin", Clock: fixedServerClock(now)}
	first, err := store.Create(context.Background(), Claim{RepoNodeID: "R_repo", IssueNumber: 31, RunID: "run-1", MachineID: "mac", ActorID: 1, LeaseEpoch: 1, FencingToken: "one", ServerObserved: now, LeaseExpiresAt: now.Add(time.Minute), BaseSHA: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), "R_repo", 31, first.OID); err != nil {
		t.Fatal(err)
	}
	_, err = store.CompareAndSwap(context.Background(), first.OID, Claim{RepoNodeID: "R_repo", IssueNumber: 31, RunID: "run-1", MachineID: "mac", ActorID: 1, LeaseEpoch: 1, FencingToken: "one", ServerObserved: now, LeaseExpiresAt: now.Add(2 * time.Minute), BaseSHA: "abc"})
	if !errors.Is(err, ErrRefConflict) {
		t.Fatalf("CAS after delete error = %v, want ErrRefConflict", err)
	}
	if _, exists, err := store.Read(context.Background(), "R_repo", 31); err != nil || exists {
		t.Fatalf("released claim was resurrected: exists=%v err=%v", exists, err)
	}
}

func cloneClaimRepository(t *testing.T, remote string) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	command := exec.Command("git", "clone", remote, repo)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, output)
	}
	runGit(t, repo, "config", "user.name", "SOP Test")
	runGit(t, repo, "config", "user.email", "sop@example.invalid")
	return repo
}

func fixedServerClock(now time.Time) ServerClock {
	return func(context.Context) (time.Time, time.Duration, error) { return now, 0, nil }
}
