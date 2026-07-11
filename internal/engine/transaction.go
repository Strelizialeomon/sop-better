package engine

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Strelizialeomon/sop-better/internal/platform"
	"github.com/Strelizialeomon/sop-better/internal/state"
)

const (
	transactionJournalPath = ".sop/transaction.json"
	transactionDataPath    = ".sop/transaction-data"
)

type transactionFile struct {
	Target  string
	Content []byte
	Mode    fs.FileMode
	Remove  bool
}

type transactionOptions struct {
	AfterReplace     func(replaced int) error
	BeforeCommit     func() error
	LeaveInterrupted bool
}

type transactionJournal struct {
	Format        int                       `json:"format"`
	Authorization string                    `json:"authorization"`
	Entries       []transactionJournalEntry `json:"entries"`
}

type transactionJournalEntry struct {
	Target          string `json:"target"`
	Candidate       string `json:"candidate"`
	CandidateSHA256 string `json:"candidate_sha256,omitempty"`
	Backup          string `json:"backup,omitempty"`
	BackupSHA256    string `json:"backup_sha256,omitempty"`
	Existed         bool   `json:"existed"`
	Mode            uint32 `json:"mode"`
	Remove          bool   `json:"remove,omitempty"`
}

type transactionAuthorization struct {
	Format        int    `json:"format"`
	ProjectID     string `json:"project_id"`
	Token         string `json:"token"`
	JournalSHA256 string `json:"journal_sha256"`
}

func applyTransaction(projectRoot string, files []transactionFile, options transactionOptions) error {
	return withProjectOperationLock(projectRoot, func() error {
		return applyTransactionLocked(projectRoot, files, options)
	})
}

// RecoverProject restores the last fully committed project state after an
// interrupted mutation. Mutation entrypoints call it before reading profile
// compatibility fields so a partially replaced profile cannot block recovery.
func RecoverProject(projectRoot string) error {
	return withProjectOperationLock(projectRoot, func() error {
		if err := recoverTransaction(projectRoot); err != nil {
			return fmt.Errorf("recover interrupted project transaction: %w", err)
		}
		return nil
	})
}

func applyTransactionLocked(projectRoot string, files []transactionFile, options transactionOptions) error {
	if err := recoverTransaction(projectRoot); err != nil {
		return fmt.Errorf("recover interrupted transaction before writing: %w", err)
	}
	journal, err := stageTransaction(projectRoot, files)
	if err != nil {
		return err
	}

	for index, entry := range journal.Entries {
		target := filepath.Join(projectRoot, filepath.FromSlash(entry.Target))
		if entry.Remove {
			if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
				return failTransaction(projectRoot, options, fmt.Errorf("remove %s: %w", entry.Target, err))
			}
		} else {
			candidate, err := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(entry.Candidate)))
			if err != nil {
				return failTransaction(projectRoot, options, fmt.Errorf("read staged %s: %w", entry.Target, err))
			}
			if err := writeAtomicFile(target, candidate, fs.FileMode(entry.Mode)); err != nil {
				return failTransaction(projectRoot, options, fmt.Errorf("replace %s: %w", entry.Target, err))
			}
		}
		if options.AfterReplace != nil {
			if err := options.AfterReplace(index + 1); err != nil {
				return failTransaction(projectRoot, options, err)
			}
		}
	}
	if options.BeforeCommit != nil {
		if err := options.BeforeCommit(); err != nil {
			return failTransaction(projectRoot, options, err)
		}
	}

	if err := clearTransaction(projectRoot); err != nil {
		return fmt.Errorf("commit transaction but cleanup failed: %w", err)
	}
	return nil
}

func withProjectOperationLock(projectRoot string, operation func() error) error {
	stateHome, err := projectStateHome()
	if err != nil {
		return err
	}
	projectID, err := projectIdentifier(projectRoot)
	if err != nil {
		return err
	}
	lockPath := filepath.Join(stateHome, "projects", projectID, "operation.lock")
	lock, err := state.AcquireFileLock(lockPath)
	if errors.Is(err, state.ErrLockBusy) {
		return errors.New("project operation is already running; project was not changed")
	}
	if err != nil {
		return fmt.Errorf("acquire project operation lock: %w", err)
	}
	operationErr := operation()
	if closeErr := lock.Close(); closeErr != nil && operationErr == nil {
		return fmt.Errorf("release project operation lock: %w", closeErr)
	}
	return operationErr
}

func ensureProjectIdle(projectRoot string) error {
	if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(transactionJournalPath))); err == nil {
		return errors.New("project has an interrupted transaction; run render or rollback to recover it before continuing")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect transaction journal: %w", err)
	}
	return nil
}

func failTransaction(projectRoot string, options transactionOptions, cause error) error {
	if options.LeaveInterrupted {
		return cause
	}
	if err := recoverTransaction(projectRoot); err != nil {
		return fmt.Errorf("%v; restore previous project state: %w", cause, err)
	}
	return cause
}

func stageTransaction(projectRoot string, files []transactionFile) (transactionJournal, error) {
	for _, internalPath := range []string{transactionJournalPath, transactionDataPath} {
		if err := rejectManagedSymlinkTraversal(projectRoot, internalPath); err != nil {
			return transactionJournal{}, fmt.Errorf("transaction state is unsafe: %w", err)
		}
	}
	dataRoot := filepath.Join(projectRoot, filepath.FromSlash(transactionDataPath))
	if err := os.RemoveAll(dataRoot); err != nil {
		return transactionJournal{}, fmt.Errorf("clear transaction staging: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dataRoot, "candidates"), 0o700); err != nil {
		return transactionJournal{}, fmt.Errorf("create transaction staging: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dataRoot, "backups"), 0o700); err != nil {
		_ = os.RemoveAll(dataRoot)
		return transactionJournal{}, fmt.Errorf("create transaction backups: %w", err)
	}

	authorizationBytes := make([]byte, 32)
	if _, err := rand.Read(authorizationBytes); err != nil {
		_ = os.RemoveAll(dataRoot)
		return transactionJournal{}, fmt.Errorf("generate transaction authorization: %w", err)
	}
	journal := transactionJournal{
		Format:        1,
		Authorization: fmt.Sprintf("%x", authorizationBytes),
		Entries:       make([]transactionJournalEntry, 0, len(files)),
	}
	seen := make(map[string]struct{}, len(files))
	for index, file := range files {
		target := filepath.ToSlash(file.Target)
		if !safeProjectTarget(target) {
			_ = os.RemoveAll(dataRoot)
			return transactionJournal{}, fmt.Errorf("transaction target %q must be repository-relative", target)
		}
		if err := rejectManagedSymlinkTraversal(projectRoot, target); err != nil {
			_ = os.RemoveAll(dataRoot)
			return transactionJournal{}, err
		}
		if _, exists := seen[target]; exists {
			_ = os.RemoveAll(dataRoot)
			return transactionJournal{}, fmt.Errorf("transaction target %q is duplicated", target)
		}
		seen[target] = struct{}{}

		candidateRelative := ""
		if !file.Remove {
			candidateRelative = filepath.ToSlash(filepath.Join(transactionDataPath, "candidates", fmt.Sprintf("%04d", index)))
			if err := os.WriteFile(filepath.Join(projectRoot, filepath.FromSlash(candidateRelative)), file.Content, 0o600); err != nil {
				_ = os.RemoveAll(dataRoot)
				return transactionJournal{}, fmt.Errorf("stage %s: %w", target, err)
			}
		}

		entry := transactionJournalEntry{
			Target:    target,
			Candidate: candidateRelative,
			Mode:      uint32(file.Mode.Perm()),
			Remove:    file.Remove,
		}
		if !file.Remove {
			entry.CandidateSHA256 = fmt.Sprintf("%x", sha256.Sum256(file.Content))
		}
		path := filepath.Join(projectRoot, filepath.FromSlash(target))
		info, err := os.Stat(path)
		switch {
		case err == nil:
			if !info.Mode().IsRegular() {
				_ = os.RemoveAll(dataRoot)
				return transactionJournal{}, fmt.Errorf("transaction target %s is not a regular file", target)
			}
			current, readErr := os.ReadFile(path)
			if readErr != nil {
				_ = os.RemoveAll(dataRoot)
				return transactionJournal{}, fmt.Errorf("backup %s: %w", target, readErr)
			}
			backupRelative := filepath.ToSlash(filepath.Join(transactionDataPath, "backups", fmt.Sprintf("%04d", index)))
			if writeErr := os.WriteFile(filepath.Join(projectRoot, filepath.FromSlash(backupRelative)), current, 0o600); writeErr != nil {
				_ = os.RemoveAll(dataRoot)
				return transactionJournal{}, fmt.Errorf("backup %s: %w", target, writeErr)
			}
			entry.Existed = true
			entry.Backup = backupRelative
			entry.BackupSHA256 = fmt.Sprintf("%x", sha256.Sum256(current))
			entry.Mode = uint32(info.Mode().Perm())
		case errors.Is(err, os.ErrNotExist):
			if entry.Mode == 0 {
				entry.Mode = 0o644
			}
		default:
			_ = os.RemoveAll(dataRoot)
			return transactionJournal{}, fmt.Errorf("inspect %s: %w", target, err)
		}
		journal.Entries = append(journal.Entries, entry)
	}

	journalData, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		_ = os.RemoveAll(dataRoot)
		return transactionJournal{}, fmt.Errorf("encode transaction journal: %w", err)
	}
	journalData = append(journalData, '\n')
	if err := writeTransactionAuthorization(projectRoot, journal.Authorization, journalData); err != nil {
		_ = os.RemoveAll(dataRoot)
		return transactionJournal{}, err
	}
	journalPath := filepath.Join(projectRoot, filepath.FromSlash(transactionJournalPath))
	if err := writeAtomicFile(journalPath, journalData, 0o600); err != nil {
		_ = removeTransactionAuthorization(projectRoot)
		_ = os.RemoveAll(dataRoot)
		return transactionJournal{}, fmt.Errorf("write transaction journal: %w", err)
	}
	return journal, nil
}

func recoverTransaction(projectRoot string) error {
	for _, internalPath := range []string{transactionJournalPath, transactionDataPath} {
		if err := rejectManagedSymlinkTraversal(projectRoot, internalPath); err != nil {
			return fmt.Errorf("transaction state is unsafe: %w", err)
		}
	}
	journalPath := filepath.Join(projectRoot, filepath.FromSlash(transactionJournalPath))
	data, err := os.ReadFile(journalPath)
	if errors.Is(err, os.ErrNotExist) {
		return clearTransaction(projectRoot)
	}
	if err != nil {
		return fmt.Errorf("read transaction journal: %w", err)
	}
	journal, err := decodeTransactionJournal(data)
	if err != nil {
		return fmt.Errorf("parse transaction journal: %w", err)
	}
	if journal.Format != 1 {
		return fmt.Errorf("transaction journal format %d is unsupported", journal.Format)
	}
	if err := verifyTransactionAuthorization(projectRoot, journal, data); err != nil {
		return err
	}
	entries, err := validateTransactionJournal(projectRoot, journal)
	if err != nil {
		return err
	}

	for index := len(entries) - 1; index >= 0; index-- {
		validated := entries[index]
		entry := validated.entry
		target := filepath.Join(projectRoot, filepath.FromSlash(entry.Target))
		if !entry.Existed {
			if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove incomplete %s: %w", entry.Target, err)
			}
			continue
		}
		if err := writeAtomicFile(target, validated.backup, fs.FileMode(entry.Mode)); err != nil {
			return fmt.Errorf("restore %s: %w", entry.Target, err)
		}
	}
	return clearTransaction(projectRoot)
}

type validatedRecoveryEntry struct {
	entry  transactionJournalEntry
	backup []byte
}

func decodeTransactionJournal(data []byte) (transactionJournal, error) {
	var journal transactionJournal
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&journal); err != nil {
		return transactionJournal{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return journal, nil
	} else if err != nil {
		return transactionJournal{}, err
	}
	return transactionJournal{}, errors.New("unexpected trailing JSON value")
}

func validateTransactionJournal(projectRoot string, journal transactionJournal) ([]validatedRecoveryEntry, error) {
	if len(journal.Authorization) != 64 {
		return nil, errors.New("transaction journal authorization is invalid")
	}
	if len(journal.Entries) == 0 {
		return nil, errors.New("transaction journal has no entries")
	}
	validated := make([]validatedRecoveryEntry, len(journal.Entries))
	seenTargets := make(map[string]struct{}, len(journal.Entries))
	for index, entry := range journal.Entries {
		if !safeProjectTarget(entry.Target) || forbiddenTransactionTarget(entry.Target) || entry.Target == transactionJournalPath ||
			entry.Target == transactionDataPath || strings.HasPrefix(entry.Target, transactionDataPath+"/") {
			return nil, fmt.Errorf("transaction journal target %q is unsafe", entry.Target)
		}
		if _, exists := seenTargets[entry.Target]; exists {
			return nil, fmt.Errorf("transaction journal target %q is duplicated", entry.Target)
		}
		seenTargets[entry.Target] = struct{}{}
		if err := rejectManagedSymlinkTraversal(projectRoot, entry.Target); err != nil {
			return nil, fmt.Errorf("transaction journal target %q is unsafe: %w", entry.Target, err)
		}
		if entry.Mode > 0o777 {
			return nil, fmt.Errorf("transaction journal target %q has invalid mode %o", entry.Target, entry.Mode)
		}

		expectedCandidate := filepath.ToSlash(filepath.Join(transactionDataPath, "candidates", fmt.Sprintf("%04d", index)))
		if entry.Remove {
			if entry.Candidate != "" || entry.CandidateSHA256 != "" {
				return nil, fmt.Errorf("transaction journal removal %q must not have a candidate", entry.Target)
			}
		} else {
			if entry.Candidate != expectedCandidate {
				return nil, fmt.Errorf("transaction candidate for %q must be %s", entry.Target, expectedCandidate)
			}
			if err := validateTransactionDataFile(projectRoot, entry.Candidate, entry.CandidateSHA256, "candidate", entry.Target); err != nil {
				return nil, err
			}
		}

		expectedBackup := filepath.ToSlash(filepath.Join(transactionDataPath, "backups", fmt.Sprintf("%04d", index)))
		if !entry.Existed {
			if entry.Backup != "" || entry.BackupSHA256 != "" {
				return nil, fmt.Errorf("transaction journal target %q did not exist but has a backup", entry.Target)
			}
			validated[index] = validatedRecoveryEntry{entry: entry}
			continue
		}
		if entry.Backup != expectedBackup {
			return nil, fmt.Errorf("transaction backup for %q must be %s", entry.Target, expectedBackup)
		}
		if err := validateTransactionDataFile(projectRoot, entry.Backup, entry.BackupSHA256, "backup", entry.Target); err != nil {
			return nil, err
		}
		backup, err := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(entry.Backup)))
		if err != nil {
			return nil, fmt.Errorf("read transaction backup for %s: %w", entry.Target, err)
		}
		validated[index] = validatedRecoveryEntry{entry: entry, backup: backup}
	}
	return validated, nil
}

func validateTransactionDataFile(projectRoot, relativePath, expectedHash, kind, target string) error {
	if len(expectedHash) != 64 {
		return fmt.Errorf("transaction %s hash for %q is invalid", kind, target)
	}
	if err := rejectManagedSymlinkTraversal(projectRoot, relativePath); err != nil {
		return fmt.Errorf("transaction %s for %q is unsafe: %w", kind, target, err)
	}
	info, err := os.Lstat(filepath.Join(projectRoot, filepath.FromSlash(relativePath)))
	if err != nil {
		return fmt.Errorf("inspect transaction %s for %s: %w", kind, target, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("transaction %s for %q is not a regular file", kind, target)
	}
	data, err := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(relativePath)))
	if err != nil {
		return fmt.Errorf("read transaction %s for %s: %w", kind, target, err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != expectedHash {
		return fmt.Errorf("transaction %s checksum mismatch for %q", kind, target)
	}
	return nil
}

func forbiddenTransactionTarget(target string) bool {
	first := strings.ToLower(strings.Split(filepath.ToSlash(target), "/")[0])
	switch first {
	case ".git", ".hg", ".svn", ".jj":
		return true
	default:
		return false
	}
}

func transactionAuthorizationPath(projectRoot string) (string, error) {
	stateHome, err := projectStateHome()
	if err != nil {
		return "", err
	}
	projectID, err := projectIdentifier(projectRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(stateHome, "projects", projectID, "transaction-authorization.json"), nil
}

func writeTransactionAuthorization(projectRoot, token string, journalData []byte) error {
	path, err := transactionAuthorizationPath(projectRoot)
	if err != nil {
		return err
	}
	projectID, err := projectIdentifier(projectRoot)
	if err != nil {
		return err
	}
	authorization := transactionAuthorization{
		Format:        1,
		ProjectID:     projectID,
		Token:         token,
		JournalSHA256: fmt.Sprintf("%x", sha256.Sum256(journalData)),
	}
	data, err := json.MarshalIndent(authorization, "", "  ")
	if err != nil {
		return fmt.Errorf("encode transaction authorization: %w", err)
	}
	data = append(data, '\n')
	if err := platform.AtomicWrite(path, data, 0o600); err != nil {
		return fmt.Errorf("write transaction authorization: %w", err)
	}
	return nil
}

func verifyTransactionAuthorization(projectRoot string, journal transactionJournal, journalData []byte) error {
	path, err := transactionAuthorizationPath(projectRoot)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("transaction recovery authorization is missing; project was not changed")
	}
	if err != nil {
		return fmt.Errorf("read transaction recovery authorization: %w", err)
	}
	var authorization transactionAuthorization
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&authorization); err != nil {
		return fmt.Errorf("parse transaction recovery authorization: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("parse transaction recovery authorization: unexpected trailing JSON value")
		}
		return fmt.Errorf("parse transaction recovery authorization: %w", err)
	}
	projectID, err := projectIdentifier(projectRoot)
	if err != nil {
		return err
	}
	wantJournalHash := fmt.Sprintf("%x", sha256.Sum256(journalData))
	if authorization.Format != 1 || authorization.ProjectID != projectID || authorization.Token != journal.Authorization ||
		authorization.JournalSHA256 != wantJournalHash || len(authorization.Token) != 64 || len(authorization.JournalSHA256) != 64 {
		return errors.New("transaction recovery authorization does not match this journal; project was not changed")
	}
	return nil
}

func removeTransactionAuthorization(projectRoot string) error {
	path, err := transactionAuthorizationPath(projectRoot)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove transaction authorization: %w", err)
	}
	return nil
}

func clearTransaction(projectRoot string) error {
	if err := os.Remove(filepath.Join(projectRoot, filepath.FromSlash(transactionJournalPath))); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.RemoveAll(filepath.Join(projectRoot, filepath.FromSlash(transactionDataPath))); err != nil {
		return err
	}
	return removeTransactionAuthorization(projectRoot)
}

func writeAtomicFile(path string, content []byte, mode fs.FileMode) (returnErr error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".sop-better-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		if returnErr != nil {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode.Perm()); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return nil
}
