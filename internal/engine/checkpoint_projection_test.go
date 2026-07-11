package engine

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Strelizialeomon/sop-better/internal/config"
)

func TestRollbackProjectionTreatsPlannedRemovalsAsMissing(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "later-only.md"), []byte("later\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "# restored\n\n[later](later-only.md)\n"
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalText(body))))
	block, err := wrapManagedBlock("restored", "0.1.0", hash, "html", canonicalText(body))
	if err != nil {
		t.Fatal(err)
	}
	lock := Lock{SOPVersion: "0.1.0", Outputs: []LockOutput{{
		ID:          "restored",
		Target:      "docs/restored.md",
		Management:  "block",
		MarkerStyle: "html",
		Hash:        hash,
	}}}
	profile := config.Profile{
		Project: config.Project{DefaultBranch: "main"},
	}
	err = validateRollbackProjection(root, lock, []transactionFile{
		{Target: "docs/restored.md", Content: []byte(block), Mode: 0o644},
		{Target: "docs/later-only.md", Remove: true},
	}, profile)
	if err == nil {
		t.Fatal("projection unexpectedly accepted a link to a planned removal")
	}
	if !strings.Contains(err.Error(), "link later-only.md does not exist") {
		t.Fatalf("projection returned the wrong error: %v", err)
	}
}
