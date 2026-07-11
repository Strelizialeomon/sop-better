package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Strelizialeomon/sop-better/internal/platform"
)

type managedLockProvenance struct {
	Format     int    `json:"format"`
	ProjectID  string `json:"project_id"`
	LockSHA256 string `json:"lock_sha256"`
}

func managedLockProvenancePath(projectRoot string) (string, error) {
	stateHome, err := projectStateHome()
	if err != nil {
		return "", err
	}
	projectID, err := projectIdentifier(projectRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(stateHome, "projects", projectID, "managed-lock.json"), nil
}

func isTrustedManagedLock(projectRoot string, lockData []byte) (bool, error) {
	if len(lockData) == 0 {
		return false, nil
	}
	path, err := managedLockProvenancePath(projectRoot)
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read managed lock provenance: %w", err)
	}
	var provenance managedLockProvenance
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&provenance); err != nil {
		return false, fmt.Errorf("parse managed lock provenance: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return false, errors.New("parse managed lock provenance: unexpected trailing JSON value")
		}
		return false, fmt.Errorf("parse managed lock provenance: %w", err)
	}
	projectID, err := projectIdentifier(projectRoot)
	if err != nil {
		return false, err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(lockData))
	if provenance.Format != 1 || provenance.ProjectID != projectID || len(provenance.LockSHA256) != 64 {
		return false, errors.New("managed lock provenance is invalid; stale outputs will not be removed")
	}
	return provenance.LockSHA256 == digest, nil
}

func persistTrustedManagedLock(projectRoot string, lockData []byte) error {
	path, err := managedLockProvenancePath(projectRoot)
	if err != nil {
		return err
	}
	projectID, err := projectIdentifier(projectRoot)
	if err != nil {
		return err
	}
	provenance := managedLockProvenance{
		Format:     1,
		ProjectID:  projectID,
		LockSHA256: fmt.Sprintf("%x", sha256.Sum256(lockData)),
	}
	data, err := json.MarshalIndent(provenance, "", "  ")
	if err != nil {
		return fmt.Errorf("encode managed lock provenance: %w", err)
	}
	data = append(data, '\n')
	if err := platform.AtomicWrite(path, data, 0o600); err != nil {
		return fmt.Errorf("write managed lock provenance: %w", err)
	}
	return nil
}
