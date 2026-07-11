package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Strelizialeomon/sop-better/internal/config"
)

func deprecatedRuntimeResidues(projectRoot string, _ config.Profile) ([]string, error) {
	skippedDirectories := map[string]struct{}{
		".git": {}, ".hg": {}, ".svn": {}, ".jj": {}, ".sop": {},
		"node_modules": {}, "vendor": {}, ".venv": {}, "dist": {}, "build": {},
	}
	var residues []string
	err := filepath.WalkDir(projectRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == projectRoot {
			return nil
		}
		name := strings.ToLower(entry.Name())
		if entry.IsDir() {
			if _, skip := skippedDirectories[name]; skip {
				return filepath.SkipDir
			}
		}
		if name != "claude.md" && name != ".claude" {
			return nil
		}
		relative, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return err
		}
		residues = append(residues, filepath.ToSlash(relative))
		if entry.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan deprecated runtime paths: %w", err)
	}
	sort.Strings(residues)
	return residues, nil
}

func rejectDeprecatedRuntimeResidues(projectRoot string, profile config.Profile) error {
	residues, err := deprecatedRuntimeResidues(projectRoot, profile)
	if err != nil {
		return err
	}
	if len(residues) == 0 {
		return nil
	}
	return fmt.Errorf(
		"deprecated Claude runtime residue must be reviewed and removed explicitly before continuing: %s; nothing was deleted",
		strings.Join(residues, ", "),
	)
}

func prependRuntimeMigrationWarnings(report string, residues []string) string {
	if len(residues) == 0 {
		return report
	}
	var warnings strings.Builder
	for _, residue := range residues {
		fmt.Fprintf(&warnings, "MANUAL MIGRATION %s: deprecated Claude runtime residue; review and remove explicitly before render.\n", residue)
	}
	if report == "No changes.\n" {
		report = "No managed changes.\n"
	}
	warnings.WriteString(report)
	return warnings.String()
}
