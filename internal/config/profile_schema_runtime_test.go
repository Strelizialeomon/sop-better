package config

import (
	"encoding/json"
	"os"
	"testing"
)

func TestProfileSchemaDeclaresLoopRuntimeContract(t *testing.T) {
	data, err := os.ReadFile("../../schemas/profile.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	versions, ok := properties["schema_version"].(map[string]any)["enum"].([]any)
	if !ok {
		t.Fatal("profile schema_version does not declare enum [1, 2]")
	}
	if len(versions) != 2 || versions[0] != float64(1) || versions[1] != float64(2) {
		t.Fatalf("schema_version enum = %v, want [1 2]", versions)
	}
	runtimeProperty, ok := properties["runtime"].(map[string]any)
	if !ok {
		t.Fatal("profile schema does not declare runtime")
	}
	runtimeRef, _ := runtimeProperty["$ref"].(string)
	if runtimeRef != "#/$defs/runtime" {
		t.Fatalf("runtime ref = %q", runtimeRef)
	}
	runtimeDefinition := schema["$defs"].(map[string]any)["runtime"].(map[string]any)
	required := runtimeDefinition["required"].([]any)
	for _, field := range []string{"mode", "tracker", "start_mode", "auto_merge", "evidence_trust", "lease_timeout_seconds", "heartbeat_interval_seconds", "trust", "checks"} {
		if !containsJSONText(required, field) {
			t.Errorf("runtime required does not contain %q: %v", field, required)
		}
	}
	runtimeProperties := runtimeDefinition["properties"].(map[string]any)
	if got := runtimeProperties["evidence_trust"].(map[string]any)["const"]; got != "cooperative-local" {
		t.Fatalf("runtime evidence_trust const = %v", got)
	}
	githubTrust := schema["$defs"].(map[string]any)["runtime_trust"].(map[string]any)["properties"].(map[string]any)["github"].(map[string]any)
	githubProperties := githubTrust["properties"].(map[string]any)
	if _, exists := githubProperties["trusted_app_ids"]; exists {
		t.Fatal("cooperative-local schema must reject trusted_app_ids instead of implying App isolation")
	}
}

func TestManifestSchemaDeclaresSupportedProfileVersions(t *testing.T) {
	data, err := os.ReadFile("../../schemas/manifest.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	versions, ok := properties["profile_schema_versions"].(map[string]any)
	if !ok {
		t.Fatal("manifest schema does not declare profile_schema_versions")
	}
	items := versions["items"].(map[string]any)
	enum, ok := items["enum"].([]any)
	if !ok || len(enum) != 2 || enum[0] != float64(1) || enum[1] != float64(2) {
		t.Fatalf("profile_schema_versions item enum = %v, want [1 2]", enum)
	}
}

func containsJSONText(values []any, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
