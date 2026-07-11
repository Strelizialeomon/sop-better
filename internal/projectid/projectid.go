package projectid

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const identifierBytes = 12

// Identifier returns a stable local identifier for a project root. Existing
// roots prefer the filesystem's native object identity so path aliases share
// one project lock and checkpoint directory.
func Identifier(projectRoot string) (string, error) {
	canonical, err := canonicalPath(projectRoot)
	if err != nil {
		return "", err
	}
	identity := "path:" + canonical
	if native, ok := nativeIdentity(canonical); ok {
		identity = "native:" + native
	}
	digest := sha256.Sum256([]byte(identity))
	return fmt.Sprintf("%x", digest[:identifierBytes]), nil
}

func canonicalPath(projectRoot string) (string, error) {
	absolute, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return filepath.Clean(canonical), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("canonicalize project root: %w", err)
	}
	return filepath.Clean(absolute), nil
}
