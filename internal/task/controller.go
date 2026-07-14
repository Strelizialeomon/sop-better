package task

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Strelizialeomon/sop-better/internal/config"
)

type Controller struct {
	Profile           config.Profile
	Leases            LeaseService
	Reconciler        Reconciler
	Workspaces        WorkspaceManager
	Reviews           ReviewStore
	Checks            CheckExecutor
	Reviewer          ReviewExecutor
	NewReviewEventID  func() (string, error)
	HeartbeatInterval time.Duration
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
	RepoNodeID     string
	IssueNumber    int
	RunID          string
	MachineID      string
	Snapshot       Snapshot
	Attestation    Attestation
	CurrentActorID int64
	RepositoryPath string
}

type ContinueResult struct {
	Claim     StoredClaim
	State     ReconcileResult
	Capsule   Capsule
	Workspace Workspace
}

type TerminalVerificationClaimRequest struct {
	RepoNodeID  string
	IssueNumber int
	RunID       string
	MachineID   string
	ActorID     int64
}

type CompletionEvidence struct {
	AcceptanceVerified bool            `json:"acceptance_verified"`
	ChecksPassed       map[string]bool `json:"checks_passed"`
	ReviewCompleted    bool            `json:"review_completed"`
	BlockingFindings   int             `json:"blocking_findings"`
	ReviewReference    string          `json:"review_reference"`
	ReviewedHeadSHA    string          `json:"reviewed_head_sha"`
	ReviewBasis        ReviewBasis     `json:"-"`
	PullRequestURL     string          `json:"pull_request_url"`
	PullRequestHeadSHA string          `json:"pull_request_head_sha"`
	MergedCommitSHA    string          `json:"merged_commit_sha"`
	FinalVerification  string          `json:"final_verification"`
	FinalCheckRuns     []CheckRun      `json:"final_check_runs"`
}

type CloseRequest struct {
	Claim        StoredClaim
	RunID        string
	MachineID    string
	Target       BusinessState
	Reason       string
	SnapshotHash string
	Workspace    string
	Evidence     CompletionEvidence
}

type ReviewRequest struct {
	Claim             StoredClaim
	RunID             string
	MachineID         string
	Snapshot          Snapshot
	Capsule           Capsule
	ChecksWorkspace   string
	ReviewerWorkspace string
	HeadSHA           string
	TaskChangedPaths  []string
	ReviewBasis       ReviewBasis
}

type ReviewResult struct {
	Plan      ReviewPlan
	Selection CheckSelection
	CheckRuns []CheckRun
	Review    ReviewExecutionResult
	Chain     ReviewChain
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
	claim, exists, err := controller.Leases.Store.Read(ctx, request.Snapshot.RepoNodeID, request.Snapshot.IssueNumber)
	if err != nil {
		return StartResult{}, fmt.Errorf("read guarded task claim: %w", err)
	}
	if !exists || claim.Claim.RunID != request.RunID || claim.Claim.MachineID != request.MachineID {
		return StartResult{}, ErrLeaseLost
	}
	workspace, err := controller.Workspaces.Prepare(ctx, request.RepositoryPath, request.Snapshot.IssueNumber, request.BaseSHA)
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
	capsule, err := BuildCapsule(controller.Profile, request.Snapshot, request.Attestation, request.CurrentActorID)
	if err != nil {
		return ContinueResult{}, err
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
	renewed, exists, err = controller.Leases.Store.Read(ctx, request.RepoNodeID, request.IssueNumber)
	if err != nil {
		return ContinueResult{}, fmt.Errorf("read guarded task claim after reconcile: %w", err)
	}
	if !exists || renewed.Claim.RunID != request.RunID || renewed.Claim.MachineID != request.MachineID {
		return ContinueResult{}, ErrLeaseLost
	}
	workspace, err := controller.Workspaces.Prepare(ctx, request.RepositoryPath, request.IssueNumber, current.Claim.BaseSHA)
	if err != nil {
		return ContinueResult{}, err
	}
	return ContinueResult{Claim: renewed, State: state, Capsule: capsule, Workspace: workspace}, nil
}

func (controller Controller) ClaimTerminalVerification(ctx context.Context, request TerminalVerificationClaimRequest) (StoredClaim, error) {
	if err := controller.Profile.Validate(); err != nil {
		return StoredClaim{}, err
	}
	if controller.Reconciler.Issues == nil || controller.Leases.Store == nil || controller.Reviews == nil {
		return StoredClaim{}, errors.New("terminal verification requires issue state, lease, and review stores")
	}
	if !slices.Contains(controller.Profile.Runtime.Trust.GitHub.TrustedActorIDs, request.ActorID) {
		return StoredClaim{}, fmt.Errorf("current GitHub actor %d is not trusted by the project profile", request.ActorID)
	}
	state, err := controller.Reconciler.Issues.ReadState(ctx, request.RepoNodeID, request.IssueNumber)
	if err != nil {
		return StoredClaim{}, fmt.Errorf("read waiting task state: %w", err)
	}
	if state.State != StateWaiting {
		return StoredClaim{}, fmt.Errorf("terminal verification claim requires waiting state, got %s", state.State)
	}
	chain, err := controller.Reviews.ReadReviewChain(ctx, request.RepoNodeID, request.IssueNumber)
	if err != nil {
		return StoredClaim{}, fmt.Errorf("read terminal review coverage: %w", err)
	}
	if len(chain.Segments) == 0 || chain.Segments[0].Mode != ReviewFull || strings.TrimSpace(chain.Segments[0].BaseSHA) == "" {
		return StoredClaim{}, errors.New("terminal verification claim requires a canonical full review base")
	}
	return controller.Leases.Claim(ctx, ClaimRequest{
		RepoNodeID: request.RepoNodeID, IssueNumber: request.IssueNumber,
		RunID: request.RunID, MachineID: request.MachineID, ActorID: request.ActorID,
		StateRevision: state.Revision, BaseSHA: chain.Segments[0].BaseSHA,
	})
}

func (controller Controller) Close(ctx context.Context, request CloseRequest) (result ReconcileResult, err error) {
	if !claimOwnedBy(request.Claim, request.RunID, request.MachineID) {
		return ReconcileResult{}, ErrLeaseLost
	}
	if strings.TrimSpace(request.Reason) == "" {
		return ReconcileResult{}, errors.New("terminal task transition requires a reason and recovery/final summary")
	}
	terminalVerification := false
	if request.Target == StateDone {
		if controller.Reconciler.Issues == nil {
			return ReconcileResult{}, errors.New("done requires canonical waiting state")
		}
		state, stateErr := controller.Reconciler.Issues.ReadState(ctx, request.Claim.Claim.RepoNodeID, request.Claim.Claim.IssueNumber)
		if stateErr != nil {
			return ReconcileResult{}, fmt.Errorf("read state before final verification: %w", stateErr)
		}
		if state.State != StateWaiting || request.Claim.Claim.StateRevision != state.Revision {
			return ReconcileResult{}, errors.New("done requires a terminal-verification claim from the current waiting revision")
		}
		terminalVerification = true
	}
	if terminalVerification {
		defer func() {
			if err == nil && result.State == StateDone {
				return
			}
			current, exists, readErr := controller.Leases.Store.Read(context.Background(), request.Claim.Claim.RepoNodeID, request.Claim.Claim.IssueNumber)
			if readErr != nil {
				err = errors.Join(err, fmt.Errorf("read failed terminal verification claim for cleanup: %w", readErr))
				return
			}
			if !exists || !claimOwnedBy(current, request.RunID, request.MachineID) {
				return
			}
			if releaseErr := controller.Leases.Release(context.Background(), current); releaseErr != nil {
				err = errors.Join(err, fmt.Errorf("release failed terminal verification claim: %w", releaseErr))
			}
		}()
	}
	var evidence *CompletionEvidence
	switch request.Target {
	case StateWaiting:
	case StateDone:
		if err := validateCompletionInput(controller.Profile, request.Evidence); err != nil {
			return ReconcileResult{}, err
		}
		if controller.Checks == nil || strings.TrimSpace(request.Workspace) == "" {
			return ReconcileResult{}, errors.New("done requires controller-run final checks in the task workspace")
		}
		guarded, err := controller.Leases.Renew(ctx, request.Claim)
		if err != nil {
			return ReconcileResult{}, err
		}
		checkContext, cancelChecks := context.WithCancel(ctx)
		heartbeat := startLeaseHeartbeat(checkContext, cancelChecks, controller.Leases, guarded, controller.heartbeatInterval())
		groups := fullCheckSelection(controller.Profile.Runtime.Checks).Groups
		checkRuns, err := controller.Checks.Run(checkContext, request.Workspace, request.Evidence.MergedCommitSHA, controller.Profile.Runtime.Checks, groups)
		guarded, heartbeatErr := heartbeat.Stop()
		cancelChecks()
		if err != nil {
			return ReconcileResult{}, err
		}
		if heartbeatErr != nil {
			return ReconcileResult{}, heartbeatErr
		}
		request.Claim = guarded
		request.Evidence.ChecksPassed = CheckRunsPassed(checkRuns)
		request.Evidence.FinalCheckRuns = checkRuns
		request.Evidence.FinalVerification = fmt.Sprintf("all %d profile check groups passed at merged commit %s", len(groups), request.Evidence.MergedCommitSHA)
		for _, group := range groups {
			if !request.Evidence.ChecksPassed[group] {
				return ReconcileResult{}, fmt.Errorf("done requires passing check group %q", group)
			}
		}
		if controller.Reviews == nil {
			return ReconcileResult{}, errors.New("done requires the canonical review store")
		}
		if strings.TrimSpace(request.SnapshotHash) == "" {
			return ReconcileResult{}, errors.New("done requires the approved snapshot hash")
		}
		scopeHash, err := ProfileScopeHash(controller.Profile)
		if err != nil {
			return ReconcileResult{}, err
		}
		chain, err := controller.Reviews.ReadReviewChain(ctx, request.Claim.Claim.RepoNodeID, request.Claim.Claim.IssueNumber)
		if err != nil {
			return ReconcileResult{}, fmt.Errorf("read canonical review coverage: %w", err)
		}
		reviewBase := request.Claim.Claim.BaseSHA
		if len(chain.Segments) > 0 {
			reviewBase = chain.Segments[0].BaseSHA
			if chain.Segments[0].ReviewBasis != request.Evidence.ReviewBasis {
				return ReconcileResult{}, errors.New("done pull request basis does not match the reviewed pull request")
			}
		}
		if err := ValidateReviewCoverage(chain, reviewBase, request.Evidence.PullRequestHeadSHA, request.SnapshotHash, scopeHash); err != nil {
			return ReconcileResult{}, err
		}
		last := chain.Segments[len(chain.Segments)-1]
		request.Evidence.ReviewCompleted = true
		request.Evidence.BlockingFindings = 0
		request.Evidence.ReviewReference = last.ReviewReference
		request.Evidence.ReviewedHeadSHA = last.HeadSHA
		evidence = &request.Evidence
	default:
		return ReconcileResult{}, errors.New("task can close only to waiting or done")
	}
	return controller.Reconciler.Transition(ctx, request.Claim, request.Target, request.Reason, evidence)
}

func validateCompletionInput(profile config.Profile, evidence CompletionEvidence) error {
	if !evidence.AcceptanceVerified {
		return errors.New("done requires verified acceptance evidence")
	}
	if profile.Runtime == nil {
		return errors.New("done evidence requires loop runtime")
	}
	if strings.TrimSpace(evidence.PullRequestURL) == "" {
		return errors.New("done requires a pull request URL")
	}
	if strings.TrimSpace(evidence.PullRequestHeadSHA) == "" || strings.TrimSpace(evidence.MergedCommitSHA) == "" {
		return errors.New("done requires GitHub-verified pull request head and merged commit SHAs")
	}
	return nil
}

func (controller Controller) Review(ctx context.Context, request ReviewRequest) (ReviewResult, error) {
	if err := controller.Profile.Validate(); err != nil {
		return ReviewResult{}, err
	}
	if controller.Reviews == nil || controller.Checks == nil || controller.Reviewer == nil || controller.NewReviewEventID == nil {
		return ReviewResult{}, errors.New("review requires canonical store, check executor, reviewer, and event ID generator")
	}
	if !claimOwnedBy(request.Claim, request.RunID, request.MachineID) {
		return ReviewResult{}, ErrLeaseLost
	}
	if strings.TrimSpace(request.ChecksWorkspace) == "" || strings.TrimSpace(request.ReviewerWorkspace) == "" || request.ChecksWorkspace == request.ReviewerWorkspace {
		return ReviewResult{}, errors.New("review requires separate checks and reviewer workspaces")
	}
	if err := ValidateCapsuleChangedPaths(request.Capsule, request.TaskChangedPaths); err != nil {
		return ReviewResult{}, err
	}
	guarded, err := controller.Leases.Renew(ctx, request.Claim)
	if err != nil {
		return ReviewResult{}, err
	}
	chain, err := controller.Reviews.ReadReviewChain(ctx, guarded.Claim.RepoNodeID, guarded.Claim.IssueNumber)
	if err != nil {
		return ReviewResult{}, fmt.Errorf("read canonical review chain: %w", err)
	}
	scopeHash, err := CapsuleScopeHash(request.Capsule)
	if err != nil {
		return ReviewResult{}, err
	}
	plan, err := PlanReview(ReviewPlanInput{
		Chain: chain, TaskBaseSHA: guarded.Claim.BaseSHA, HeadSHA: request.HeadSHA,
		SnapshotHash: request.Capsule.SnapshotHash, ScopeHash: scopeHash, ReviewBasis: request.ReviewBasis,
	})
	if err != nil {
		return ReviewResult{}, err
	}
	workContext, cancelWork := context.WithCancel(ctx)
	heartbeat := startLeaseHeartbeat(workContext, cancelWork, controller.Leases, guarded, controller.heartbeatInterval())
	defer cancelWork()
	selection := fullCheckSelection(controller.Profile.Runtime.Checks)
	checkRuns, err := controller.Checks.Run(workContext, request.ChecksWorkspace, request.HeadSHA, controller.Profile.Runtime.Checks, selection.Groups)
	if err != nil {
		heartbeat.Stop()
		return ReviewResult{}, err
	}
	checksPassed := CheckRunsPassed(checkRuns)
	for _, group := range selection.Groups {
		if !checksPassed[group] {
			heartbeat.Stop()
			return ReviewResult{}, fmt.Errorf("review requires passing affected check group %q", group)
		}
	}
	executeReview := func(currentPlan ReviewPlan) (ReviewExecutionResult, error) {
		reviewerCapsule := BuildReviewerCapsule(request.Snapshot, request.Capsule, currentPlan, request.HeadSHA, selection, checkRuns)
		prompt, promptErr := reviewerCapsule.Prompt()
		if promptErr != nil {
			return ReviewExecutionResult{}, promptErr
		}
		result, executeErr := controller.Reviewer.Execute(workContext, ReviewExecutionRequest{
			Workspace: request.ReviewerWorkspace, BaseSHA: currentPlan.BaseSHA, HeadSHA: request.HeadSHA, Mode: currentPlan.Mode, Prompt: prompt,
		})
		result.InputBytes = len(prompt)
		return result, executeErr
	}
	reviewResult, err := executeReview(plan)
	if err == nil && reviewResult.RequiresFullReview && plan.Mode == ReviewDelta {
		plan = ReviewPlan{Mode: ReviewFull, BaseSHA: chain.Segments[0].BaseSHA, Reason: "delta evidence unavailable; reset to the complete pull request change", OpenFindings: plan.OpenFindings}
		reviewResult, err = executeReview(plan)
	}
	if err == nil && reviewResult.RequiresFullReview {
		err = fmt.Errorf("full reviewer could not reach a decision: %s", reviewResult.FullReviewReason)
	}
	if err != nil {
		heartbeat.Stop()
		return ReviewResult{}, err
	}
	if err := workContext.Err(); err != nil && !errors.Is(err, context.Canceled) {
		heartbeat.Stop()
		return ReviewResult{}, err
	}
	guarded, err = heartbeat.Renew(workContext)
	if err != nil {
		heartbeat.Stop()
		return ReviewResult{}, err
	}
	eventID, err := controller.NewReviewEventID()
	if err != nil {
		heartbeat.Stop()
		return ReviewResult{}, fmt.Errorf("generate review event ID: %w", err)
	}
	event, err := BuildReviewEvent(chain, ReviewEventInput{
		EventID: eventID, RunID: guarded.Claim.RunID, LeaseEpoch: guarded.Claim.LeaseEpoch,
		FencingToken: guarded.Claim.FencingToken, SourceServerTime: guarded.Claim.ServerObserved,
		BaseSHA: plan.BaseSHA, HeadSHA: request.HeadSHA, Mode: plan.Mode,
		SnapshotHash: request.Capsule.SnapshotHash, ScopeHash: scopeHash,
		ReviewBasis: request.ReviewBasis, ReviewReference: reviewResult.Reference,
		CheckRuns: checkRuns, InputBytes: reviewResult.InputBytes, DurationMillis: reviewResult.DurationMillis, Findings: reviewResult.Findings,
	})
	if err != nil {
		heartbeat.Stop()
		return ReviewResult{}, err
	}
	chain, err = controller.Reviews.AppendReviewEvent(workContext, guarded.Claim.RepoNodeID, guarded.Claim.IssueNumber, event)
	_, heartbeatErr := heartbeat.Stop()
	if err != nil {
		return ReviewResult{}, fmt.Errorf("append canonical review event: %w", err)
	}
	if heartbeatErr != nil {
		return ReviewResult{}, heartbeatErr
	}
	return ReviewResult{Plan: plan, Selection: selection, CheckRuns: checkRuns, Review: reviewResult, Chain: chain}, nil
}

func claimOwnedBy(claim StoredClaim, runID, machineID string) bool {
	return strings.TrimSpace(runID) != "" && strings.TrimSpace(machineID) != "" &&
		claim.Claim.RunID == strings.TrimSpace(runID) && claim.Claim.MachineID == strings.TrimSpace(machineID)
}

func (controller Controller) heartbeatInterval() time.Duration {
	if controller.HeartbeatInterval > 0 {
		return controller.HeartbeatInterval
	}
	return time.Duration(controller.Profile.Runtime.HeartbeatIntervalSeconds) * time.Second
}
