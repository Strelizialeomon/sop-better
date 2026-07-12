package task

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceManagerCreatesAndResumesOneWorktreePerIssue(t *testing.T) {
	repo := initWorkspaceTestRepository(t)
	manager := WorkspaceManager{Root: filepath.Join(t.TempDir(), "workspaces")}

	baseSHA := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	first, err := manager.Prepare(context.Background(), repo, 31, baseSHA)
	if err != nil {
		t.Fatal(err)
	}
	if first.Branch != "sop/issue-31" {
		t.Fatalf("branch = %q", first.Branch)
	}
	if _, err := os.Stat(filepath.Join(first.Path, ".git")); err != nil {
		t.Fatalf("worktree .git: %v", err)
	}
	if got := strings.TrimSpace(runGit(t, first.Path, "branch", "--show-current")); got != first.Branch {
		t.Fatalf("checked out branch = %q, want %q", got, first.Branch)
	}

	second, err := manager.Prepare(context.Background(), repo, 31, baseSHA)
	if err != nil {
		t.Fatal(err)
	}
	if second.Path != first.Path || !second.Resumed {
		t.Fatalf("second workspace = %#v, want resumed %#v", second, first)
	}
}

func TestWorkspaceManagerUsesDifferentPathsForDifferentRepositories(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspaces")
	repoOne := initWorkspaceTestRepository(t)
	repoTwo := initWorkspaceTestRepository(t)
	manager := WorkspaceManager{Root: root}
	oneBase := strings.TrimSpace(runGit(t, repoOne, "rev-parse", "HEAD"))
	twoBase := strings.TrimSpace(runGit(t, repoTwo, "rev-parse", "HEAD"))
	one, err := manager.Prepare(context.Background(), repoOne, 31, oneBase)
	if err != nil {
		t.Fatal(err)
	}
	two, err := manager.Prepare(context.Background(), repoTwo, 31, twoBase)
	if err != nil {
		t.Fatal(err)
	}
	if one.Path == two.Path {
		t.Fatalf("repositories collided at %s", one.Path)
	}
}

func TestWorkspaceManagerRestoresMissingWorktreeFromExistingIssueBranch(t *testing.T) {
	repo := initWorkspaceTestRepository(t)
	manager := WorkspaceManager{Root: filepath.Join(t.TempDir(), "workspaces")}
	baseSHA := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	first, err := manager.Prepare(context.Background(), repo, 31, baseSHA)
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "worktree", "remove", first.Path)
	restored, err := manager.Prepare(context.Background(), repo, 31, baseSHA)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.Resumed || restored.Path != first.Path {
		t.Fatalf("restored = %#v", restored)
	}
}

func initWorkspaceTestRepository(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.name", "SOP Test")
	runGit(t, repo, "config", "user.email", "sop@example.invalid")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")
	return repo
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}
