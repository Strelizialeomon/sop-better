package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Strelizialeomon/sop-better/internal/config"
)

func TestDiscardingFailedCandidateCheckpointKeepsTwoSuccessfulCheckpoints(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("SOP_STATE_HOME", stateHome)
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".sop"), 0o755); err != nil {
		t.Fatal(err)
	}
	profile := config.Profile{
		SchemaVersion: 1,
		SOPVersion:    "0.1.0",
		Project: config.Project{
			Name:             "checkpoint-retention",
			DefaultBranch:    "main",
			SOPInitializedOn: "2026-07-10",
		},
		Ends:       []config.End{{Name: "app", Path: "app"}},
		Humans:     []config.Human{{ID: "owner", Roles: []string{"developer"}}},
		Risk:       "reversible",
		HouseStyle: []string{},
	}
	profileData, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	profileData = append(profileData, '\n')
	if err := os.WriteFile(filepath.Join(projectRoot, ".sop", "profile.json"), profileData, 0o644); err != nil {
		t.Fatal(err)
	}
	profileHash, err := profileDigest(profile)
	if err != nil {
		t.Fatal(err)
	}
	lock := Lock{
		SchemaVersion:    1,
		SOPVersion:       "0.1.0",
		GeneratorVersion: "0.1.0",
		RulesVersion:     "0.1.0",
		ProfileHash:      profileHash,
		Outputs:          []LockOutput{},
	}
	lockData, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	lockData = append(lockData, '\n')
	if err := os.WriteFile(filepath.Join(projectRoot, ".sop", "lock.json"), lockData, 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := createCheckpoint(projectRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := createCheckpoint(projectRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	failedCandidate, err := createCheckpoint(projectRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	discardCheckpoint(failedCandidate)

	checkpoints, err := filepath.Glob(filepath.Join(filepath.Dir(first), "*"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(checkpoints)
	want := []string{first, second}
	sort.Strings(want)
	if len(checkpoints) != len(want) || checkpoints[0] != want[0] || checkpoints[1] != want[1] {
		t.Fatalf("checkpoints after failed candidate = %v, want prior successful checkpoints %v", checkpoints, want)
	}
}
