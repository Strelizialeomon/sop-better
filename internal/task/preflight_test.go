package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckClaimWorkflowIsolationRejectsUnfilteredPushWorkflow(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, ".github", "workflows", "ci.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("on:\n  push:\n\njobs: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckClaimWorkflowIsolation(repo); err == nil {
		t.Fatal("unfiltered push workflow passed claim-ref isolation")
	}
	if err := os.WriteFile(path, []byte("on:\n  push:\n    branches-ignore:\n      - 'sop/claims/**'\n\njobs: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckClaimWorkflowIsolation(repo); err != nil {
		t.Fatalf("filtered workflow: %v", err)
	}
}

func TestCheckClaimWorkflowIsolationAllowsRepositoryWithoutWorkflows(t *testing.T) {
	if err := CheckClaimWorkflowIsolation(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
