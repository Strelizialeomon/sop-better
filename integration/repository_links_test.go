package integration_test

import (
	"bufio"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var repositoryMarkdownLink = regexp.MustCompile(`!?\[[^\]]*\]\(([^)\s]+)(?:\s+[^)]*)?\)`)

func TestRepositoryDocumentationLocalLinksResolve(t *testing.T) {
	repoRoot := repositoryRoot(t)
	paths := []string{
		filepath.Join(repoRoot, "AGENTS.md"),
		filepath.Join(repoRoot, "README.md"),
		filepath.Join(repoRoot, "STANDARD.md"),
		filepath.Join(repoRoot, "PLAYBOOK.md"),
		filepath.Join(repoRoot, "docs"),
		filepath.Join(repoRoot, "experiments"),
	}
	for _, root := range paths {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".md" {
				return nil
			}
			checkRepositoryMarkdownFile(t, repoRoot, path)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func checkRepositoryMarkdownFile(t *testing.T, repoRoot string, path string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	inFence := false
	lineNumber := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		for _, match := range repositoryMarkdownLink.FindAllStringSubmatch(line, -1) {
			raw := strings.Trim(match[1], "<>")
			if raw == "" || strings.HasPrefix(raw, "#") || strings.Contains(raw, "://") || strings.HasPrefix(raw, "mailto:") {
				continue
			}
			target, _, _ := strings.Cut(raw, "#")
			target, _, _ = strings.Cut(target, "?")
			decoded, err := url.PathUnescape(target)
			if err != nil {
				t.Errorf("%s:%d has invalid link escaping %q", relativePath(repoRoot, path), lineNumber, raw)
				continue
			}
			resolved := filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(decoded)))
			if _, err := os.Stat(resolved); errors.Is(err, os.ErrNotExist) {
				t.Errorf("%s:%d link %q does not exist", relativePath(repoRoot, path), lineNumber, raw)
			} else if err != nil {
				t.Errorf("%s:%d cannot inspect link %q: %v", relativePath(repoRoot, path), lineNumber, raw, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func relativePath(root string, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(relative)
}
