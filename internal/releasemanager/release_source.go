package releasemanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Strelizialeomon/sop-better/internal/platform"
)

const releaseSourceFormat = 1

type releaseSourceConfig struct {
	Format int    `json:"format"`
	Type   string `json:"type"`
	Root   string `json:"root"`
}

func releaseSourcePath(stateHome string) string {
	return filepath.Join(stateHome, "release-source.json")
}

func normalizeLocalReleaseSource(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve local release source: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve local release source symlinks: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("inspect local release source: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("local release source must be a directory")
	}
	return filepath.Clean(resolved), nil
}

func readReleaseSource(stateHome string) (releaseSourceConfig, error) {
	file, err := os.Open(releaseSourcePath(stateHome))
	if err != nil {
		return releaseSourceConfig{}, fmt.Errorf("read release source configuration: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var source releaseSourceConfig
	if err := decoder.Decode(&source); err != nil {
		return releaseSourceConfig{}, fmt.Errorf("parse release source configuration: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values are not allowed")
		}
		return releaseSourceConfig{}, fmt.Errorf("parse release source configuration: %w", err)
	}
	if source.Format != releaseSourceFormat || source.Type != "local" || source.Root == "" || !filepath.IsAbs(source.Root) {
		return releaseSourceConfig{}, errors.New("release source configuration is invalid")
	}
	if filepath.Clean(source.Root) != source.Root {
		return releaseSourceConfig{}, errors.New("release source configuration root is not canonical")
	}
	return source, nil
}

func releaseSourceNeedsCreate(stateHome, root string) (bool, error) {
	info, err := os.Lstat(releaseSourcePath(stateHome))
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, errors.New("existing release source configuration must be a regular file")
	}
	existing, err := readReleaseSource(stateHome)
	if err != nil {
		return false, err
	}
	if existing.Root != root {
		return false, fmt.Errorf("existing release source is %s; refusing to replace it with %s", existing.Root, root)
	}
	return false, nil
}

func installReleaseSource(stateHome, root string) (bool, error) {
	normalized, err := normalizeLocalReleaseSource(root)
	if err != nil {
		return false, err
	}
	if normalized != root {
		return false, errors.New("release source root changed after confirmation")
	}
	created, err := releaseSourceNeedsCreate(stateHome, root)
	if err != nil || !created {
		return created, err
	}
	data, err := json.MarshalIndent(releaseSourceConfig{Format: releaseSourceFormat, Type: "local", Root: root}, "", "  ")
	if err != nil {
		return false, err
	}
	if err := platform.AtomicWrite(releaseSourcePath(stateHome), append(data, '\n'), 0o600); err != nil {
		return false, fmt.Errorf("write release source configuration: %w", err)
	}
	return true, nil
}

func removeCreatedReleaseSource(stateHome, expectedRoot string) error {
	source, err := readReleaseSource(stateHome)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if source.Root != expectedRoot {
		return errors.New("created release source configuration changed during recovery")
	}
	return os.Remove(releaseSourcePath(stateHome))
}

func (manager Manager) releaseSourceRoot() (string, error) {
	if manager.ReleaseSource != "" {
		return normalizeLocalReleaseSource(manager.ReleaseSource)
	}
	source, err := readReleaseSource(manager.StateHome)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errors.New("release source is not configured; rerun sop-install with --release-source PATH or set SOP_RELEASE_SOURCE")
		}
		return "", err
	}
	return normalizeLocalReleaseSource(source.Root)
}
