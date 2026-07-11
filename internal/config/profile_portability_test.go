package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProfileRejectsMachineAbsolutePathsInPortableTextFields(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(map[string]any)
		wantField string
	}{
		{
			name: "unix house style",
			mutate: func(profile map[string]any) {
				profile["house_style"] = []any{"/Users/alice/code/go-style"}
			},
			wantField: "profile.house_style[0]",
		},
		{
			name: "windows house style",
			mutate: func(profile map[string]any) {
				profile["house_style"] = []any{`C:\Users\alice\code\go-style`}
			},
			wantField: "profile.house_style[0]",
		},
		{
			name: "unc house style",
			mutate: func(profile map[string]any) {
				profile["house_style"] = []any{`\\server\share\style`}
			},
			wantField: "profile.house_style[0]",
		},
		{
			name: "embedded unix path in description",
			mutate: func(profile map[string]any) {
				profile["project"].(map[string]any)["description"] = "copy rules from /home/alice/style"
			},
			wantField: "profile.project.description",
		},
		{
			name: "absolute sdk in stack",
			mutate: func(profile map[string]any) {
				profile["ends"].([]any)[0].(map[string]any)["stack"] = "/opt/toolchain"
			},
			wantField: "profile.ends[0].stack",
		},
		{
			name: "embedded windows path in risk item",
			mutate: func(profile map[string]any) {
				profile["risk_items"] = []any{`do not edit D:\prod\secrets`}
			},
			wantField: "profile.risk_items[0]",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := portableProfileFixture()
			test.mutate(profile)
			data, err := json.Marshal(profile)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ParseProfile(data)
			if err == nil || !strings.Contains(err.Error(), test.wantField) || !strings.Contains(err.Error(), "machine absolute path") {
				t.Fatalf("ParseProfile() error = %v, want %s machine absolute path", err, test.wantField)
			}
		})
	}
}

func TestProfileAllowsPortableReferencesAndNonFilesystemSlashText(t *testing.T) {
	profile := portableProfileFixture()
	profile["house_style"] = []any{"go_dispatch_backend", "github.com/acme/style", "https://github.com/acme/style"}
	profile["project"].(map[string]any)["description"] = "API route /v1/users"
	profile["project"].(map[string]any)["default_branch"] = "feature/portable-profile"
	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseProfile(data); err != nil {
		t.Fatalf("portable profile was rejected: %v", err)
	}
}

func TestProfileRejectsInvalidGitDefaultBranches(t *testing.T) {
	for _, branch := range []string{"-danger", "feature..hidden", "topic.lock", "refs/heads/main", "bad branch", "main~1", "topic/"} {
		t.Run(branch, func(t *testing.T) {
			profile := portableProfileFixture()
			profile["project"].(map[string]any)["default_branch"] = branch
			data, err := json.Marshal(profile)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ParseProfile(data)
			if err == nil || !strings.Contains(err.Error(), "profile.project.default_branch: must be a valid Git branch name") {
				t.Fatalf("ParseProfile() error = %v, want invalid Git branch", err)
			}
		})
	}
}

func portableProfileFixture() map[string]any {
	return map[string]any{
		"schema_version":  1,
		"sop_version":     "0.1.0",
		"project":         map[string]any{"name": "portable", "description": "portable profile", "default_branch": "main", "sop_initialized_on": "2026-07-10"},
		"ends":            []any{map[string]any{"name": "backend", "path": "backend", "stack": "Go"}},
		"humans":          []any{map[string]any{"id": "owner", "roles": []any{"developer"}}},
		"parallel_agents": false,
		"risk":            "reversible",
		"house_style":     []any{},
		"risk_items":      []any{"push protected branch"},
	}
}
