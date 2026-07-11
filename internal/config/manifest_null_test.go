package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestRejectsJSONNullWhereSchemaRequiresConcreteTypes(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(map[string]any)
		wantField string
	}{
		{"slot required", func(manifest map[string]any) {
			manifest["slots"].(map[string]any)["project_name"].(map[string]any)["required"] = nil
		}, "manifest.slots.project_name.required"},
		{"component references", func(manifest map[string]any) {
			manifest["components"].(map[string]any)["root"].(map[string]any)["references"] = nil
		}, "manifest.components.root.references"},
		{"optional for each", func(manifest map[string]any) { manifest["outputs"].([]any)[0].(map[string]any)["for_each"] = nil }, "manifest.outputs[0].for_each"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := manifestNullFixture()
			test.mutate(manifest)
			data, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "manifest.json")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err = LoadManifest(path)
			if err == nil || !strings.Contains(err.Error(), test.wantField) || !strings.Contains(err.Error(), "must not be null") {
				t.Fatalf("LoadManifest() error = %v, want %s null rejection", err, test.wantField)
			}
		})
	}
}

func manifestNullFixture() map[string]any {
	return map[string]any{
		"schema_version":         1,
		"sop_version":            "0.1.0",
		"profile_schema_version": 1,
		"rules_version":          "2026-07-11",
		"standard":               map[string]any{"path": "STANDARD.md", "sha256": strings.Repeat("0", 64)},
		"slots": map[string]any{
			"project_name": map[string]any{"type": "string", "source": "/project/name", "required": true, "format": "inline"},
		},
		"components": map[string]any{
			"root": map[string]any{"template": "master/root.md", "rule_ids": []any{"SOP-CORE"}, "slots": []any{"project_name"}, "references": []any{}},
		},
		"outputs": []any{
			map[string]any{"id": "root", "target": "AGENTS.md", "when": "always", "management": "block", "components": []any{map[string]any{"id": "root", "when": "always"}}},
		},
	}
}
