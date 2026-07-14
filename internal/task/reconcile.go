package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrStateConflict = errors.New("issue state revision changed")

type BusinessState string

const (
	StateReady   BusinessState = "ready"
	StateRunning BusinessState = "running"
	StateWaiting BusinessState = "waiting"
	StateDone    BusinessState = "done"
)

type IssueState struct {
	State    BusinessState
	Revision int64
}

type StateEvent struct {
	EventID                  string              `json:"event_id"`
	StateRevision            int64               `json:"state_revision"`
	ExpectedPreviousRevision int64               `json:"expected_previous_revision"`
	RunID                    string              `json:"run_id,omitempty"`
	LeaseEpoch               int64               `json:"lease_epoch,omitempty"`
	FencingToken             string              `json:"fencing_token,omitempty"`
	SourceActor              string              `json:"source_actor"`
	SourceServerTime         time.Time           `json:"source_server_time"`
	From                     BusinessState       `json:"from"`
	To                       BusinessState       `json:"to"`
	Reason                   string              `json:"reason"`
	SnapshotHash             string              `json:"snapshot_hash,omitempty"`
	Evidence                 *CompletionEvidence `json:"evidence,omitempty"`
}

type IssueStateStore interface {
	ReadState(context.Context, string, int) (IssueState, error)
	AppendStateEvent(context.Context, string, int, StateEvent) (IssueState, error)
}

type IssueStateProjector interface {
	ProjectState(context.Context, string, int, BusinessState) error
}

type ReconcileAction string

const (
	ReconcileNoop          ReconcileAction = "noop"
	ReconcileAppendedState ReconcileAction = "appended-state"
	ReconcileReleasedClaim ReconcileAction = "released-claim"
)

type ReconcileRequest struct {
	RepoNodeID    string
	IssueNumber   int
	SnapshotHash  string
	RecoveryClaim *ClaimRequest
}

type ReconcileResult struct {
	State    BusinessState
	Revision int64
	Action   ReconcileAction
}

type Reconciler struct {
	Claims     ClaimStore
	Issues     IssueStateStore
	Leases     LeaseService
	NewEventID func() (string, error)
}

func (reconciler Reconciler) Transition(ctx context.Context, claim StoredClaim, target BusinessState, reason string, evidence *CompletionEvidence) (ReconcileResult, error) {
	issue, err := reconciler.Issues.ReadState(ctx, claim.Claim.RepoNodeID, claim.Claim.IssueNumber)
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("read canonical issue state: %w", err)
	}
	runningTransition := issue.State == StateRunning && target == StateWaiting
	waitingCompletion := issue.State == StateWaiting && target == StateDone && claim.Claim.StateRevision == issue.Revision
	if !runningTransition && !waitingCompletion {
		if issue.State == StateWaiting && target == StateDone {
			return ReconcileResult{}, errors.New("cannot finish waiting task with a claim from a stale waiting revision")
		}
		return ReconcileResult{}, fmt.Errorf("cannot transition task from %s to %s", issue.State, target)
	}
	guarded, err := reconciler.Leases.Renew(ctx, claim)
	if err != nil {
		return ReconcileResult{}, err
	}
	eventID, err := reconciler.NewEventID()
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("generate state event ID: %w", err)
	}
	event := StateEvent{
		EventID: eventID, StateRevision: issue.Revision + 1, ExpectedPreviousRevision: issue.Revision,
		RunID: guarded.Claim.RunID, LeaseEpoch: guarded.Claim.LeaseEpoch, FencingToken: guarded.Claim.FencingToken,
		SourceActor: "runtime-runner", SourceServerTime: guarded.Claim.ServerObserved,
		From: issue.State, To: target, Reason: strings.TrimSpace(reason), Evidence: evidence,
	}
	updated, err := reconciler.Issues.AppendStateEvent(ctx, guarded.Claim.RepoNodeID, guarded.Claim.IssueNumber, event)
	if errors.Is(err, ErrStateConflict) {
		return ReconcileResult{}, ErrStateConflict
	}
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("append terminal issue state event: %w", err)
	}
	if projector, ok := reconciler.Issues.(IssueStateProjector); ok {
		if err := projector.ProjectState(ctx, guarded.Claim.RepoNodeID, guarded.Claim.IssueNumber, target); err != nil {
			return ReconcileResult{}, fmt.Errorf("terminal event was written but issue projection needs reconcile: %w", err)
		}
	}
	if err := reconciler.Leases.Release(ctx, guarded); err != nil {
		return ReconcileResult{}, fmt.Errorf("terminal state was written but claim cleanup needs reconcile: %w", err)
	}
	return ReconcileResult{State: updated.State, Revision: updated.Revision, Action: ReconcileReleasedClaim}, nil
}

func (reconciler Reconciler) Reconcile(ctx context.Context, request ReconcileRequest) (ReconcileResult, error) {
	if err := reconciler.validate(request); err != nil {
		return ReconcileResult{}, err
	}
	issue, err := reconciler.Issues.ReadState(ctx, request.RepoNodeID, request.IssueNumber)
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("read canonical issue state: %w", err)
	}
	claim, claimExists, err := reconciler.Claims.Read(ctx, request.RepoNodeID, request.IssueNumber)
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("read task claim during reconcile: %w", err)
	}

	if issue.State == StateWaiting || issue.State == StateDone {
		if projector, ok := reconciler.Issues.(IssueStateProjector); ok {
			if err := projector.ProjectState(ctx, request.RepoNodeID, request.IssueNumber, issue.State); err != nil {
				return ReconcileResult{}, fmt.Errorf("repair issue state projection: %w", err)
			}
		}
		if !claimExists {
			return ReconcileResult{State: issue.State, Revision: issue.Revision, Action: ReconcileNoop}, nil
		}
		if err := reconciler.Leases.Release(ctx, claim); err != nil {
			return ReconcileResult{}, err
		}
		return ReconcileResult{State: issue.State, Revision: issue.Revision, Action: ReconcileReleasedClaim}, nil
	}

	active := false
	if claimExists {
		active, err = reconciler.claimIsDefinitelyActive(ctx, claim)
		if err != nil {
			return ReconcileResult{}, err
		}
	}
	if issue.State == StateReady && active {
		guarded, err := reconciler.Leases.Renew(ctx, claim)
		if err != nil {
			return ReconcileResult{}, err
		}
		return reconciler.appendState(ctx, request, issue, guarded, StateRunning, "claim-created")
	}
	if issue.State == StateReady && claimExists {
		if err := reconciler.Leases.Release(ctx, claim); err != nil {
			return ReconcileResult{}, err
		}
		return ReconcileResult{State: issue.State, Revision: issue.Revision, Action: ReconcileReleasedClaim}, nil
	}
	if issue.State == StateRunning && !active {
		if request.RecoveryClaim == nil {
			return ReconcileResult{}, errors.New("orphaned running task requires an atomic recovery claim")
		}
		recoveryRequest := *request.RecoveryClaim
		recoveryRequest.RepoNodeID = request.RepoNodeID
		recoveryRequest.IssueNumber = request.IssueNumber
		recoveryRequest.StateRevision = issue.Revision
		var recovery StoredClaim
		if claimExists {
			recovery, err = reconciler.Leases.Takeover(ctx, recoveryRequest)
		} else {
			recovery, err = reconciler.Leases.Claim(ctx, recoveryRequest)
		}
		if err != nil {
			return ReconcileResult{}, fmt.Errorf("acquire orphan-recovery claim: %w", err)
		}
		result, err := reconciler.appendState(ctx, request, issue, recovery, StateWaiting, "missing-active-claim")
		if err != nil {
			if cleanupErr := reconciler.Leases.Release(ctx, recovery); cleanupErr != nil {
				return ReconcileResult{}, fmt.Errorf("reconcile state failed: %v; recovery claim cleanup also failed: %w", err, cleanupErr)
			}
			return ReconcileResult{}, err
		}
		if err := reconciler.Leases.Release(ctx, recovery); err != nil {
			return ReconcileResult{}, err
		}
		return result, nil
	}
	return ReconcileResult{State: issue.State, Revision: issue.Revision, Action: ReconcileNoop}, nil
}

func (reconciler Reconciler) appendState(
	ctx context.Context,
	request ReconcileRequest,
	issue IssueState,
	claim StoredClaim,
	target BusinessState,
	reason string,
) (ReconcileResult, error) {
	eventID, err := reconciler.NewEventID()
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("generate state event ID: %w", err)
	}
	if strings.TrimSpace(eventID) == "" {
		return ReconcileResult{}, errors.New("generated state event ID is empty")
	}
	event := StateEvent{
		EventID:                  eventID,
		StateRevision:            issue.Revision + 1,
		ExpectedPreviousRevision: issue.Revision,
		RunID:                    claim.Claim.RunID,
		LeaseEpoch:               claim.Claim.LeaseEpoch,
		FencingToken:             claim.Claim.FencingToken,
		SourceActor:              "runtime-reconciler",
		SourceServerTime:         claim.Claim.ServerObserved,
		From:                     issue.State,
		To:                       target,
		Reason:                   reason,
		SnapshotHash:             strings.TrimSpace(request.SnapshotHash),
	}
	updated, err := reconciler.Issues.AppendStateEvent(ctx, request.RepoNodeID, request.IssueNumber, event)
	if errors.Is(err, ErrStateConflict) {
		return ReconcileResult{}, ErrStateConflict
	}
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("append canonical issue state event: %w", err)
	}
	return ReconcileResult{State: updated.State, Revision: updated.Revision, Action: ReconcileAppendedState}, nil
}

func (reconciler Reconciler) claimIsDefinitelyActive(ctx context.Context, claim StoredClaim) (bool, error) {
	now, uncertainty, err := reconciler.Claims.ServerNow(ctx)
	if err != nil {
		return false, fmt.Errorf("read GitHub server time during reconcile: %w", err)
	}
	if now.IsZero() || uncertainty < 0 {
		return false, errors.New("GitHub server time evidence is invalid")
	}
	return now.Add(uncertainty).Before(claim.Claim.LeaseExpiresAt), nil
}

func (reconciler Reconciler) validate(request ReconcileRequest) error {
	if reconciler.Claims == nil || reconciler.Issues == nil {
		return errors.New("reconcile claim and issue stores are required")
	}
	if reconciler.Leases.Store == nil {
		return errors.New("reconcile lease service is required")
	}
	if reconciler.NewEventID == nil {
		return errors.New("reconcile event ID generator is required")
	}
	if strings.TrimSpace(request.RepoNodeID) == "" || request.IssueNumber <= 0 {
		return errors.New("reconcile repo_node_id and positive issue_number are required")
	}
	return nil
}
