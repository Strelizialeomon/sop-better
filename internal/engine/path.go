package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func rejectManagedSymlinkTraversal(projectRoot string, target string) error {
	if !safeProjectTarget(target) {
		return fmt.Errorf("%s: managed target must be repository-relative", target)
	}
	current := projectRoot
	var traversed []string
	for _, part := range strings.Split(filepath.ToSlash(target), "/") {
		if part == "" || part == "." {
			continue
		}
		traversed = append(traversed, part)
		current = filepath.Join(current, filepath.FromSlash(part))
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: inspect managed target: %w", target, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s: managed target traverses symbolic link %s", target, filepath.ToSlash(filepath.Join(traversed...)))
		}
	}
	return nil
}
