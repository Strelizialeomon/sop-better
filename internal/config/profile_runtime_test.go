package config

import (
	"strings"
	"testing"
)

func TestParseProfileAcceptsLoopV1RuntimeProfile(t *testing.T) {
	profile, err := ParseProfile([]byte(validLoopProfileJSON()))
	if err != nil {
		t.Fatalf("ParseProfile() error = %v", err)
	}
	if profile.Runtime == nil {
		t.Fatal("ParseProfile() runtime = nil")
	}
	if got, want := profile.Runtime.Mode, "loop-v1-experimental"; got != want {
		t.Fatalf("runtime.mode = %q, want %q", got, want)
	}
	if got, want := profile.Runtime.EvidenceTrust, "cooperative-local"; got != want {
		t.Fatalf("runtime.evidence_trust = %q, want %q", got, want)
	}
	if got, want := profile.Runtime.Trust.GitHub.TrustedActorIDs[0], int64(123456); got != want {
		t.Fatalf("trusted actor id = %d, want %d", got, want)
	}
}

func validLoopProfileJSON() string {
	return `{
  "schema_version": 2,
  "sop_version": "0.1.0",
  "project": {"name": "demo", "default_branch": "main", "sop_initialized_on": "2026-07-12"},
  "ends": [{"name": "backend", "path": "backend"}],
  "humans": [{"id": "owner", "roles": ["product", "developer"]}],
  "parallel_agents": false,
  "risk": "reversible",
  "house_style": [],
  "runtime": {
    "mode": "loop-v1-experimental",
    "tracker": "github",
    "start_mode": "manual",
    "auto_merge": "disabled",
    "evidence_trust": "cooperative-local",
    "lease_timeout_seconds": 600,
    "heartbeat_interval_seconds": 60,
    "trust": {"github": {"trusted_actor_ids": [123456]}},
    "checks": {"test": ["go test ./..."], "build": ["go build ./..."]}
  }
}`
}

func TestParseProfileRequiresRuntimeForSchemaVersionTwo(t *testing.T) {
	_, err := ParseProfile([]byte(`{
  "schema_version": 2,
  "sop_version": "0.1.0",
  "project": {"name": "demo", "default_branch": "main", "sop_initialized_on": "2026-07-12"},
  "ends": [{"name": "backend", "path": "backend"}],
  "humans": [{"id": "owner", "roles": ["product", "developer"]}],
  "parallel_agents": false,
  "risk": "reversible",
  "house_style": []
}`))
	if err == nil || !strings.Contains(err.Error(), "profile.runtime: is required for schema_version 2") {
		t.Fatalf("ParseProfile() error = %v, want missing runtime", err)
	}
}

func TestParseProfileRejectsUnsafeLoopRuntime(t *testing.T) {
	tests := []struct {
		name        string
		old         string
		replacement string
		want        string
	}{
		{"unknown mode", `"mode": "loop-v1-experimental"`, `"mode": "loop-v2"`, "profile.runtime.mode"},
		{"unsupported tracker", `"tracker": "github"`, `"tracker": "linear"`, "profile.runtime.tracker"},
		{"background start before release", `"start_mode": "manual"`, `"start_mode": "watch"`, "profile.runtime.start_mode"},
		{"automatic merge in mvp", `"auto_merge": "disabled"`, `"auto_merge": "low_risk"`, "profile.runtime.auto_merge"},
		{"wrong evidence trust", `"evidence_trust": "cooperative-local"`, `"evidence_trust": "github-app"`, "profile.runtime.evidence_trust"},
		{"nonpositive lease", `"lease_timeout_seconds": 600`, `"lease_timeout_seconds": 0`, "profile.runtime.lease_timeout_seconds"},
		{"nonpositive heartbeat", `"heartbeat_interval_seconds": 60`, `"heartbeat_interval_seconds": 0`, "profile.runtime.heartbeat_interval_seconds"},
		{"heartbeat exceeds one third", `"heartbeat_interval_seconds": 60`, `"heartbeat_interval_seconds": 201`, "must not exceed one third"},
		{"missing trusted actor", `"trusted_actor_ids": [123456]`, `"trusted_actor_ids": []`, "requires at least 1 trusted actor"},
		{"nonpositive trusted actor", `"trusted_actor_ids": [123456]`, `"trusted_actor_ids": [0]`, "trusted_actor_ids[0]"},
		{"duplicate trusted actor", `"trusted_actor_ids": [123456]`, `"trusted_actor_ids": [123456, 123456]`, "trusted_actor_ids[1]"},
		{"misleading trusted app", `"trusted_actor_ids": [123456]`, `"trusted_actor_ids": [123456], "trusted_app_ids": []`, "unknown field"},
		{"removed check triggers", `"checks": {"test": ["go test ./..."], "build": ["go build ./..."]}`, `"checks": {"test": ["go test ./..."], "build": ["go build ./..."]}, "check_triggers": {"test": ["internal/**"]}`, "unknown field"},
		{"removed delta paths", `"checks": {"test": ["go test ./..."], "build": ["go build ./..."]}`, `"checks": {"test": ["go test ./..."], "build": ["go build ./..."]}, "delta_review_paths": ["backend/**"]`, "unknown field"},
		{"missing checks", `"checks": {"test": ["go test ./..."], "build": ["go build ./..."]}`, `"checks": {}`, "profile.runtime.checks"},
		{"empty check command", `"go test ./..."`, `""`, "profile.runtime.checks.test[0]"},
		{"multiline check command", `"go test ./..."`, `"go test\nrm -rf ."`, "profile.runtime.checks.test[0]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := strings.Replace(validLoopProfileJSON(), test.old, test.replacement, 1)
			_, err := ParseProfile([]byte(data))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ParseProfile() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestParseProfileRejectsRuntimeOnLegacySchema(t *testing.T) {
	data := strings.Replace(validLoopProfileJSON(), `"schema_version": 2`, `"schema_version": 1`, 1)
	_, err := ParseProfile([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "profile.runtime: requires schema_version 2") {
		t.Fatalf("ParseProfile() error = %v, want legacy/runtime mismatch", err)
	}
}

func TestParseProfileRejectsMultiScopeLoopMVP(t *testing.T) {
	tests := []struct {
		name        string
		old         string
		replacement string
		want        string
	}{
		{
			"multiple ends",
			`"ends": [{"name": "backend", "path": "backend"}]`,
			`"ends": [{"name": "backend", "path": "backend"}, {"name": "frontend", "path": "frontend"}]`,
			"Loop MVP requires exactly 1 end",
		},
		{
			"parallel agents",
			`"parallel_agents": false`,
			`"parallel_agents": true`,
			"Loop MVP does not support parallel_agents",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := strings.Replace(validLoopProfileJSON(), test.old, test.replacement, 1)
			_, err := ParseProfile([]byte(data))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ParseProfile() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestParseProfileRejectsNonReversibleLoopMVP(t *testing.T) {
	data := strings.Replace(validLoopProfileJSON(), `"risk": "reversible"`, `"risk": "controlled"`, 1)
	_, err := ParseProfile([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "Loop MVP requires reversible risk") {
		t.Fatalf("ParseProfile() error = %v, want reversible risk requirement", err)
	}
}
