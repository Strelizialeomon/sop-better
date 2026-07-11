package engine

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Strelizialeomon/sop-better/internal/config"
)

func TestWriteRecoversInterruptedProjectTransactionBeforeInspectingManagedFiles(t *testing.T) {
	t.Setenv("SOP_STATE_HOME", t.TempDir())
	projectRoot := t.TempDir()
	profile := config.Profile{
		SchemaVersion: 1,
		SOPVersion:    "0.1.0",
		Project: config.Project{
			Name:             "recovery",
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

	initialOutputs, initialLock := recoveryRenderCandidate(t, profile, "# initial\n")
	if err := Write(projectRoot, initialOutputs, initialLock, profile, profileData); err != nil {
		t.Fatalf("initial write: %v", err)
	}
	candidateOutputs, candidateLock := recoveryRenderCandidate(t, profile, "# candidate\n")
	candidateLockData, err := encodeLock(candidateLock)
	if err != nil {
		t.Fatal(err)
	}
	err = applyTransaction(projectRoot, []transactionFile{
		{Target: "AGENTS.md", Content: candidateOutputs[0].Content, Mode: 0o644},
		{Target: ".sop/lock.json", Content: candidateLockData, Mode: 0o644},
	}, transactionOptions{
		AfterReplace: func(replaced int) error {
			if replaced == 1 {
				return errors.New("simulated process death")
			}
			return nil
		},
		LeaveInterrupted: true,
	})
	if err == nil {
		t.Fatal("interrupted transaction unexpectedly succeeded")
	}

	if err := Write(projectRoot, candidateOutputs, candidateLock, profile, profileData); err != nil {
		t.Fatalf("write did not recover before inspecting managed files: %v", err)
	}
	assertTransactionFixture(t, projectRoot, "AGENTS.md", string(candidateOutputs[0].Content))
	actualLock, err := os.ReadFile(filepath.Join(projectRoot, ".sop", "lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(actualLock) != string(candidateLockData) {
		t.Fatalf("lock was not committed after recovery:\n%s", actualLock)
	}
	assertNoTransactionResidue(t, projectRoot)
}

func recoveryRenderCandidate(t *testing.T, profile config.Profile, content string) ([]RenderedOutput, Lock) {
	t.Helper()
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalText(content))))
	managed, err := wrapManagedBlock("root-agents", "0.1.0", hash, "html", canonicalText(content))
	if err != nil {
		t.Fatal(err)
	}
	profileHash, err := profileDigest(profile)
	if err != nil {
		t.Fatal(err)
	}
	outputLock := LockOutput{
		ID:          "root-agents",
		Target:      "AGENTS.md",
		Management:  "block",
		MarkerStyle: "html",
		Components:  []string{"root"},
		Hash:        hash,
	}
	return []RenderedOutput{{Target: "AGENTS.md", Content: []byte(managed), Lock: outputLock}}, Lock{
		SchemaVersion:    1,
		SOPVersion:       "0.1.0",
		GeneratorVersion: "0.1.0",
		RulesVersion:     "0.1.0",
		ProfileHash:      profileHash,
		Outputs:          []LockOutput{outputLock},
	}
}
