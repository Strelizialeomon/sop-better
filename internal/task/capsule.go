package task

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/Strelizialeomon/sop-better/internal/config"
)

const maxCapsuleBytes = 4 * 1024

type Snapshot struct {
	RepoNodeID    string
	IssueNumber   int
	Goal          string
	Acceptance    []string
	DocumentURL   string
	UntrustedBody string
}

type Attestation struct {
	RepoNodeID   string    `json:"repo_node_id"`
	IssueNumber  int       `json:"issue_number"`
	SnapshotHash string    `json:"snapshot_hash"`
	ActorID      int64     `json:"actor_id"`
	SOPVersion   string    `json:"sop_version"`
	ServerTime   time.Time `json:"server_time"`
}

type Capsule struct {
	Task            int                 `json:"task"`
	Goal            string              `json:"goal"`
	Acceptance      []string            `json:"acceptance"`
	Role            string              `json:"role"`
	AllowedPaths    []string            `json:"allowed_paths"`
	ForbiddenPaths  []string            `json:"forbidden_paths,omitempty"`
	State           string              `json:"state"`
	Phase           string              `json:"phase"`
	RequiredContext []ContextReference  `json:"required_context,omitempty"`
	Checks          map[string][]string `json:"checks"`
	Risk            Risk                `json:"risk"`
	NextAction      string              `json:"next_action"`
	StopConditions  []string            `json:"stop_conditions"`
	Sources         map[string]string   `json:"sources"`
	SnapshotHash    string              `json:"snapshot_hash"`
}

type ContextReference struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
	Trust string `json:"trust"`
}

type Risk struct {
	Class              string   `json:"class"`
	MatchedRules       []string `json:"matched_rules"`
	Provenance         string   `json:"provenance"`
	ReversibleEvidence string   `json:"reversible_evidence"`
	Approvals          []string `json:"approvals"`
}

type normalizedSnapshot struct {
	RepoNodeID  string   `json:"repo_node_id"`
	IssueNumber int      `json:"issue_number"`
	Goal        string   `json:"goal"`
	Acceptance  []string `json:"acceptance"`
	DocumentURL string   `json:"document_url,omitempty"`
}

func SnapshotHash(snapshot Snapshot) (string, error) {
	normalized, err := normalizeSnapshot(snapshot)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("encode normalized task snapshot: %w", err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func BuildCapsule(profile config.Profile, snapshot Snapshot, attestation Attestation, currentActorID int64) (Capsule, error) {
	if profile.Runtime == nil || profile.Runtime.Mode != "loop-v1-experimental" {
		return Capsule{}, errors.New("task capsule requires loop-v1-experimental runtime")
	}
	if len(profile.Ends) != 1 {
		return Capsule{}, errors.New("task capsule requires exactly one end in the Loop MVP")
	}
	if !slices.Contains(profile.Runtime.Trust.GitHub.TrustedActorIDs, currentActorID) {
		return Capsule{}, fmt.Errorf("current GitHub actor %d is not trusted by the project profile", currentActorID)
	}
	if attestation.ActorID != currentActorID {
		return Capsule{}, errors.New("ready attestation actor does not match the current GitHub actor")
	}
	if attestation.ServerTime.IsZero() {
		return Capsule{}, errors.New("ready attestation server_time is required")
	}
	normalized, err := normalizeSnapshot(snapshot)
	if err != nil {
		return Capsule{}, err
	}
	hash, err := SnapshotHash(snapshot)
	if err != nil {
		return Capsule{}, err
	}
	if attestation.RepoNodeID != normalized.RepoNodeID || attestation.IssueNumber != normalized.IssueNumber ||
		attestation.SnapshotHash != hash || attestation.SOPVersion != profile.SOPVersion {
		return Capsule{}, errors.New("ready attestation does not match repo, issue, snapshot, or SOP version")
	}
	if profile.Risk != "reversible" {
		return Capsule{}, errors.New("Loop MVP only accepts reversible project risk")
	}

	end := profile.Ends[0]
	allowedPath := strings.TrimSuffix(path.Clean(end.Path), "/") + "/"
	checks := cloneChecks(profile.Runtime.Checks)
	capsule := Capsule{
		Task:         normalized.IssueNumber,
		Goal:         normalized.Goal,
		Acceptance:   slices.Clone(normalized.Acceptance),
		Role:         end.Name,
		AllowedPaths: []string{allowedPath},
		State:        "running",
		Phase:        "investigate",
		Checks:       checks,
		Risk: Risk{
			Class:              "low",
			MatchedRules:       []string{"reversible-code-change"},
			Provenance:         "project-profile://risk",
			ReversibleEvidence: "task branch only; no automatic merge in Loop MVP",
			Approvals:          []string{},
		},
		NextAction: "inspect approved evidence and reproduce the issue",
		StopConditions: []string{
			"a high-risk action is required",
			"the task must expand beyond allowed_paths",
			"the lease guard fails",
		},
		Sources: map[string]string{
			"goal":          fmt.Sprintf("github-issue://%d#approved-snapshot", normalized.IssueNumber),
			"acceptance":    fmt.Sprintf("github-issue://%d#approved-snapshot", normalized.IssueNumber),
			"role":          "project-profile://ends/0/name",
			"allowed_paths": "project-profile://ends/0/path",
			"checks":        "project-profile://runtime/checks",
			"risk":          "project-profile://risk",
		},
		SnapshotHash: hash,
	}
	if normalized.DocumentURL != "" {
		capsule.RequiredContext = []ContextReference{{Kind: "document", Value: normalized.DocumentURL, Trust: "untrusted-data"}}
	}
	data, err := json.Marshal(capsule)
	if err != nil {
		return Capsule{}, fmt.Errorf("encode task capsule: %w", err)
	}
	if len(data) > maxCapsuleBytes {
		return Capsule{}, fmt.Errorf("task capsule is %d bytes; limit is %d", len(data), maxCapsuleBytes)
	}
	return capsule, nil
}

func normalizeSnapshot(snapshot Snapshot) (normalizedSnapshot, error) {
	normalized := normalizedSnapshot{
		RepoNodeID:  strings.TrimSpace(snapshot.RepoNodeID),
		IssueNumber: snapshot.IssueNumber,
		Goal:        strings.TrimSpace(snapshot.Goal),
		DocumentURL: strings.TrimSpace(snapshot.DocumentURL),
	}
	for _, acceptance := range snapshot.Acceptance {
		if trimmed := strings.TrimSpace(acceptance); trimmed != "" {
			normalized.Acceptance = append(normalized.Acceptance, trimmed)
		}
	}
	if normalized.RepoNodeID == "" {
		return normalizedSnapshot{}, errors.New("task snapshot repo_node_id is required")
	}
	if normalized.IssueNumber <= 0 {
		return normalizedSnapshot{}, errors.New("task snapshot issue_number must be positive")
	}
	if normalized.Goal == "" {
		return normalizedSnapshot{}, errors.New("task snapshot goal is required")
	}
	if len(normalized.Acceptance) == 0 {
		return normalizedSnapshot{}, errors.New("task snapshot acceptance requires at least one item")
	}
	return normalized, nil
}

func cloneChecks(checks map[string][]string) map[string][]string {
	cloned := make(map[string][]string, len(checks))
	for group, commands := range checks {
		cloned[group] = slices.Clone(commands)
	}
	return cloned
}
