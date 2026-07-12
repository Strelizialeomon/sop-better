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
	"strconv"
	"strings"
	"time"

	soptask "github.com/Strelizialeomon/sop-better/internal/task"
)

func runTask(args []string, assetRoot string) error {
	if len(args) < 2 {
		return errors.New("usage: sopctl task <start|continue|status|explain> <issue> [--project-root <repo>]")
	}
	action := args[0]
	switch action {
	case "start", "continue", "status", "explain":
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
	if err := flags.Parse(args[2:]); err != nil {
		return err
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
		Workspaces: soptask.WorkspaceManager{Root: filepath.Join(taskHome, "workspaces")},
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
			ready, err = tracker.AttestCurrentIssue(context.Background(), repository.NodeID, issueNumber, actor.ID, profile.SOPVersion, 0)
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
		if !exists {
			state, stateErr := tracker.ReadState(context.Background(), repository.NodeID, issueNumber)
			if stateErr != nil {
				return stateErr
			}
			if state.State != soptask.StateWaiting || *targetState != "" {
				return soptask.ErrNoClaim
			}
			ready, attestErr := tracker.AttestCurrentIssue(context.Background(), repository.NodeID, issueNumber, actor.ID, profile.SOPVersion, state.Revision+1)
			if attestErr != nil {
				return attestErr
			}
			return startReady(ready, soptask.IssueState{State: soptask.StateReady, Revision: state.Revision + 1})
		}
		if *targetState != "" {
			target := soptask.BusinessState(*targetState)
			evidence := soptask.CompletionEvidence{}
			if target == soptask.StateDone {
				if strings.TrimSpace(*evidencePath) == "" {
					return errors.New("task continue --to done requires --evidence <file.json>")
				}
				if err := loadCompletionEvidence(*evidencePath, &evidence); err != nil {
					return err
				}
			}
			closed, err := controller.Close(context.Background(), soptask.CloseRequest{
				Claim: claim, Target: target, Reason: *reason, Evidence: evidence,
			})
			if err != nil {
				return err
			}
			return writeTaskResult(map[string]any{"state": closed.State, "state_revision": closed.Revision, "claim_released": true})
		}
		result, err := controller.Continue(context.Background(), soptask.ContinueRequest{
			RepoNodeID: repository.NodeID, IssueNumber: issueNumber, RunID: claim.Claim.RunID, MachineID: machineID,
		})
		if err != nil {
			return err
		}
		return writeTaskResult(map[string]any{
			"state": result.State.State, "run_id": result.Claim.Claim.RunID,
			"machine_id": machineID, "lease_expires_at": result.Claim.Claim.LeaseExpiresAt,
			"next_action": "continue the capsule action; lease guard passed",
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
		if action == "explain" {
			status["meaning"] = explainTaskState(state.State, exists)
		}
		return writeTaskResult(status)
	}
	return nil
}

func loadCompletionEvidence(path string, output *soptask.CompletionEvidence) error {
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
