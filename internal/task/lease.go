package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrAlreadyClaimed = errors.New("task is already claimed")
	ErrLeaseLost      = errors.New("task lease is no longer owned by this run")
	ErrNoClaim        = errors.New("task has no active claim")
	ErrRefConflict    = errors.New("remote claim ref changed")
)

type Claim struct {
	RepoNodeID     string    `json:"repo_node_id"`
	IssueNumber    int       `json:"issue_number"`
	RunID          string    `json:"run_id"`
	MachineID      string    `json:"machine_id"`
	ActorID        int64     `json:"actor_id"`
	LeaseEpoch     int64     `json:"lease_epoch"`
	FencingToken   string    `json:"fencing_token"`
	ServerObserved time.Time `json:"server_observed_at"`
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
	StateRevision  int64     `json:"state_revision"`
	BaseSHA        string    `json:"base_sha"`
}

type StoredClaim struct {
	OID   string
	Claim Claim
}

type ClaimRequest struct {
	RepoNodeID    string
	IssueNumber   int
	RunID         string
	MachineID     string
	ActorID       int64
	StateRevision int64
	BaseSHA       string
}

type ClaimStore interface {
	ServerNow(context.Context) (time.Time, time.Duration, error)
	Read(context.Context, string, int) (StoredClaim, bool, error)
	Create(context.Context, Claim) (StoredClaim, error)
	CompareAndSwap(context.Context, string, Claim) (StoredClaim, error)
	Delete(context.Context, string, int, string) error
}

type LeaseService struct {
	Store    ClaimStore
	NewToken func() (string, error)
	TTL      time.Duration
}

func (service LeaseService) Claim(ctx context.Context, request ClaimRequest) (StoredClaim, error) {
	if err := service.validate(); err != nil {
		return StoredClaim{}, err
	}
	if err := validateClaimRequest(request); err != nil {
		return StoredClaim{}, err
	}
	token, err := service.NewToken()
	if err != nil {
		return StoredClaim{}, fmt.Errorf("generate fencing token: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return StoredClaim{}, errors.New("generated fencing token is empty")
	}
	now, uncertainty, err := service.serverNow(ctx)
	if err != nil {
		return StoredClaim{}, err
	}
	claim := Claim{
		RepoNodeID:     strings.TrimSpace(request.RepoNodeID),
		IssueNumber:    request.IssueNumber,
		RunID:          strings.TrimSpace(request.RunID),
		MachineID:      strings.TrimSpace(request.MachineID),
		ActorID:        request.ActorID,
		LeaseEpoch:     1,
		FencingToken:   token,
		ServerObserved: now,
		LeaseExpiresAt: now.Add(service.TTL - uncertainty),
		StateRevision:  request.StateRevision,
		BaseSHA:        strings.TrimSpace(request.BaseSHA),
	}
	stored, err := service.Store.Create(ctx, claim)
	if errors.Is(err, ErrRefConflict) {
		return StoredClaim{}, ErrAlreadyClaimed
	}
	if err != nil {
		return StoredClaim{}, fmt.Errorf("create task claim: %w", err)
	}
	return stored, nil
}

func (service LeaseService) Takeover(ctx context.Context, request ClaimRequest) (StoredClaim, error) {
	if err := service.validate(); err != nil {
		return StoredClaim{}, err
	}
	if err := validateClaimRequest(request); err != nil {
		return StoredClaim{}, err
	}
	current, exists, err := service.Store.Read(ctx, request.RepoNodeID, request.IssueNumber)
	if err != nil {
		return StoredClaim{}, fmt.Errorf("read current task claim: %w", err)
	}
	if !exists {
		return StoredClaim{}, ErrNoClaim
	}
	now, uncertainty, err := service.serverNow(ctx)
	if err != nil {
		return StoredClaim{}, err
	}
	if !now.Add(-uncertainty).After(current.Claim.LeaseExpiresAt) {
		return StoredClaim{}, ErrAlreadyClaimed
	}
	token, err := service.NewToken()
	if err != nil {
		return StoredClaim{}, fmt.Errorf("generate fencing token: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return StoredClaim{}, errors.New("generated fencing token is empty")
	}
	replacement := Claim{
		RepoNodeID:     strings.TrimSpace(request.RepoNodeID),
		IssueNumber:    request.IssueNumber,
		RunID:          strings.TrimSpace(request.RunID),
		MachineID:      strings.TrimSpace(request.MachineID),
		ActorID:        request.ActorID,
		LeaseEpoch:     current.Claim.LeaseEpoch + 1,
		FencingToken:   token,
		ServerObserved: now,
		LeaseExpiresAt: now.Add(service.TTL - uncertainty),
		StateRevision:  request.StateRevision,
		BaseSHA:        strings.TrimSpace(request.BaseSHA),
	}
	stored, err := service.Store.CompareAndSwap(ctx, current.OID, replacement)
	if errors.Is(err, ErrRefConflict) {
		return StoredClaim{}, ErrAlreadyClaimed
	}
	if err != nil {
		return StoredClaim{}, fmt.Errorf("replace expired task claim: %w", err)
	}
	return stored, nil
}

func (service LeaseService) Renew(ctx context.Context, current StoredClaim) (StoredClaim, error) {
	if err := service.validate(); err != nil {
		return StoredClaim{}, err
	}
	if strings.TrimSpace(current.OID) == "" {
		return StoredClaim{}, errors.New("stored claim OID is required")
	}
	now, uncertainty, err := service.serverNow(ctx)
	if err != nil {
		return StoredClaim{}, err
	}
	if !now.Add(uncertainty).Before(current.Claim.LeaseExpiresAt) {
		return StoredClaim{}, ErrLeaseLost
	}
	updated := current.Claim
	updated.ServerObserved = now
	updated.LeaseExpiresAt = now.Add(service.TTL - uncertainty)
	stored, err := service.Store.CompareAndSwap(ctx, current.OID, updated)
	if errors.Is(err, ErrRefConflict) {
		return StoredClaim{}, ErrLeaseLost
	}
	if err != nil {
		return StoredClaim{}, fmt.Errorf("renew task claim: %w", err)
	}
	return stored, nil
}

func (service LeaseService) Release(ctx context.Context, current StoredClaim) error {
	if err := service.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(current.OID) == "" {
		return errors.New("stored claim OID is required")
	}
	if err := service.Store.Delete(ctx, current.Claim.RepoNodeID, current.Claim.IssueNumber, current.OID); errors.Is(err, ErrRefConflict) {
		return ErrLeaseLost
	} else if err != nil {
		return fmt.Errorf("release task claim: %w", err)
	}
	return nil
}

func (service LeaseService) validate() error {
	if service.Store == nil {
		return errors.New("lease store is required")
	}
	if service.NewToken == nil {
		return errors.New("lease token generator is required")
	}
	if service.TTL <= 0 {
		return errors.New("lease TTL must be positive")
	}
	return nil
}

func (service LeaseService) serverNow(ctx context.Context) (time.Time, time.Duration, error) {
	now, uncertainty, err := service.Store.ServerNow(ctx)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("read GitHub server time: %w", err)
	}
	if now.IsZero() {
		return time.Time{}, 0, errors.New("GitHub server time is missing")
	}
	if uncertainty < 0 || uncertainty >= service.TTL {
		return time.Time{}, 0, errors.New("server time uncertainty must be non-negative and smaller than lease TTL")
	}
	return now, uncertainty, nil
}

func validateClaimRequest(request ClaimRequest) error {
	if strings.TrimSpace(request.RepoNodeID) == "" {
		return errors.New("claim repo_node_id is required")
	}
	if request.IssueNumber <= 0 {
		return errors.New("claim issue_number must be positive")
	}
	if strings.TrimSpace(request.RunID) == "" {
		return errors.New("claim run_id is required")
	}
	if strings.TrimSpace(request.MachineID) == "" {
		return errors.New("claim machine_id is required")
	}
	if request.ActorID <= 0 {
		return errors.New("claim actor_id must be positive")
	}
	if request.StateRevision < 0 {
		return errors.New("claim state_revision must be non-negative")
	}
	if strings.TrimSpace(request.BaseSHA) == "" {
		return errors.New("claim base_sha is required")
	}
	return nil
}
