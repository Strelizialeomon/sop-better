package task

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Strelizialeomon/sop-better/internal/config"
)

type Controller struct {
	Profile    config.Profile
	Leases     LeaseService
	Reconciler Reconciler
	Workspaces WorkspaceManager
}

type StartRequest struct {
	Snapshot       Snapshot
	Attestation    Attestation
	CurrentActorID int64
	RunID          string
	MachineID      string
	StateRevision  int64
	BaseSHA        string
	RepositoryPath string
}

type StartResult struct {
	Capsule   Capsule
	Claim     StoredClaim
	Workspace Workspace
	State     ReconcileResult
}

type ContinueRequest struct {
	RepoNodeID  string
	IssueNumber int
	RunID       string
	MachineID   string
}

type ContinueResult struct {
	Claim StoredClaim
	State ReconcileResult
}

type CompletionEvidence struct {
	AcceptanceVerified bool            `json:"acceptance_verified"`
	ChecksPassed       map[string]bool `json:"checks_passed"`
	ReviewCompleted    bool            `json:"review_completed"`
	BlockingFindings   int             `json:"blocking_findings"`
	PullRequestURL     string          `json:"pull_request_url"`
	FinalVerification  string          `json:"final_verification"`
}

type CloseRequest struct {
	Claim    StoredClaim
	Target   BusinessState
	Reason   string
	Evidence CompletionEvidence
}

func (controller Controller) Start(ctx context.Context, request StartRequest) (StartResult, error) {
	if err := controller.Profile.Validate(); err != nil {
		return StartResult{}, err
	}
	// BuildCapsule verifies the trusted snapshot before any remote claim is created.
	capsule, err := BuildCapsule(controller.Profile, request.Snapshot, request.Attestation, request.CurrentActorID)
	if err != nil {
		return StartResult{}, err
	}
	if controller.Leases.Store == nil {
		return StartResult{}, errors.New("task lease service is required")
	}
	if controller.Reconciler.Issues == nil {
		return StartResult{}, errors.New("task issue state store is required")
	}
	if strings.TrimSpace(controller.Workspaces.Root) == "" {
		return StartResult{}, errors.New("task workspace manager is required")
	}
	claim, err := controller.Leases.Claim(ctx, ClaimRequest{
		RepoNodeID: request.Snapshot.RepoNodeID, IssueNumber: request.Snapshot.IssueNumber,
		RunID: request.RunID, MachineID: request.MachineID, ActorID: request.CurrentActorID,
		StateRevision: request.StateRevision, BaseSHA: request.BaseSHA,
	})
	if err != nil {
		return StartResult{}, err
	}
	state, err := controller.Reconciler.Reconcile(ctx, ReconcileRequest{
		RepoNodeID: request.Snapshot.RepoNodeID, IssueNumber: request.Snapshot.IssueNumber, SnapshotHash: capsule.SnapshotHash,
	})
	if err != nil {
		return StartResult{}, fmt.Errorf("reconcile claimed task: %w", err)
	}
	workspace, err := controller.Workspaces.Prepare(ctx, request.RepositoryPath, request.Snapshot.IssueNumber, controller.Profile.Project.DefaultBranch)
	if err != nil {
		return StartResult{}, err
	}
	return StartResult{Capsule: capsule, Claim: claim, Workspace: workspace, State: state}, nil
}

func (controller Controller) Continue(ctx context.Context, request ContinueRequest) (ContinueResult, error) {
	if controller.Leases.Store == nil {
		return ContinueResult{}, errors.New("task lease service is required")
	}
	current, exists, err := controller.Leases.Store.Read(ctx, request.RepoNodeID, request.IssueNumber)
	if err != nil {
		return ContinueResult{}, fmt.Errorf("read task claim: %w", err)
	}
	if !exists {
		return ContinueResult{}, ErrNoClaim
	}
	if current.Claim.RunID != strings.TrimSpace(request.RunID) || current.Claim.MachineID != strings.TrimSpace(request.MachineID) {
		return ContinueResult{}, ErrLeaseLost
	}
	renewed, err := controller.Leases.Renew(ctx, current)
	if err != nil {
		return ContinueResult{}, err
	}
	state := ReconcileResult{Action: ReconcileNoop}
	if controller.Reconciler.Issues != nil {
		state, err = controller.Reconciler.Reconcile(ctx, ReconcileRequest{RepoNodeID: request.RepoNodeID, IssueNumber: request.IssueNumber})
		if err != nil {
			return ContinueResult{}, err
		}
	}
	return ContinueResult{Claim: renewed, State: state}, nil
}

func (controller Controller) Close(ctx context.Context, request CloseRequest) (ReconcileResult, error) {
	if strings.TrimSpace(request.Reason) == "" {
		return ReconcileResult{}, errors.New("terminal task transition requires a reason and recovery/final summary")
	}
	var evidence *CompletionEvidence
	switch request.Target {
	case StateWaiting:
	case StateDone:
		if err := validateCompletionEvidence(controller.Profile, request.Evidence); err != nil {
			return ReconcileResult{}, err
		}
		evidence = &request.Evidence
	default:
		return ReconcileResult{}, errors.New("task can close only to waiting or done")
	}
	renewed, err := controller.Leases.Renew(ctx, request.Claim)
	if err != nil {
		return ReconcileResult{}, err
	}
	return controller.Reconciler.Transition(ctx, renewed, request.Target, request.Reason, evidence)
}

func validateCompletionEvidence(profile config.Profile, evidence CompletionEvidence) error {
	if !evidence.AcceptanceVerified {
		return errors.New("done requires verified acceptance evidence")
	}
	if profile.Runtime == nil {
		return errors.New("done evidence requires loop runtime")
	}
	for group := range profile.Runtime.Checks {
		if !evidence.ChecksPassed[group] {
			return fmt.Errorf("done requires passing check group %q", group)
		}
	}
	if !evidence.ReviewCompleted || evidence.BlockingFindings != 0 {
		return errors.New("done requires completed independent review with zero blocking findings")
	}
	if strings.TrimSpace(evidence.PullRequestURL) == "" {
		return errors.New("done requires a pull request URL")
	}
	if strings.TrimSpace(evidence.FinalVerification) == "" {
		return errors.New("done requires final verification evidence")
	}
	return nil
}
