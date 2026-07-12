package task

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Workspace struct {
	Path    string
	Branch  string
	Resumed bool
}

type WorkspaceManager struct {
	Root      string
	GitBinary string
}

func (manager WorkspaceManager) Prepare(ctx context.Context, repository string, issueNumber int, baseRevision string) (Workspace, error) {
	if strings.TrimSpace(manager.Root) == "" {
		return Workspace{}, errors.New("workspace root is required")
	}
	if issueNumber <= 0 {
		return Workspace{}, errors.New("workspace issue number must be positive")
	}
	if strings.TrimSpace(baseRevision) == "" {
		return Workspace{}, errors.New("workspace base revision is required")
	}
	repositoryID, repositoryPath, err := canonicalRepositoryID(repository)
	if err != nil {
		return Workspace{}, err
	}
	branch := fmt.Sprintf("sop/issue-%d", issueNumber)
	path := filepath.Join(manager.Root, repositoryID, fmt.Sprintf("issue-%d", issueNumber))
	if _, err := os.Stat(path); err == nil {
		current, commandErr := manager.git(ctx, path, "branch", "--show-current")
		if commandErr != nil {
			return Workspace{}, fmt.Errorf("inspect existing issue workspace: %w", commandErr)
		}
		if strings.TrimSpace(current) != branch {
			return Workspace{}, fmt.Errorf("existing issue workspace uses branch %q, want %q", strings.TrimSpace(current), branch)
		}
		return Workspace{Path: path, Branch: branch, Resumed: true}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return Workspace{}, fmt.Errorf("inspect issue workspace: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Workspace{}, fmt.Errorf("create workspace parent: %w", err)
	}
	if _, err := manager.git(ctx, repositoryPath, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		if _, err := manager.git(ctx, repositoryPath, "worktree", "add", path, branch); err != nil {
			return Workspace{}, fmt.Errorf("restore per-issue worktree: %w", err)
		}
		return Workspace{Path: path, Branch: branch, Resumed: true}, nil
	}
	if _, err := manager.git(ctx, repositoryPath, "worktree", "add", "-b", branch, path, baseRevision); err != nil {
		return Workspace{}, fmt.Errorf("create per-issue worktree: %w", err)
	}
	return Workspace{Path: path, Branch: branch}, nil
}

func canonicalRepositoryID(repository string) (string, string, error) {
	absolute, err := filepath.Abs(repository)
	if err != nil {
		return "", "", fmt.Errorf("resolve repository path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", "", fmt.Errorf("resolve repository identity: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", "", fmt.Errorf("inspect repository: %w", err)
	}
	if !info.IsDir() {
		return "", "", errors.New("repository path is not a directory")
	}
	digest := sha256.Sum256([]byte(filepath.Clean(resolved)))
	return fmt.Sprintf("%x", digest[:8]), resolved, nil
}

func (manager WorkspaceManager) git(ctx context.Context, directory string, args ...string) (string, error) {
	binary := manager.GitBinary
	if binary == "" {
		binary = "git"
	}
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
