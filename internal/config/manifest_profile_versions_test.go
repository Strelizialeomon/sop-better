package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestSupportsExplicitProfileSchemaVersions(t *testing.T) {
	manifestJSON := manifestNullFixture()
	manifestJSON["profile_schema_versions"] = []any{1, 2}
	data, err := json.Marshal(manifestJSON)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	for _, version := range []int{1, 2} {
		if !manifest.SupportsProfileSchema(version) {
			t.Errorf("SupportsProfileSchema(%d) = false", version)
		}
	}
	if manifest.SupportsProfileSchema(3) {
		t.Error("SupportsProfileSchema(3) = true")
	}
}

func TestRepositoryManifestSupportsLoopProfile(t *testing.T) {
	manifest, err := LoadManifest("../../manifest.json")
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if !manifest.SupportsProfileSchema(2) {
		t.Fatal("repository manifest does not support profile schema 2")
	}
}
