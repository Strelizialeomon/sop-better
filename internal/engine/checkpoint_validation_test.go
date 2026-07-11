package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateCheckpointMetadataRejectsUnsafePaths(t *testing.T) {
	checkpointRoot := t.TempDir()
	projectRoot := t.TempDir()
	lock := Lock{
		SchemaVersion: 1,
		SOPVersion:    "0.1.0",
		RulesVersion:  "2026-07-10",
		Outputs: []LockOutput{{
			ID:         "root-agents",
			Target:     "AGENTS.md",
			Management: "full",
			Hash:       strings.Repeat("0", 64),
		}},
	}

	tests := []struct {
		name  string
		entry checkpointEntry
		want  string
	}{
		{
			name: "project target escapes repository",
			entry: checkpointEntry{
				ID: "root-agents", Target: "../outside.md", Management: "full",
				Existed: true, ContentFile: "files/0000", SHA256: strings.Repeat("0", 64),
			},
			want: "repository-relative",
		},
		{
			name: "content path escapes checkpoint",
			entry: checkpointEntry{
				ID: "root-agents", Target: "AGENTS.md", Management: "full",
				Existed: true, ContentFile: "../secret", SHA256: strings.Repeat("0", 64),
			},
			want: "content_file",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata := checkpoint{Format: checkpointFormat, Entries: []checkpointEntry{test.entry}}
			err := validateCheckpointMetadata(checkpointRoot, projectRoot, metadata, lock)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateCheckpointMetadata() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestValidateCheckpointMetadataRejectsContentSymlink(t *testing.T) {
	checkpointRoot := t.TempDir()
	projectRoot := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(checkpointRoot, "files"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(checkpointRoot, "files", "0000")); err != nil {
		t.Skipf("symlink is unavailable: %v", err)
	}
	lock := Lock{Outputs: []LockOutput{{
		ID: "root-agents", Target: "AGENTS.md", Management: "full", Hash: strings.Repeat("0", 64),
	}}}
	metadata := checkpoint{Format: checkpointFormat, Entries: []checkpointEntry{{
		ID: "root-agents", Target: "AGENTS.md", Management: "full",
		Existed: true, ContentFile: "files/0000", SHA256: strings.Repeat("0", 64),
	}}}

	err := validateCheckpointMetadata(checkpointRoot, projectRoot, metadata, lock)
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("validateCheckpointMetadata() error = %v, want symbolic-link rejection", err)
	}
}

func TestDecodeCheckpointRejectsUnknownAndTrailingJSON(t *testing.T) {
	for _, data := range []string{
		`{"format":1,"id":"cp","created_at":"now","project_id":"project","lock_sha256":"hash","entries":[],"unknown":true}`,
		`{"format":1,"id":"cp","created_at":"now","project_id":"project","lock_sha256":"hash","entries":[]} {}`,
	} {
		if _, err := decodeCheckpoint([]byte(data)); err == nil {
			t.Fatalf("decodeCheckpoint unexpectedly accepted %s", data)
		}
	}
}
