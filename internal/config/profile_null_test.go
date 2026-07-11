package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProfileRejectsJSONNullWhereSchemaRequiresConcreteTypes(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(map[string]any)
		wantField string
	}{
		{"parallel agents", func(profile map[string]any) { profile["parallel_agents"] = nil }, "profile.parallel_agents"},
		{"house style", func(profile map[string]any) { profile["house_style"] = nil }, "profile.house_style"},
		{"optional risk items", func(profile map[string]any) { profile["risk_items"] = nil }, "profile.risk_items"},
		{"optional description", func(profile map[string]any) { profile["project"].(map[string]any)["description"] = nil }, "profile.project.description"},
		{"end docs item", func(profile map[string]any) { profile["ends"].([]any)[0].(map[string]any)["docs"] = []any{nil} }, "profile.ends[0].docs[0]"},
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
			if err == nil || !strings.Contains(err.Error(), test.wantField) || !strings.Contains(err.Error(), "must not be null") {
				t.Fatalf("ParseProfile() error = %v, want %s null rejection", err, test.wantField)
			}
		})
	}
}
