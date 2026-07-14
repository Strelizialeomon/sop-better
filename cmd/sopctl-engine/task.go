package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	soptask "github.com/Strelizialeomon/sop-better/internal/task"
)

func runTask(args []string, assetRoot string) error {
	if len(args) < 2 {
		return errors.New("usage: sopctl task <start|continue|review|status|explain> <issue> [--project-root <repo>] [--pull-request <URL>]")
	}
	action := args[0]
	switch action {
	case "start", "continue", "review", "status", "explain":
	default:
		return fmt.Errorf("unknown task command %q", action)
	}
	issueNumber, err := strconv.Atoi(args[1])
	if err != nil || issueNumber <= 0 {
		return errors.New("task issue must be a positive number")
	}
	flags := flag.NewFlagSet("task "+action, flag.ContinueOnError)
	projectRoot := flags.String("project-root", ".", "project root")
	targetState := flags.String("to", "", "terminal state for continue: waiting or done")
	reason := flags.String("reason", "", "terminal reason or recovery/final summary")
	evidencePath := flags.String("evidence", "", "completion evidence JSON for --to done")
	pullRequestURL := flags.String("pull-request", "", "pull request URL for review")
	if err := flags.Parse(args[2:]); err != nil {
		return err
	}
	if action == "review" && strings.TrimSpace(*pullRequestURL) == "" {
		return errors.New("task review requires --pull-request <URL>")
	}
	absoluteRoot, err := filepath.Abs(*projectRoot)
	if err != nil {
		return fmt.Errorf("resolve project root: %w", err)
	}
	profile, _, err := loadContract(absoluteRoot, assetRoot)
	if err != nil {
		return err
	}
	if profile.Runtime == nil || profile.Runtime.Mode != "loop-v1-experimental" {
		return errors.New("sopctl task requires profile.runtime.mode loop-v1-experimental; legacy projects must not use the loop runner")
	}
	tracker := soptask.GitHubTracker{
		RepositoryPath:  absoluteRoot,
		TrustedActorIDs: profile.Runtime.Trust.GitHub.TrustedActorIDs,
	}
	repository, err := tracker.Repository(context.Background())
	if err != nil {
		return err
	}
	actor, err := tracker.CurrentActor(context.Background())
	if err != nil {
		return err
	}
	if action == "start" || action == "continue" || action == "review" {
		if err := soptask.CheckClaimWorkflowIsolation(absoluteRoot); err != nil {
			return err
		}
		if err := tracker.PreflightWrite(context.Background(), repository); err != nil {
			return err
		}
	}
	claimStore := &soptask.GitClaimStore{RepositoryPath: absoluteRoot, Remote: "origin", Clock: tracker.ServerClock()}
	leasing := soptask.LeaseService{
		Store:    claimStore,
		NewToken: func() (string, error) { return randomID("fence-", 16) },
		TTL:      time.Duration(profile.Runtime.LeaseTimeoutSeconds) * time.Second,
	}
	reconciler := soptask.Reconciler{
		Claims: claimStore, Issues: tracker, Leases: leasing,
		NewEventID: func() (string, error) { return randomID("evt-", 12) },
	}
	taskHome, err := localTaskHome()
	if err != nil {
		return err
	}
	machineID, err := loadOrCreateMachineID(taskHome)
	if err != nil {
		return err
	}
	controller := soptask.Controller{
		Profile: profile, Leases: leasing, Reconciler: reconciler,
		Workspaces:       soptask.WorkspaceManager{Root: filepath.Join(taskHome, "workspaces")},
		Reviews:          tracker,
		Checks:           soptask.ExactHeadCheckExecutor{Inner: soptask.ShellCheckExecutor{}},
		Reviewer:         soptask.ExactHeadReviewExecutor{Inner: soptask.CodexReviewExecutor{TempRoot: taskHome}},
		NewReviewEventID: func() (string, error) { return randomID("review-", 12) },
	}
	startReady := func(ready soptask.ReadyEnvelope, state soptask.IssueState) error {
		baseSHA, err := remoteBranchSHA(context.Background(), absoluteRoot, profile.Project.DefaultBranch)
		if err != nil {
			return err
		}
		runID, err := randomID("run-", 12)
		if err != nil {
			return err
		}
		result, err := controller.Start(context.Background(), soptask.StartRequest{
			Snapshot: ready.Snapshot, Attestation: ready.Attestation, CurrentActorID: actor.ID,
			RunID: runID, MachineID: machineID, StateRevision: state.Revision, BaseSHA: baseSHA,
			RepositoryPath: absoluteRoot,
		})
		if err != nil {
			return err
		}
		return writeTaskResult(map[string]any{
			"state": result.State.State, "run_id": result.Claim.Claim.RunID,
			"machine_id": machineID, "lease_expires_at": result.Claim.Claim.LeaseExpiresAt,
			"workspace": result.Workspace.Path, "branch": result.Workspace.Branch, "capsule": result.Capsule,
		})
	}

	switch action {
	case "start":
		ready, err := tracker.ApprovedTask(context.Background(), repository.NodeID, issueNumber)
		if errors.Is(err, soptask.ErrNoReadyAttestation) {
			ready, err = tracker.AttestCurrentIssue(context.Background(), repository.NodeID, repository.FullName, issueNumber, actor.ID, profile.SOPVersion, 0)
		}
		if err != nil {
			return err
		}
		state, err := tracker.ReadState(context.Background(), repository.NodeID, issueNumber)
		if err != nil {
			return err
		}
		if state.State != soptask.StateReady {
			return fmt.Errorf("task #%d is %s at revision %d, not ready", issueNumber, state.State, state.Revision)
		}
		return startReady(ready, state)
	case "continue":
		claim, exists, err := claimStore.Read(context.Background(), repository.NodeID, issueNumber)
		if err != nil {
			return err
		}
		var preparedInput *completionInput
		var preparedMerged *soptask.MergedPullRequest
		var preparedWorkspace string
		var preparedSnapshotHash string
		if !exists {
			state, stateErr := tracker.ReadState(context.Background(), repository.NodeID, issueNumber)
			if stateErr != nil {
				return stateErr
			}
			if state.State == soptask.StateRunning && *targetState == "" {
				baseSHA, baseErr := remoteBranchSHA(context.Background(), absoluteRoot, profile.Project.DefaultBranch)
				if baseErr != nil {
					return baseErr
				}
				recoveryRun, idErr := randomID("reconcile-", 12)
				if idErr != nil {
					return idErr
				}
				reconciled, reconcileErr := reconciler.Reconcile(context.Background(), soptask.ReconcileRequest{
					RepoNodeID: repository.NodeID, IssueNumber: issueNumber,
					RecoveryClaim: &soptask.ClaimRequest{RunID: recoveryRun, MachineID: machineID, ActorID: actor.ID, BaseSHA: baseSHA},
				})
				if reconcileErr != nil {
					return reconcileErr
				}
				state = soptask.IssueState{State: reconciled.State, Revision: reconciled.Revision}
			}
			if state.State == soptask.StateWaiting && *targetState == string(soptask.StateDone) {
				if strings.TrimSpace(*evidencePath) == "" {
					return errors.New("task continue --to done requires --evidence <file.json>")
				}
				var input completionInput
				if err := loadCompletionEvidence(*evidencePath, &input); err != nil {
					return err
				}
				merged, err := tracker.VerifyMergedPullRequest(context.Background(), repository.NodeID, repository.FullName, profile.Project.DefaultBranch, input.PullRequestURL)
				if err != nil {
					return err
				}
				verificationWorkspace, cleanup, err := prepareFinalVerificationWorkspace(context.Background(), absoluteRoot, taskHome, profile.Project.DefaultBranch, merged.CommitSHA)
				if err != nil {
					return err
				}
				defer cleanup()
				ready, err := tracker.ApprovedTask(context.Background(), repository.NodeID, issueNumber)
				if err != nil {
					return err
				}
				terminalRun, err := randomID("terminal-", 12)
				if err != nil {
					return err
				}
				claim, err = controller.ClaimTerminalVerification(context.Background(), soptask.TerminalVerificationClaimRequest{
					RepoNodeID: repository.NodeID, IssueNumber: issueNumber, RunID: terminalRun, MachineID: machineID, ActorID: actor.ID,
				})
				if err != nil {
					return err
				}
				exists = true
				preparedInput = &input
				preparedMerged = &merged
				preparedWorkspace = verificationWorkspace
				preparedSnapshotHash = ready.Attestation.SnapshotHash
			}
			if !exists && (state.State != soptask.StateWaiting || *targetState != "") {
				return soptask.ErrNoClaim
			}
			if !exists {
				ready, attestErr := tracker.AttestCurrentIssue(context.Background(), repository.NodeID, repository.FullName, issueNumber, actor.ID, profile.SOPVersion, state.Revision+1)
				if attestErr != nil {
					return attestErr
				}
				return startReady(ready, soptask.IssueState{State: soptask.StateReady, Revision: state.Revision + 1})
			}
		}
		if claim.Claim.MachineID != machineID || claim.Claim.ActorID != actor.ID {
			return soptask.ErrLeaseLost
		}
		if *targetState != "" {
			target := soptask.BusinessState(*targetState)
			evidence := soptask.CompletionEvidence{}
			workspacePath := ""
			snapshotHash := ""
			if target == soptask.StateWaiting && strings.TrimSpace(*pullRequestURL) != "" {
				chain, err := tracker.ReadReviewChain(context.Background(), repository.NodeID, issueNumber)
				if err != nil || len(chain.Segments) == 0 {
					return errors.New("awaiting merge requires canonical review coverage")
				}
				head := reviewChainHead(chain)
				basis, err := tracker.VerifyReviewPullRequest(context.Background(), repository.NodeID, repository.FullName, profile.Project.DefaultBranch, *pullRequestURL, head)
				if err != nil || chain.Segments[0].ReviewBasis != basis {
					return errors.New("pull request review basis changed; run task review again")
				}
				ready, err := tracker.ApprovedTask(context.Background(), repository.NodeID, issueNumber)
				if err != nil {
					return err
				}
				scopeHash, err := soptask.ProfileScopeHash(profile)
				if err != nil {
					return err
				}
				if err := soptask.ValidateReviewCoverage(chain, basis.MergeBaseSHA, head, ready.Attestation.SnapshotHash, scopeHash); err != nil {
					return err
				}
			}
			if target == soptask.StateDone {
				if strings.TrimSpace(*evidencePath) == "" {
					return errors.New("task continue --to done requires --evidence <file.json>")
				}
				var input completionInput
				if preparedInput != nil {
					input = *preparedInput
				} else if err := loadCompletionEvidence(*evidencePath, &input); err != nil {
					return err
				}
				evidence.AcceptanceVerified = input.AcceptanceVerified
				evidence.PullRequestURL = input.PullRequestURL
				var merged soptask.MergedPullRequest
				if preparedMerged != nil {
					merged = *preparedMerged
				} else {
					merged, err = tracker.VerifyMergedPullRequest(context.Background(), repository.NodeID, repository.FullName, profile.Project.DefaultBranch, evidence.PullRequestURL)
					if err != nil {
						return err
					}
				}
				evidence.PullRequestHeadSHA = merged.HeadSHA
				evidence.ReviewBasis = merged.ReviewBasis
				evidence.MergedCommitSHA = merged.CommitSHA
				if preparedWorkspace != "" {
					workspacePath = preparedWorkspace
					snapshotHash = preparedSnapshotHash
				} else {
					workspace, err := controller.Workspaces.Prepare(context.Background(), absoluteRoot, issueNumber, claim.Claim.BaseSHA)
					if err != nil {
						return err
					}
					localSHA, err := gitHeadSHA(context.Background(), workspace.Path)
					if err != nil {
						return err
					}
					if localSHA != merged.HeadSHA {
						return fmt.Errorf("task workspace HEAD %s does not match pull request head %s", localSHA, merged.HeadSHA)
					}
					verificationWorkspace, cleanup, err := prepareFinalVerificationWorkspace(context.Background(), absoluteRoot, taskHome, profile.Project.DefaultBranch, merged.CommitSHA)
					if err != nil {
						return err
					}
					defer cleanup()
					ready, err := tracker.ApprovedTask(context.Background(), repository.NodeID, issueNumber)
					if err != nil {
						return err
					}
					workspacePath = verificationWorkspace
					snapshotHash = ready.Attestation.SnapshotHash
				}
			}
			closed, err := controller.Close(context.Background(), soptask.CloseRequest{
				Claim: claim, RunID: claim.Claim.RunID, MachineID: machineID,
				Target: target, Reason: *reason, SnapshotHash: snapshotHash, Workspace: workspacePath, Evidence: evidence,
			})
			if err != nil {
				return err
			}
			return writeTaskResult(map[string]any{"state": closed.State, "state_revision": closed.Revision, "claim_released": true})
		}
		ready, err := tracker.ApprovedTask(context.Background(), repository.NodeID, issueNumber)
		if err != nil {
			return err
		}
		result, err := controller.Continue(context.Background(), soptask.ContinueRequest{
			RepoNodeID: repository.NodeID, IssueNumber: issueNumber, RunID: claim.Claim.RunID, MachineID: machineID,
			Snapshot: ready.Snapshot, Attestation: ready.Attestation, CurrentActorID: actor.ID, RepositoryPath: absoluteRoot,
		})
		if err != nil {
			return err
		}
		return writeTaskResult(map[string]any{
			"state": result.State.State, "run_id": result.Claim.Claim.RunID,
			"machine_id": machineID, "lease_expires_at": result.Claim.Claim.LeaseExpiresAt,
			"workspace": result.Workspace.Path, "branch": result.Workspace.Branch, "capsule": result.Capsule,
			"next_action": "continue the capsule action; lease guard passed",
		})
	case "review":
		claim, exists, err := claimStore.Read(context.Background(), repository.NodeID, issueNumber)
		if err != nil {
			return err
		}
		if !exists {
			return soptask.ErrNoClaim
		}
		if claim.Claim.MachineID != machineID || claim.Claim.ActorID != actor.ID {
			return soptask.ErrLeaseLost
		}
		ready, err := tracker.ApprovedTask(context.Background(), repository.NodeID, issueNumber)
		if err != nil {
			return err
		}
		continued, err := controller.Continue(context.Background(), soptask.ContinueRequest{
			RepoNodeID: repository.NodeID, IssueNumber: issueNumber, RunID: claim.Claim.RunID, MachineID: machineID,
			Snapshot: ready.Snapshot, Attestation: ready.Attestation, CurrentActorID: actor.ID, RepositoryPath: absoluteRoot,
		})
		if err != nil {
			return err
		}
		headSHA, err := gitHeadSHA(context.Background(), continued.Workspace.Path)
		if err != nil {
			return err
		}
		reviewBasis, err := tracker.VerifyReviewPullRequest(context.Background(), repository.NodeID, repository.FullName, profile.Project.DefaultBranch, *pullRequestURL, headSHA)
		if err != nil {
			return err
		}
		reviewWorkspaces, cleanup, err := prepareReviewWorkspaces(context.Background(), continued.Workspace.Path, taskHome, headSHA)
		if err != nil {
			return err
		}
		defer cleanup()
		reviewCapsule := continued.Capsule
		if strings.TrimSpace(ready.Snapshot.DocumentURL) != "" {
			reference, err := tracker.MaterializeDecisionDocument(context.Background(), ready.Snapshot, repository.FullName, reviewWorkspaces.ReviewerPath)
			if err != nil {
				reason := "decision document unavailable or changed: " + err.Error() + "; resume after the pinned document and digest are readable"
				waiting, closeErr := controller.Close(context.Background(), soptask.CloseRequest{
					Claim: continued.Claim, RunID: continued.Claim.Claim.RunID, MachineID: machineID,
					Target: soptask.StateWaiting, Reason: reason,
				})
				if closeErr != nil {
					return fmt.Errorf("materialize decision document: %v; also failed to move task to waiting: %w", err, closeErr)
				}
				return writeTaskResult(map[string]any{"state": waiting.State, "state_revision": waiting.Revision, "claim_released": true, "reason": reason})
			}
			reviewCapsule.RequiredContext = []soptask.ContextReference{reference}
		}
		changedPaths, err := gitChangedPaths(context.Background(), reviewWorkspaces.ReviewerPath, reviewBasis.MergeBaseSHA, headSHA)
		if err != nil {
			return err
		}
		result, err := controller.Review(context.Background(), soptask.ReviewRequest{
			Claim: continued.Claim, RunID: continued.Claim.Claim.RunID, MachineID: machineID,
			Snapshot: ready.Snapshot, Capsule: reviewCapsule,
			ChecksWorkspace: reviewWorkspaces.ChecksPath, ReviewerWorkspace: reviewWorkspaces.ReviewerPath,
			HeadSHA: headSHA, TaskChangedPaths: changedPaths, ReviewBasis: reviewBasis,
		})
		if err != nil {
			return err
		}
		return writeTaskResult(map[string]any{
			"mode": result.Plan.Mode, "base_sha": result.Plan.BaseSHA, "head_sha": headSHA,
			"reason": result.Plan.Reason, "review_reference": result.Review.Reference,
			"check_selection": result.Selection, "check_runs": result.CheckRuns,
			"findings": result.Chain.Findings, "coverage_segments": result.Chain.Segments,
		})
	case "status", "explain":
		state, err := tracker.ReadState(context.Background(), repository.NodeID, issueNumber)
		if err != nil {
			return err
		}
		claim, exists, err := claimStore.Read(context.Background(), repository.NodeID, issueNumber)
		if err != nil {
			return err
		}
		status := map[string]any{"issue": issueNumber, "state": state.State, "state_revision": state.Revision, "claimed": exists}
		if exists {
			status["run_id"] = claim.Claim.RunID
			status["machine_id"] = claim.Claim.MachineID
			status["lease_epoch"] = claim.Claim.LeaseEpoch
			status["lease_expires_at"] = claim.Claim.LeaseExpiresAt
		}
		reviews, reviewErr := tracker.ReadReviewChain(context.Background(), repository.NodeID, issueNumber)
		if reviewErr != nil {
			return reviewErr
		}
		status["review_segments"] = len(reviews.Segments)
		status["review_head_sha"] = reviewChainHead(reviews)
		status["open_review_findings"] = openFindingIDs(reviews)
		if action == "explain" {
			status["meaning"] = explainTaskState(state.State, exists)
		}
		return writeTaskResult(status)
	}
	return nil
}

type completionInput struct {
	AcceptanceVerified bool   `json:"acceptance_verified"`
	PullRequestURL     string `json:"pull_request_url"`
}

func loadCompletionEvidence(path string, output *completionInput) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open completion evidence: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("parse completion evidence: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("completion evidence contains trailing JSON values")
	}
	return nil
}

func randomID(prefix string, size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate %s ID: %w", strings.TrimSuffix(prefix, "-"), err)
	}
	return prefix + hex.EncodeToString(data), nil
}

func localTaskHome() (string, error) {
	if override := strings.TrimSpace(os.Getenv("SOP_TASK_HOME")); override != "" {
		return filepath.Abs(override)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache directory: %w", err)
	}
	return filepath.Join(cache, "sopctl", "loop-v1"), nil
}

func loadOrCreateMachineID(home string) (string, error) {
	path := filepath.Join(home, "machine-id")
	if data, err := os.ReadFile(path); err == nil {
		if value := strings.TrimSpace(string(data)); value != "" {
			return value, nil
		}
		return "", errors.New("stored task machine ID is empty")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read task machine ID: %w", err)
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return "", fmt.Errorf("create task state directory: %w", err)
	}
	id, err := randomID("machine-", 12)
	if err != nil {
		return "", err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return loadOrCreateMachineID(home)
	}
	if err != nil {
		return "", fmt.Errorf("create task machine ID: %w", err)
	}
	if _, err := file.WriteString(id + "\n"); err != nil {
		file.Close()
		return "", fmt.Errorf("write task machine ID: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close task machine ID: %w", err)
	}
	return id, nil
}

func remoteBranchSHA(ctx context.Context, repositoryPath, branch string) (string, error) {
	command := exec.CommandContext(ctx, "git", "ls-remote", "--refs", "origin", "refs/heads/"+branch)
	command.Dir = repositoryPath
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read remote base branch: %w: %s", err, strings.TrimSpace(string(output)))
	}
	fields := strings.Fields(string(output))
	if len(fields) != 2 {
		return "", fmt.Errorf("remote default branch %q was not found", branch)
	}
	return fields[0], nil
}

func gitHeadSHA(ctx context.Context, repositoryPath string) (string, error) {
	command := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	command.Dir = repositoryPath
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read task workspace HEAD: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func prepareFinalVerificationWorkspace(ctx context.Context, repositoryPath, taskHome, defaultBranch, commitSHA string) (string, func(), error) {
	root, err := os.MkdirTemp(taskHome, "final-verify-")
	if err != nil {
		return "", nil, fmt.Errorf("create final verification root: %w", err)
	}
	cleanupRoot := func() { _ = os.RemoveAll(root) }
	if _, err := runGitRead(ctx, repositoryPath, "fetch", "--quiet", "origin", "refs/heads/"+defaultBranch); err != nil {
		cleanupRoot()
		return "", nil, fmt.Errorf("fetch final default branch: %w", err)
	}
	if _, err := runGitRead(ctx, repositoryPath, "merge-base", "--is-ancestor", commitSHA, "FETCH_HEAD"); err != nil {
		cleanupRoot()
		return "", nil, errors.New("merged commit is not reachable from the fetched default branch")
	}
	workspace := filepath.Join(root, "workspace")
	if _, err := runGitRead(ctx, repositoryPath, "worktree", "add", "--detach", workspace, commitSHA); err != nil {
		cleanupRoot()
		return "", nil, fmt.Errorf("create final verification worktree: %w", err)
	}
	cleanup := func() {
		_, _ = runGitRead(context.Background(), repositoryPath, "worktree", "remove", "--force", workspace)
		cleanupRoot()
	}
	return workspace, cleanup, nil
}

type reviewWorkspaces struct {
	ChecksPath   string
	ReviewerPath string
}

func prepareReviewWorkspaces(ctx context.Context, sourceWorkspace, taskHome, headSHA string) (reviewWorkspaces, func(), error) {
	status, err := runGitRead(ctx, sourceWorkspace, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return reviewWorkspaces{}, nil, fmt.Errorf("inspect review source workspace: %w", err)
	}
	if strings.TrimSpace(status) != "" {
		return reviewWorkspaces{}, nil, errors.New("review requires a clean committed source workspace")
	}
	if err := soptask.VerifyExactHeadWorkspace(ctx, sourceWorkspace, headSHA); err != nil {
		return reviewWorkspaces{}, nil, err
	}
	if err := os.MkdirAll(taskHome, 0o700); err != nil {
		return reviewWorkspaces{}, nil, fmt.Errorf("create task state directory: %w", err)
	}
	root, err := os.MkdirTemp(taskHome, "review-workspaces-")
	if err != nil {
		return reviewWorkspaces{}, nil, fmt.Errorf("create review workspace root: %w", err)
	}
	workspaces := reviewWorkspaces{ChecksPath: filepath.Join(root, "checks"), ReviewerPath: filepath.Join(root, "reviewer")}
	created := make([]string, 0, 2)
	cleanup := func() {
		for index := len(created) - 1; index >= 0; index-- {
			_, _ = runGitRead(context.Background(), sourceWorkspace, "worktree", "remove", "--force", created[index])
		}
		_ = os.RemoveAll(root)
	}
	for _, workspace := range []string{workspaces.ChecksPath, workspaces.ReviewerPath} {
		if _, err := runGitRead(ctx, sourceWorkspace, "worktree", "add", "--detach", workspace, headSHA); err != nil {
			cleanup()
			return reviewWorkspaces{}, nil, fmt.Errorf("create exact-HEAD review workspace: %w", err)
		}
		created = append(created, workspace)
		if err := soptask.VerifyExactHeadWorkspace(ctx, workspace, headSHA); err != nil {
			cleanup()
			return reviewWorkspaces{}, nil, err
		}
	}
	return workspaces, cleanup, nil
}

func gitChangedPaths(ctx context.Context, workspace, baseSHA, headSHA string) ([]string, error) {
	output, err := runGitRead(ctx, workspace, "diff", "--no-renames", "--name-only", baseSHA+".."+headSHA, "--")
	if err != nil {
		return nil, fmt.Errorf("read review changed paths: %w", err)
	}
	var paths []string
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		if value := strings.TrimSpace(line); value != "" {
			paths = append(paths, value)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func runGitRead(ctx context.Context, workspace string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = workspace
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func reviewChainHead(chain soptask.ReviewChain) string {
	if len(chain.Segments) == 0 {
		return ""
	}
	return chain.Segments[len(chain.Segments)-1].HeadSHA
}

func openFindingIDs(chain soptask.ReviewChain) []string {
	var ids []string
	for id, finding := range chain.Findings {
		if finding.Status == soptask.FindingOpen {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func explainTaskState(state soptask.BusinessState, claimed bool) string {
	switch state {
	case soptask.StateReady:
		return "可信任务已准备好；没有 claim 时可以 start"
	case soptask.StateRunning:
		if claimed {
			return "一台机器持有有效任务 claim；远端写入前必须 continue 续租"
		}
		return "Issue 显示 running 但没有 claim；需要 reconcile 转 waiting"
	case soptask.StateWaiting:
		return "任务在等待 owner 或外部条件；满足恢复条件后再 continue/start"
	case soptask.StateDone:
		return "最终证据和对账已完成；不应再有 claim"
	default:
		return "未知状态；停止写入并检查 canonical state events"
	}
}

func writeTaskResult(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
