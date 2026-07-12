package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	ruleMarkerPattern = regexp.MustCompile(`<!--\s*rule:([A-Z0-9-]+)\s*-->`)
	slotPattern       = regexp.MustCompile(`\{\{([a-zA-Z0-9_]+)\}\}`)
)

type Manifest struct {
	SchemaVersion         int                      `json:"schema_version"`
	SOPVersion            string                   `json:"sop_version"`
	ProfileSchemaVersion  int                      `json:"profile_schema_version"`
	ProfileSchemaVersions []int                    `json:"profile_schema_versions,omitempty"`
	RulesVersion          string                   `json:"rules_version"`
	Standard              StandardSpec             `json:"standard"`
	Slots                 map[string]SlotSpec      `json:"slots"`
	Components            map[string]ComponentSpec `json:"components"`
	Outputs               []OutputSpec             `json:"outputs"`
}

type StandardSpec struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type SlotSpec struct {
	Type     string `json:"type"`
	Source   string `json:"source"`
	Required bool   `json:"required"`
	Format   string `json:"format,omitempty"`
}

type ComponentSpec struct {
	Template   string   `json:"template"`
	RuleIDs    []string `json:"rule_ids"`
	Slots      []string `json:"slots"`
	References []string `json:"references"`
}

type OutputSpec struct {
	ID          string         `json:"id"`
	Target      string         `json:"target"`
	When        string         `json:"when"`
	ForEach     string         `json:"for_each,omitempty"`
	Management  string         `json:"management"`
	MarkerStyle string         `json:"marker_style,omitempty"`
	Components  []ComponentRef `json:"components"`
}

type ComponentRef struct {
	ID   string `json:"id"`
	When string `json:"when"`
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Manifest{}, errors.New("manifest.json: file does not exist")
		}
		return Manifest{}, fmt.Errorf("read manifest.json: %w", err)
	}

	var manifest Manifest
	if err := strictDecodeJSON(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest.json: %w", err)
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest.json: %w", err)
	}
	if err := rejectJSONNull(raw, "manifest"); err != nil {
		return Manifest{}, err
	}
	if manifest.SchemaVersion != 1 {
		return Manifest{}, errors.New("manifest.schema_version: must be 1")
	}
	if manifest.ProfileSchemaVersion != 1 {
		return Manifest{}, errors.New("manifest.profile_schema_version: must be 1")
	}
	if len(manifest.ProfileSchemaVersions) > 0 {
		seen := make(map[int]struct{}, len(manifest.ProfileSchemaVersions))
		for index, version := range manifest.ProfileSchemaVersions {
			if version != 1 && version != 2 {
				return Manifest{}, fmt.Errorf("manifest.profile_schema_versions[%d]: must be 1 or 2", index)
			}
			if _, exists := seen[version]; exists {
				return Manifest{}, fmt.Errorf("manifest.profile_schema_versions[%d]: is duplicated", index)
			}
			seen[version] = struct{}{}
		}
		if _, includesDefault := seen[manifest.ProfileSchemaVersion]; !includesDefault {
			return Manifest{}, errors.New("manifest.profile_schema_versions: must include profile_schema_version")
		}
	}
	if !semanticVersionPattern.MatchString(manifest.SOPVersion) {
		return Manifest{}, errors.New("manifest.sop_version: must be semantic version X.Y.Z")
	}
	if strings.TrimSpace(manifest.RulesVersion) == "" {
		return Manifest{}, errors.New("manifest.rules_version: is required")
	}
	if err := validateManifestRequiredFields(data, manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (manifest Manifest) SupportsProfileSchema(version int) bool {
	if len(manifest.ProfileSchemaVersions) == 0 {
		return version == manifest.ProfileSchemaVersion
	}
	for _, supported := range manifest.ProfileSchemaVersions {
		if version == supported {
			return true
		}
	}
	return false
}

func validateManifestRequiredFields(data []byte, manifest Manifest) error {
	if len(manifest.Outputs) == 0 {
		return errors.New("manifest.outputs: requires at least 1 output")
	}
	var root struct {
		Slots      map[string]map[string]json.RawMessage `json:"slots"`
		Components map[string]map[string]json.RawMessage `json:"components"`
		Outputs    []json.RawMessage                     `json:"outputs"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse manifest.json: %w", err)
	}
	for name, slot := range root.Slots {
		for _, required := range []string{"type", "source", "required"} {
			if _, ok := slot[required]; !ok {
				return fmt.Errorf("slot %s.%s: is required", name, required)
			}
		}
	}
	for name, component := range root.Components {
		for _, required := range []string{"template", "rule_ids", "slots", "references"} {
			if _, ok := component[required]; !ok {
				return fmt.Errorf("component %s.%s: is required", name, required)
			}
		}
	}
	for index, raw := range root.Outputs {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return fmt.Errorf("manifest.outputs[%d]: %w", index, err)
		}
		var id string
		_ = json.Unmarshal(fields["id"], &id)
		for _, required := range []string{"id", "target", "when", "management", "components"} {
			if _, ok := fields[required]; !ok {
				label := id
				if label == "" {
					label = fmt.Sprintf("[%d]", index)
				}
				return fmt.Errorf("output %s.%s: is required", label, required)
			}
		}
		var components []map[string]json.RawMessage
		if err := json.Unmarshal(fields["components"], &components); err != nil {
			return fmt.Errorf("output %s.components: %w", id, err)
		}
		if len(components) == 0 {
			return fmt.Errorf("output %s.components: requires at least 1 component", id)
		}
		for componentIndex, component := range components {
			for _, required := range []string{"id", "when"} {
				if _, ok := component[required]; !ok {
					return fmt.Errorf("output %s.components[%d].%s: is required", id, componentIndex, required)
				}
			}
		}
	}
	return nil
}

func (manifest Manifest) ValidateAssets(assetRoot string) error {
	if !repositoryRelativePath(manifest.Standard.Path) {
		return errors.New("manifest.standard.path: must be release-relative")
	}
	standardData, err := os.ReadFile(filepath.Join(assetRoot, filepath.FromSlash(manifest.Standard.Path)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("manifest.standard.path: %s does not exist", manifest.Standard.Path)
		}
		return fmt.Errorf("read manifest standard: %w", err)
	}
	standardSum := sha256.Sum256(standardData)
	if !strings.EqualFold(manifest.Standard.SHA256, hex.EncodeToString(standardSum[:])) {
		return errors.New("manifest.standard.sha256: does not match STANDARD.md")
	}

	ruleIDs := make(map[string]struct{})
	for _, match := range ruleMarkerPattern.FindAllSubmatch(standardData, -1) {
		ruleID := string(match[1])
		if _, exists := ruleIDs[ruleID]; exists {
			return fmt.Errorf("STANDARD rule %s is declared more than once", ruleID)
		}
		ruleIDs[ruleID] = struct{}{}
	}

	for _, name := range sortedKeys(manifest.Slots) {
		slot := manifest.Slots[name]
		if strings.TrimSpace(slot.Type) == "" {
			return fmt.Errorf("slot %s.type: is required", name)
		}
		if strings.TrimSpace(slot.Source) == "" {
			return fmt.Errorf("slot %s.source: is required", name)
		}
		if !strings.HasPrefix(slot.Source, "/") {
			return fmt.Errorf("slot %s.source: must be a JSON pointer beginning with /", name)
		}
		switch slot.Type {
		case "string", "string_list":
		default:
			return fmt.Errorf("slot %s.type: unsupported type %q", name, slot.Type)
		}
		if strings.TrimSpace(slot.Format) == "" {
			return fmt.Errorf("slot %s.format: is required", name)
		}
		if slot.Format != "inline" && slot.Format != "lines" {
			return fmt.Errorf("slot %s.format: unsupported format %q", name, slot.Format)
		}
	}

	usedComponents := make(map[string]struct{})
	outputIDs := make(map[string]struct{}, len(manifest.Outputs))
	outputTargets := make(map[string]struct{}, len(manifest.Outputs))
	for _, output := range manifest.Outputs {
		if strings.TrimSpace(output.ID) == "" {
			return errors.New("manifest output id: is required")
		}
		if _, exists := outputIDs[output.ID]; exists {
			return fmt.Errorf("output %s: id is duplicated", output.ID)
		}
		outputIDs[output.ID] = struct{}{}
		if !repositoryRelativePath(output.Target) {
			return fmt.Errorf("output %s.target: must be repository-relative", output.ID)
		}
		for existing := range outputTargets {
			if strings.EqualFold(existing, output.Target) {
				return fmt.Errorf("output %s.target: collides with %s on macOS/Windows", output.ID, existing)
			}
		}
		outputTargets[output.Target] = struct{}{}
		if !knownCondition(output.When) {
			return fmt.Errorf("output %s.when: unknown condition %q", output.ID, output.When)
		}
		if output.ForEach != "" && output.ForEach != "ends" {
			return fmt.Errorf("output %s.for_each: unsupported value %q", output.ID, output.ForEach)
		}
		switch output.Management {
		case "block":
			if output.MarkerStyle != "" && output.MarkerStyle != "html" && output.MarkerStyle != "hash" {
				return fmt.Errorf("output %s.marker_style: unsupported value %q", output.ID, output.MarkerStyle)
			}
		case "full":
			return fmt.Errorf("output %s.management: full-file management is not supported because it cannot prove file ownership; use a managed block", output.ID)
		default:
			return fmt.Errorf("output %s.management: unsupported value %q", output.ID, output.Management)
		}
		for _, inline := range []string{output.ID, output.Target} {
			for _, match := range slotPattern.FindAllStringSubmatch(inline, -1) {
				if _, ok := manifest.Slots[string(match[1])]; !ok {
					return fmt.Errorf("output %s: uses unregistered slot %s", output.ID, match[1])
				}
			}
		}
		for _, component := range output.Components {
			if _, ok := manifest.Components[component.ID]; !ok {
				return fmt.Errorf("output %s: component %s does not exist", output.ID, component.ID)
			}
			if !knownCondition(component.When) {
				return fmt.Errorf("output %s component %s.when: unknown condition %q", output.ID, component.ID, component.When)
			}
			usedComponents[component.ID] = struct{}{}
		}
	}

	consumedRuleIDs := make(map[string]struct{}, len(ruleIDs))
	for _, name := range sortedKeys(manifest.Components) {
		component := manifest.Components[name]
		if _, ok := usedComponents[name]; !ok {
			return fmt.Errorf("component %s: is not used by any output", name)
		}
		if !repositoryRelativePath(component.Template) {
			return fmt.Errorf("component %s: template must be release-relative", name)
		}
		if len(component.RuleIDs) == 0 {
			return fmt.Errorf("component %s.rule_ids: requires at least 1 rule id", name)
		}
		templateData, err := os.ReadFile(filepath.Join(assetRoot, filepath.FromSlash(component.Template)))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("component %s: template %s does not exist", name, component.Template)
			}
			return fmt.Errorf("component %s: read template: %w", name, err)
		}
		for _, ruleID := range component.RuleIDs {
			if _, ok := ruleIDs[ruleID]; !ok {
				return fmt.Errorf("component %s: rule_id %s not found in STANDARD.md", name, ruleID)
			}
			consumedRuleIDs[ruleID] = struct{}{}
		}
		for _, reference := range component.References {
			if _, ok := outputIDs[reference]; !ok {
				return fmt.Errorf("component %s: reference %s does not match an output id", name, reference)
			}
		}
		declaredSlots := make(map[string]struct{}, len(component.Slots))
		for _, slot := range component.Slots {
			if _, ok := manifest.Slots[slot]; !ok {
				return fmt.Errorf("component %s: declares unregistered slot %s", name, slot)
			}
			declaredSlots[slot] = struct{}{}
		}
		for _, match := range slotPattern.FindAllSubmatch(templateData, -1) {
			slot := string(match[1])
			if _, ok := manifest.Slots[slot]; !ok {
				return fmt.Errorf("component %s: template uses unregistered slot %s", name, slot)
			}
			if _, ok := declaredSlots[slot]; !ok {
				return fmt.Errorf("component %s: template uses undeclared slot %s", name, slot)
			}
		}
	}
	for _, ruleID := range sortedKeys(ruleIDs) {
		if _, consumed := consumedRuleIDs[ruleID]; !consumed {
			return fmt.Errorf("STANDARD rule %s is not consumed by any component", ruleID)
		}
	}
	return nil
}

func knownCondition(condition string) bool {
	switch condition {
	case "always", "collaborators", "multiend", "parallel", "serial", "collaborators_or_parallel", "legacy", "loop":
		return true
	default:
		return false
	}
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
