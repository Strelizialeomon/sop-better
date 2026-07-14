package task

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExactHeadCheckExecutorRejectsTrackedMutation(t *testing.T) {
	repository := initWorkspaceTestRepository(t)
	headSHA := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
	command := "printf changed > README.md"
	if runtime.GOOS == "windows" {
		command = `Set-Content -Path README.md -Value changed`
	}
	executor := ExactHeadCheckExecutor{Inner: ShellCheckExecutor{}}
	_, err := executor.Run(context.Background(), repository, headSHA, map[string][]string{"test": {command}}, []string{"test"})
	if err == nil || !strings.Contains(err.Error(), "tracked files changed") {
		t.Fatalf("guarded check error = %v, want tracked mutation rejection", err)
	}
}

func TestExactHeadReviewExecutorRejectsWrongHeadAndTrackedMutation(t *testing.T) {
	repository := initWorkspaceTestRepository(t)
	headSHA := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
	called := false
	executor := ExactHeadReviewExecutor{Inner: reviewExecutorFunc(func(context.Context, ReviewExecutionRequest) (ReviewExecutionResult, error) {
		called = true
		if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("changed\n"), 0o600); err != nil {
			return ReviewExecutionResult{}, err
		}
		return ReviewExecutionResult{Reference: "codex-review://test"}, nil
	})}
	request := ReviewExecutionRequest{Workspace: repository, BaseSHA: "base", HeadSHA: "wrong", Mode: ReviewFull, Prompt: "review"}
	if _, err := executor.Execute(context.Background(), request); err == nil || !strings.Contains(err.Error(), "HEAD") {
		t.Fatalf("wrong-head review error = %v", err)
	}
	if called {
		t.Fatal("reviewer ran despite wrong HEAD")
	}
	request.HeadSHA = headSHA
	if _, err := executor.Execute(context.Background(), request); err == nil || !strings.Contains(err.Error(), "tracked files changed") {
		t.Fatalf("mutating review error = %v", err)
	}
}

type reviewExecutorFunc func(context.Context, ReviewExecutionRequest) (ReviewExecutionResult, error)

func (function reviewExecutorFunc) Execute(ctx context.Context, request ReviewExecutionRequest) (ReviewExecutionResult, error) {
	return function(ctx, request)
}
