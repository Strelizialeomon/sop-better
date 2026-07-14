package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCompletionEvidenceRejectsAgentWrittenReviewFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.json")
	data := `{"acceptance_verified":true,"pull_request_url":"https://github.com/acme/repo/pull/31","review_completed":true}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	var input completionInput
	err := loadCompletionEvidence(path, &input)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("loadCompletionEvidence() error = %v", err)
	}
}

func TestRunTaskReviewRequiresPullRequest(t *testing.T) {
	err := runTask([]string{"review", "31", "--project-root", t.TempDir()}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "--pull-request") {
		t.Fatalf("runTask() error = %v", err)
	}
}

func TestPrepareFinalVerificationWorkspaceChecksOutMergedDefaultBranchCommit(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	repository := filepath.Join(root, "source")
	runTaskGit(t, root, "init", "--bare", remote)
	runTaskGit(t, root, "init", "-b", "main", repository)
	runTaskGit(t, repository, "config", "user.name", "SOP Test")
	runTaskGit(t, repository, "config", "user.email", "sop@example.invalid")
	if err := os.WriteFile(filepath.Join(repository, "result.txt"), []byte("merged\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTaskGit(t, repository, "add", "result.txt")
	runTaskGit(t, repository, "commit", "-m", "merged result")
	commitSHA := strings.TrimSpace(runTaskGit(t, repository, "rev-parse", "HEAD"))
	runTaskGit(t, repository, "remote", "add", "origin", remote)
	runTaskGit(t, repository, "push", "-u", "origin", "main")

	workspace, cleanup, err := prepareFinalVerificationWorkspace(context.Background(), repository, root, "main", commitSHA)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(runTaskGit(t, workspace, "rev-parse", "HEAD")); got != commitSHA {
		t.Fatalf("verification HEAD = %s, want %s", got, commitSHA)
	}
	cleanup()
	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatalf("verification workspace was not cleaned up: %v", err)
	}
}

func TestPrepareReviewWorkspacesRejectsDirtySource(t *testing.T) {
	repository := initTaskTestRepository(t)
	headSHA := strings.TrimSpace(runTaskGit(t, repository, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(repository, "scratch.txt"), []byte("not committed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := prepareReviewWorkspaces(context.Background(), repository, t.TempDir(), headSHA)
	if err == nil || !strings.Contains(err.Error(), "clean committed source workspace") {
		t.Fatalf("prepareReviewWorkspaces() error = %v, want dirty source rejection", err)
	}
}

func TestPrepareReviewWorkspacesCreatesTwoDetachedExactHeadWorktrees(t *testing.T) {
	repository := initTaskTestRepository(t)
	headSHA := strings.TrimSpace(runTaskGit(t, repository, "rev-parse", "HEAD"))
	workspaces, cleanup, err := prepareReviewWorkspaces(context.Background(), repository, t.TempDir(), headSHA)
	if err != nil {
		t.Fatal(err)
	}
	for name, workspace := range map[string]string{"checks": workspaces.ChecksPath, "reviewer": workspaces.ReviewerPath} {
		if got := strings.TrimSpace(runTaskGit(t, workspace, "rev-parse", "HEAD")); got != headSHA {
			t.Fatalf("%s HEAD = %s, want %s", name, got, headSHA)
		}
		if got := strings.TrimSpace(runTaskGit(t, workspace, "branch", "--show-current")); got != "" {
			t.Fatalf("%s branch = %q, want detached", name, got)
		}
	}
	if workspaces.ChecksPath == workspaces.ReviewerPath {
		t.Fatal("checks and reviewer unexpectedly share one workspace")
	}
	cleanup()
	for _, workspace := range []string{workspaces.ChecksPath, workspaces.ReviewerPath} {
		if _, err := os.Stat(workspace); !os.IsNotExist(err) {
			t.Fatalf("review workspace was not cleaned up: %s: %v", workspace, err)
		}
	}
}

func TestGitChangedPathsIncludesBothSidesOfRename(t *testing.T) {
	repository := t.TempDir()
	runTaskGit(t, repository, "init", "-b", "main")
	runTaskGit(t, repository, "config", "user.name", "SOP Test")
	runTaskGit(t, repository, "config", "user.email", "sop@example.invalid")
	if err := os.MkdirAll(filepath.Join(repository, "outside"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "outside", "contract.go"), []byte("package outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "outside", "type.go"), []byte("package outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTaskGit(t, repository, "add", ".")
	runTaskGit(t, repository, "commit", "-m", "base")
	baseSHA := strings.TrimSpace(runTaskGit(t, repository, "rev-parse", "HEAD"))
	if err := os.MkdirAll(filepath.Join(repository, "allowed"), 0o700); err != nil {
		t.Fatal(err)
	}
	runTaskGit(t, repository, "mv", "outside/contract.go", "allowed/contract.go")
	runTaskGit(t, repository, "commit", "-m", "rename")
	headSHA := strings.TrimSpace(runTaskGit(t, repository, "rev-parse", "HEAD"))

	paths, err := gitChangedPaths(context.Background(), repository, baseSHA, headSHA)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(paths, ","), "allowed/contract.go,outside/contract.go"; got != want {
		t.Fatalf("changed paths = %q, want %q", got, want)
	}

	blobSHA := strings.TrimSpace(runTaskGit(t, repository, "rev-parse", "HEAD:outside/type.go"))
	runTaskGit(t, repository, "update-index", "--cacheinfo", "120000,"+blobSHA+",outside/type.go")
	runTaskGit(t, repository, "commit", "-m", "change file type")
	typeHeadSHA := strings.TrimSpace(runTaskGit(t, repository, "rev-parse", "HEAD"))
	paths, err = gitChangedPaths(context.Background(), repository, headSHA, typeHeadSHA)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(paths, ","), "outside/type.go"; got != want {
		t.Fatalf("type-change paths = %q, want %q", got, want)
	}
}

func runTaskGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func initTaskTestRepository(t *testing.T) string {
	t.Helper()
	repository := t.TempDir()
	runTaskGit(t, repository, "init", "-b", "main")
	runTaskGit(t, repository, "config", "user.name", "SOP Test")
	runTaskGit(t, repository, "config", "user.email", "sop@example.invalid")
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTaskGit(t, repository, "add", "README.md")
	runTaskGit(t, repository, "commit", "-m", "base")
	return repository
}
