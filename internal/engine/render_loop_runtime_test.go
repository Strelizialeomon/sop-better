package engine

import (
	"strings"
	"testing"

	"github.com/Strelizialeomon/sop-better/internal/config"
)

func TestRenderLoopProfileUsesThinRuntimeAgentEntry(t *testing.T) {
	profile := config.Profile{
		SchemaVersion: 2,
		SOPVersion:    "0.1.0",
		Project: config.Project{
			Name:             "demo",
			DefaultBranch:    "main",
			SOPInitializedOn: "2026-07-12",
		},
		Ends:       []config.End{{Name: "backend", Path: "backend"}},
		Humans:     []config.Human{{ID: "owner", Roles: []string{"product", "developer"}}},
		Risk:       "reversible",
		HouseStyle: []string{},
		Runtime: &config.Runtime{
			Mode:                     "loop-v1-experimental",
			Tracker:                  "github",
			StartMode:                "manual",
			AutoMerge:                "disabled",
			LeaseTimeoutSeconds:      600,
			HeartbeatIntervalSeconds: 60,
			Trust: config.RuntimeTrust{GitHub: config.GitHubTrust{
				TrustedActorIDs: []int64{123456},
			}},
			Checks: map[string][]string{"test": {"go test ./..."}},
		},
	}
	manifest, err := config.LoadManifest("../../manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	outputs, _, err := Render(profile, manifest, "../..", "0.1.0-dev")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	var agents string
	for _, output := range outputs {
		if output.Lock.ID == "root-agents" {
			agents = string(output.Content)
			break
		}
	}
	if agents == "" {
		t.Fatal("Render() did not produce root-agents")
	}
	if !strings.Contains(agents, "$sop-run") {
		t.Fatalf("loop AGENTS.md does not route through $sop-run:\n%s", agents)
	}
	if strings.Contains(agents, "issue、doc、PR 的操作细则") {
		t.Fatalf("loop AGENTS.md contains legacy workflow detail:\n%s", agents)
	}
	if len(agents) > 4*1024 {
		t.Fatalf("loop AGENTS.md size = %d bytes, want <= 4096", len(agents))
	}
}
