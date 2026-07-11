package engine

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Strelizialeomon/sop-better/internal/state"
)

func TestGeneratedGitignoreCoversProjectTransactionResidue(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate transaction test")
	}
	repositoryRoot := filepath.Dir(filepath.Dir(filepath.Dir(source)))
	data, err := os.ReadFile(filepath.Join(repositoryRoot, "master", "base", "gitignore-block.txt"))
	if err != nil {
		t.Fatal(err)
	}
	lines := make(map[string]struct{})
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		lines[strings.TrimSpace(line)] = struct{}{}
	}
	for _, runtimePath := range []string{transactionJournalPath, transactionDataPath + "/"} {
		if _, ok := lines[runtimePath]; !ok {
			t.Errorf("generated .gitignore does not cover runtime transaction path %q", runtimePath)
		}
	}
}

func TestApplyTransactionRestoresEveryFileWhenAReplaceFails(t *testing.T) {
	root := t.TempDir()
	writeTransactionFixture(t, root, "one.md", "old one\n")
	writeTransactionFixture(t, root, "nested/two.md", "old two\n")

	err := applyTransaction(root, []transactionFile{
		{Target: "one.md", Content: []byte("new one\n"), Mode: 0o644},
		{Target: "nested/two.md", Content: []byte("new two\n"), Mode: 0o644},
	}, transactionOptions{
		AfterReplace: func(replaced int) error {
			if replaced == 1 {
				return errors.New("injected replace failure")
			}
			return nil
		},
	})
	if err == nil {
		t.Fatal("transaction unexpectedly succeeded")
	}
	assertTransactionFixture(t, root, "one.md", "old one\n")
	assertTransactionFixture(t, root, "nested/two.md", "old two\n")
	assertNoTransactionResidue(t, root)
}

func TestConcurrentTransactionsReturnBusyInsteadOfSharingJournal(t *testing.T) {
	root := t.TempDir()
	writeTransactionFixture(t, root, "one.md", "old one\n")
	writeTransactionFixture(t, root, "two.md", "old two\n")
	firstReplaced := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstResult := make(chan error, 1)
	go func() {
		firstResult <- applyTransaction(root, []transactionFile{
			{Target: "one.md", Content: []byte("first one\n"), Mode: 0o644},
			{Target: "two.md", Content: []byte("first two\n"), Mode: 0o644},
		}, transactionOptions{AfterReplace: func(replaced int) error {
			if replaced == 1 {
				close(firstReplaced)
				<-releaseFirst
			}
			return nil
		}})
	}()
	select {
	case <-firstReplaced:
	case <-time.After(5 * time.Second):
		t.Fatal("first transaction did not reach the barrier")
	}

	secondResult := make(chan error, 1)
	go func() {
		secondResult <- applyTransaction(root, []transactionFile{
			{Target: "one.md", Content: []byte("second one\n"), Mode: 0o644},
			{Target: "two.md", Content: []byte("second two\n"), Mode: 0o644},
		}, transactionOptions{})
	}()
	var secondErr error
	select {
	case secondErr = <-secondResult:
	case <-time.After(5 * time.Second):
		close(releaseFirst)
		t.Fatal("second transaction did not return busy")
	}
	if secondErr == nil || !strings.Contains(secondErr.Error(), "project operation is already running") {
		close(releaseFirst)
		t.Fatalf("second transaction error = %v, want busy", secondErr)
	}
	close(releaseFirst)
	if err := <-firstResult; err != nil {
		t.Fatalf("first transaction failed: %v", err)
	}
	assertTransactionFixture(t, root, "one.md", "first one\n")
	assertTransactionFixture(t, root, "two.md", "first two\n")
	assertNoTransactionResidue(t, root)
}

func TestProjectTransactionUsesCrashSafeStateLock(t *testing.T) {
	root := t.TempDir()
	stateHome := t.TempDir()
	t.Setenv("SOP_STATE_HOME", stateHome)
	writeTransactionFixture(t, root, "one.md", "old one\n")
	projectID, err := projectIdentifier(root)
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(stateHome, "projects", projectID, "operation.lock")
	lock, err := state.AcquireFileLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()

	err = applyTransaction(root, []transactionFile{{
		Target: "one.md", Content: []byte("new one\n"), Mode: 0o644,
	}}, transactionOptions{})
	if err == nil || !strings.Contains(err.Error(), "project operation is already running") {
		t.Fatalf("applyTransaction() error = %v, want busy state lock", err)
	}
	assertTransactionFixture(t, root, "one.md", "old one\n")
}

func TestBeforeCommitValidationFailureRestoresTransaction(t *testing.T) {
	root := t.TempDir()
	writeTransactionFixture(t, root, "one.md", "old one\n")
	writeTransactionFixture(t, root, "two.md", "old two\n")
	err := applyTransaction(root, []transactionFile{
		{Target: "one.md", Content: []byte("new one\n"), Mode: 0o644},
		{Target: "two.md", Content: []byte("new two\n"), Mode: 0o644},
	}, transactionOptions{BeforeCommit: func() error {
		return errors.New("candidate verification failed")
	}})
	if err == nil || !strings.Contains(err.Error(), "candidate verification failed") {
		t.Fatalf("transaction error = %v, want verification failure", err)
	}
	assertTransactionFixture(t, root, "one.md", "old one\n")
	assertTransactionFixture(t, root, "two.md", "old two\n")
	assertNoTransactionResidue(t, root)
}

func TestRecoverTransactionRepairsAnInterruptedWriteBeforeNextMutation(t *testing.T) {
	root := t.TempDir()
	writeTransactionFixture(t, root, "one.md", "old one\n")
	writeTransactionFixture(t, root, "nested/two.md", "old two\n")

	err := applyTransaction(root, []transactionFile{
		{Target: "one.md", Content: []byte("interrupted one\n"), Mode: 0o644},
		{Target: "nested/two.md", Content: []byte("interrupted two\n"), Mode: 0o644},
	}, transactionOptions{
		AfterReplace: func(replaced int) error {
			if replaced == 1 {
				return errors.New("simulated process death")
			}
			return nil
		},
		LeaveInterrupted: true,
	})
	if err == nil {
		t.Fatal("interrupted transaction unexpectedly succeeded")
	}
	assertTransactionFixture(t, root, "one.md", "interrupted one\n")

	if err := recoverTransaction(root); err != nil {
		t.Fatalf("recover transaction: %v", err)
	}
	assertTransactionFixture(t, root, "one.md", "old one\n")
	assertTransactionFixture(t, root, "nested/two.md", "old two\n")
	assertNoTransactionResidue(t, root)
}

func TestRecoverTransactionRejectsForgedBackupPathWithoutChangingTarget(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SOP_STATE_HOME", t.TempDir())
	writeTransactionFixture(t, root, "AGENTS.md", "safe agents\n")
	writeTransactionFixture(t, root, "README.md", "forged backup\n")
	writeTransactionFixture(t, root, ".sop/transaction-data/candidates/0000", "candidate\n")
	writeForgedTransactionJournal(t, root, transactionJournal{Format: 1, Entries: []transactionJournalEntry{{
		Target: "AGENTS.md", Candidate: ".sop/transaction-data/candidates/0000", Backup: "README.md",
		Existed: true, Mode: 0o644,
	}}})
	authorizeForgedTransactionJournal(t, root)

	err := recoverTransaction(root)
	if err == nil || !strings.Contains(err.Error(), "backup") {
		t.Fatalf("recoverTransaction() error = %v, want forged backup rejection", err)
	}
	assertTransactionFixture(t, root, "AGENTS.md", "safe agents\n")
}

func TestRecoverTransactionRejectsUnauthenticatedJournalBeforeChangingGitConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SOP_STATE_HOME", t.TempDir())
	writeTransactionFixture(t, root, ".git/config", "[core]\n\trepositoryformatversion = 0\n")
	writeTransactionFixture(t, root, ".sop/transaction-data/backups/0000", "[core]\n\thooksPath = attacker-hooks\n")
	writeTransactionFixture(t, root, ".sop/transaction-data/candidates/0000", "candidate\n")
	writeForgedTransactionJournal(t, root, transactionJournal{Format: 1, Entries: []transactionJournalEntry{{
		Target: ".git/config", Candidate: ".sop/transaction-data/candidates/0000",
		Backup: ".sop/transaction-data/backups/0000", Existed: true, Mode: 0o644,
	}}})

	err := recoverTransaction(root)
	if err == nil || !strings.Contains(err.Error(), "authorization") {
		t.Fatalf("recoverTransaction() error = %v, want missing authorization rejection", err)
	}
	assertTransactionFixture(t, root, ".git/config", "[core]\n\trepositoryformatversion = 0\n")
}

func TestRecoverTransactionRejectsSymlinkEscapeBeforeWritingOutsideProject(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SOP_STATE_HOME", t.TempDir())
	external := t.TempDir()
	writeTransactionFixture(t, external, "victim.txt", "external sentinel\n")
	if err := os.Symlink(external, filepath.Join(root, "escape")); err != nil {
		t.Skipf("create symlink: %v", err)
	}
	writeTransactionFixture(t, root, ".sop/transaction-data/backups/0000", "forged external content\n")
	writeTransactionFixture(t, root, ".sop/transaction-data/candidates/0000", "candidate\n")
	writeForgedTransactionJournal(t, root, transactionJournal{Format: 1, Entries: []transactionJournalEntry{{
		Target: "escape/victim.txt", Candidate: ".sop/transaction-data/candidates/0000",
		Backup: ".sop/transaction-data/backups/0000", Existed: true, Mode: 0o644,
	}}})
	authorizeForgedTransactionJournal(t, root)

	err := recoverTransaction(root)
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("recoverTransaction() error = %v, want symlink rejection", err)
	}
	assertTransactionFixture(t, external, "victim.txt", "external sentinel\n")
}

func TestRecoverTransactionValidatesWholeJournalBeforeRestoringAnyEntry(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SOP_STATE_HOME", t.TempDir())
	writeTransactionFixture(t, root, "safe.md", "partial write\n")
	writeTransactionFixture(t, root, ".sop/transaction-data/backups/0001", "original safe\n")
	writeTransactionFixture(t, root, ".sop/transaction-data/candidates/0001", "candidate safe\n")
	writeTransactionFixture(t, root, ".sop/transaction-data/candidates/0000", "candidate unsafe\n")
	writeForgedTransactionJournal(t, root, transactionJournal{Format: 1, Entries: []transactionJournalEntry{
		{Target: "../escape", Candidate: ".sop/transaction-data/candidates/0000", Mode: 0o644},
		{Target: "safe.md", Candidate: ".sop/transaction-data/candidates/0001", Backup: ".sop/transaction-data/backups/0001", Existed: true, Mode: 0o644},
	}})
	authorizeForgedTransactionJournal(t, root)

	if err := recoverTransaction(root); err == nil {
		t.Fatal("recoverTransaction() unexpectedly accepted unsafe journal")
	}
	assertTransactionFixture(t, root, "safe.md", "partial write\n")
}

func writeForgedTransactionJournal(t *testing.T, root string, journal transactionJournal) {
	t.Helper()
	if journal.Authorization == "" {
		journal.Authorization = strings.Repeat("a", 64)
	}
	for index := range journal.Entries {
		entry := &journal.Entries[index]
		if !entry.Remove {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(entry.Candidate)))
			if err != nil {
				t.Fatal(err)
			}
			entry.CandidateSHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
		}
		if entry.Existed {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(entry.Backup)))
			if err != nil {
				t.Fatal(err)
			}
			entry.BackupSHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
		}
	}
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	path := filepath.Join(root, filepath.FromSlash(transactionJournalPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func authorizeForgedTransactionJournal(t *testing.T, root string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(transactionJournalPath)))
	if err != nil {
		t.Fatal(err)
	}
	journal, err := decodeTransactionJournal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeTransactionAuthorization(root, journal.Authorization, data); err != nil {
		t.Fatal(err)
	}
}

func writeTransactionFixture(t *testing.T, root string, target string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(target))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertTransactionFixture(t *testing.T, root string, target string, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(target)))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", target, data, want)
	}
}

func assertNoTransactionResidue(t *testing.T, root string) {
	t.Helper()
	for _, target := range []string{".sop/transaction.json", ".sop/transaction-data"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(target))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("transaction residue %s: %v", target, err)
		}
	}
	authorizationPath, err := transactionAuthorizationPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(authorizationPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("transaction authorization residue %s: %v", authorizationPath, err)
	}
}
