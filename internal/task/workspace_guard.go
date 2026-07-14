package task

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ExactHeadCheckExecutor struct {
	Inner CheckExecutor
}

func (executor ExactHeadCheckExecutor) Run(
	ctx context.Context,
	workspace string,
	headSHA string,
	checks map[string][]string,
	groups []string,
) ([]CheckRun, error) {
	if executor.Inner == nil {
		return nil, errors.New("exact-HEAD check executor requires an inner executor")
	}
	if err := VerifyExactHeadWorkspace(ctx, workspace, headSHA); err != nil {
		return nil, err
	}
	runs, runErr := executor.Inner.Run(ctx, workspace, headSHA, checks, groups)
	if verifyErr := verifyExactHeadAfter(workspace, headSHA); verifyErr != nil {
		return runs, verifyErr
	}
	return runs, runErr
}

type ExactHeadReviewExecutor struct {
	Inner ReviewExecutor
}

func (executor ExactHeadReviewExecutor) Execute(ctx context.Context, request ReviewExecutionRequest) (ReviewExecutionResult, error) {
	if executor.Inner == nil {
		return ReviewExecutionResult{}, errors.New("exact-HEAD review executor requires an inner executor")
	}
	if err := VerifyExactHeadWorkspace(ctx, request.Workspace, request.HeadSHA); err != nil {
		return ReviewExecutionResult{}, err
	}
	result, runErr := executor.Inner.Execute(ctx, request)
	if verifyErr := verifyExactHeadAfter(request.Workspace, request.HeadSHA); verifyErr != nil {
		return ReviewExecutionResult{}, verifyErr
	}
	return result, runErr
}

func VerifyExactHeadWorkspace(ctx context.Context, workspace, expectedHead string) error {
	if strings.TrimSpace(workspace) == "" || strings.TrimSpace(expectedHead) == "" {
		return errors.New("exact-HEAD workspace requires a path and expected HEAD")
	}
	head, err := runWorkspaceGit(ctx, workspace, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(head) != strings.TrimSpace(expectedHead) {
		return fmt.Errorf("review workspace HEAD %s does not match expected HEAD %s", strings.TrimSpace(head), strings.TrimSpace(expectedHead))
	}
	status, err := runWorkspaceGit(ctx, workspace, "status", "--porcelain=v1", "--untracked-files=no")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) != "" {
		return errors.New("review workspace tracked files changed")
	}
	return nil
}

func verifyExactHeadAfter(workspace, expectedHead string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return VerifyExactHeadWorkspace(ctx, workspace, expectedHead)
}

func runWorkspaceGit(ctx context.Context, workspace string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = workspace
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("verify review workspace with git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
