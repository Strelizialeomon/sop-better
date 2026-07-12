package task

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var workflowPushPattern = regexp.MustCompile(`(?m)^\s*(?:push\s*:|on\s*:\s*push\s*$|on\s*:\s*\[[^\]]*push[^\]]*\])`)

func CheckClaimWorkflowIsolation(repositoryPath string) error {
	directory := filepath.Join(repositoryPath, ".github", "workflows")
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read GitHub workflows for claim isolation: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if extension != ".yml" && extension != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return fmt.Errorf("read workflow %s: %w", entry.Name(), err)
		}
		text := string(data)
		if !workflowPushPattern.MatchString(text) {
			continue
		}
		lower := strings.ToLower(text)
		if !strings.Contains(lower, "branches-ignore:") || !strings.Contains(lower, "sop/claims/**") {
			return fmt.Errorf("workflow %s listens to push but does not explicitly branches-ignore sop/claims/**", entry.Name())
		}
	}
	return nil
}
