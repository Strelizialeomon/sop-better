package engine

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Strelizialeomon/sop-better/internal/config"
	"github.com/Strelizialeomon/sop-better/internal/projectid"
)

func Diff(projectRoot string, outputs []RenderedOutput, lock Lock, profile config.Profile, profileData []byte) (string, error) {
	var result string
	err := withProjectOperationLock(projectRoot, func() error {
		var err error
		result, err = diffLocked(projectRoot, outputs, lock, profile, profileData)
		return err
	})
	return result, err
}

// DiffUnderHeldLock is the read-only release-preview path used after the
// manager has acquired this project's native operation lock.
func DiffUnderHeldLock(projectRoot, expectedProjectID string, outputs []RenderedOutput, lock Lock, profile config.Profile, profileData []byte) (string, error) {
	actualProjectID, err := projectid.Identifier(projectRoot)
	if err != nil {
		return "", err
	}
	if actualProjectID != expectedProjectID {
		return "", errors.New("manager-held project lock identity does not match project root")
	}
	return diffLocked(projectRoot, outputs, lock, profile, profileData)
}

func diffLocked(projectRoot string, outputs []RenderedOutput, lock Lock, profile config.Profile, profileData []byte) (string, error) {
	if err := ensureProjectIdle(projectRoot); err != nil {
		return "", err
	}
	if err := ValidateCandidateOutputs(projectRoot, outputs, profile); err != nil {
		return "", err
	}
	if len(profileData) == 0 {
		return "", errors.New("candidate profile data is empty")
	}
	currentLockData, readLockErr := os.ReadFile(filepath.Join(projectRoot, ".sop", "lock.json"))
	if readLockErr != nil && !errors.Is(readLockErr, os.ErrNotExist) {
		return "", fmt.Errorf("read .sop/lock.json: %w", readLockErr)
	}
	trustedPreviousLock, err := isTrustedManagedLock(projectRoot, currentLockData)
	if err != nil {
		return "", err
	}
	outputChanges, err := prepareProjectChanges(projectRoot, outputs, trustedPreviousLock)
	if err != nil {
		return "", err
	}
	lockData, err := encodeLock(lock)
	if err != nil {
		return "", err
	}
	changes := make([]projectChange, 0, len(outputChanges)+2)
	for _, candidate := range []projectChange{
		{Target: ".sop/profile.json", Content: profileData},
	} {
		current, readErr := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(candidate.Target)))
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return "", fmt.Errorf("read %s: %w", candidate.Target, readErr)
		}
		if !bytes.Equal(current, candidate.Content) {
			changes = append(changes, candidate)
		}
	}
	changes = append(changes, outputChanges...)
	if !bytes.Equal(currentLockData, lockData) {
		changes = append(changes, projectChange{Target: ".sop/lock.json", Content: lockData})
	}
	for _, change := range changes {
		if err := rejectManagedSymlinkTraversal(projectRoot, change.Target); err != nil {
			return "", err
		}
	}
	report, err := formatProjectChanges(projectRoot, changes)
	if err != nil {
		return "", err
	}
	residues, err := deprecatedRuntimeResidues(projectRoot, profile)
	if err != nil {
		return "", err
	}
	return prependRuntimeMigrationWarnings(report, residues), nil
}

func formatProjectChanges(projectRoot string, changes []projectChange) (string, error) {
	var report strings.Builder
	for _, change := range changes {
		path := filepath.Join(projectRoot, filepath.FromSlash(change.Target))
		current, err := os.ReadFile(path)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return "", fmt.Errorf("read %s: %w", change.Target, err)
			}
			if change.Remove {
				continue
			}
			fmt.Fprintf(&report, "CREATE %s\n%s", change.Target, change.Content)
			continue
		}
		if change.Remove {
			fmt.Fprintf(&report, "DELETE %s\n", change.Target)
			continue
		}
		if bytes.Equal(current, change.Content) {
			continue
		}
		fmt.Fprintf(&report, "UPDATE %s\n--- current/%s\n+++ candidate/%s\n%s", change.Target, change.Target, change.Target, lineDiff(current, change.Content))
	}
	if report.Len() == 0 {
		return "No changes.\n", nil
	}
	return report.String(), nil
}

func lineDiff(current []byte, candidate []byte) string {
	oldLines := splitDiffLines(string(current))
	newLines := splitDiffLines(string(candidate))
	dp := make([][]int, len(oldLines)+1)
	for i := range dp {
		dp[i] = make([]int, len(newLines)+1)
	}
	for i := len(oldLines) - 1; i >= 0; i-- {
		for j := len(newLines) - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var report strings.Builder
	report.WriteString("@@\n")
	for i, j := 0, 0; i < len(oldLines) || j < len(newLines); {
		switch {
		case i < len(oldLines) && j < len(newLines) && oldLines[i] == newLines[j]:
			report.WriteString(" " + oldLines[i] + "\n")
			i++
			j++
		case i < len(oldLines) && (j == len(newLines) || dp[i+1][j] >= dp[i][j+1]):
			report.WriteString("-" + oldLines[i] + "\n")
			i++
		default:
			report.WriteString("+" + newLines[j] + "\n")
			j++
		}
	}
	return report.String()
}

func splitDiffLines(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.TrimSuffix(value, "\n")
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}
