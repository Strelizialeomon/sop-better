package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var semanticVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

var (
	endNamePattern          = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	windowsInvalidCharacter = regexp.MustCompile(`[<>:"|?*]`)
	windowsPathInText       = regexp.MustCompile(`(?i)(?:^|[\s"'(=])(?:[A-Z]:[\\/]|\\\\[^\\\s]+\\[^\\\s]+)`)
	unixMachinePathInText   = regexp.MustCompile(`(?:^|[\s"'(=])/(?:Users|home|var|etc|opt|tmp|private|Volumes|mnt|srv|root)(?:/|[\s"')]|$)`)
)

type Profile struct {
	SchemaVersion  int      `json:"schema_version"`
	SOPVersion     string   `json:"sop_version"`
	Project        Project  `json:"project"`
	Ends           []End    `json:"ends"`
	Humans         []Human  `json:"humans"`
	ParallelAgents bool     `json:"parallel_agents"`
	Risk           string   `json:"risk"`
	HouseStyle     []string `json:"house_style"`
	RiskItems      []string `json:"risk_items,omitempty"`
	ProdInfraNote  string   `json:"prod_infra_note,omitempty"`
}

type Project struct {
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	DefaultBranch    string `json:"default_branch"`
	SOPInitializedOn string `json:"sop_initialized_on"`
}

type End struct {
	Name                     string   `json:"name"`
	DisplayName              string   `json:"display_name,omitempty"`
	Path                     string   `json:"path"`
	Stack                    string   `json:"stack,omitempty"`
	Docs                     []string `json:"docs,omitempty"`
	ImplementationVocabulary []string `json:"implementation_vocabulary,omitempty"`
	Milestones               []string `json:"milestones,omitempty"`
	HighRiskItems            []string `json:"high_risk_items,omitempty"`
}

type Human struct {
	ID    string   `json:"id"`
	Roles []string `json:"roles"`
}

func LoadProfile(path string) (Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Profile{}, errors.New(".sop/profile.json: file does not exist")
		}
		return Profile{}, fmt.Errorf("read .sop/profile.json: %w", err)
	}
	return ParseProfile(data)
}

func ParseProfile(data []byte) (Profile, error) {
	var profile Profile
	if err := strictDecodeJSON(data, &profile); err != nil {
		return Profile{}, fmt.Errorf("parse .sop/profile.json: %w", err)
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return Profile{}, fmt.Errorf("parse .sop/profile.json: %w", err)
	}
	if err := rejectJSONNull(raw, "profile"); err != nil {
		return Profile{}, err
	}
	if profile.SchemaVersion != 1 {
		return Profile{}, errors.New("profile.schema_version: must be 1")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return Profile{}, fmt.Errorf("parse .sop/profile.json: %w", err)
	}
	for _, required := range []string{"parallel_agents", "house_style"} {
		if _, ok := fields[required]; !ok {
			return Profile{}, fmt.Errorf("profile.%s: is required", required)
		}
	}
	if err := profile.Validate(); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func rejectJSONNull(value any, path string) error {
	if value == nil {
		return fmt.Errorf("%s: must not be null", path)
	}
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := rejectJSONNull(typed[key], path+"."+key); err != nil {
				return err
			}
		}
	case []any:
		for index, item := range typed {
			if err := rejectJSONNull(item, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (profile Profile) Validate() error {
	if profile.SchemaVersion != 1 {
		return errors.New("profile.schema_version: must be 1")
	}
	if !semanticVersionPattern.MatchString(profile.SOPVersion) {
		return errors.New("profile.sop_version: must be semantic version X.Y.Z")
	}
	if strings.TrimSpace(profile.Project.Name) == "" {
		return errors.New("profile.project.name: is required")
	}
	if strings.TrimSpace(profile.Project.DefaultBranch) == "" {
		return errors.New("profile.project.default_branch: is required")
	}
	if !validGitBranchName(profile.Project.DefaultBranch) {
		return errors.New("profile.project.default_branch: must be a valid Git branch name")
	}
	if profile.Project.SOPInitializedOn == "" {
		return errors.New("profile.project.sop_initialized_on: is required")
	}
	if _, err := time.Parse("2006-01-02", profile.Project.SOPInitializedOn); err != nil {
		return errors.New("profile.project.sop_initialized_on: must use YYYY-MM-DD")
	}
	for _, value := range []struct {
		field string
		text  string
	}{
		{"profile.project.name", profile.Project.Name},
		{"profile.project.description", profile.Project.Description},
		{"profile.project.default_branch", profile.Project.DefaultBranch},
		{"profile.prod_infra_note", profile.ProdInfraNote},
	} {
		if err := validatePortableProfileText(value.field, value.text); err != nil {
			return err
		}
	}
	for index, value := range profile.HouseStyle {
		if err := validatePortableProfileText(fmt.Sprintf("profile.house_style[%d]", index), value); err != nil {
			return err
		}
	}
	for index, value := range profile.RiskItems {
		if err := validatePortableProfileText(fmt.Sprintf("profile.risk_items[%d]", index), value); err != nil {
			return err
		}
	}

	switch profile.Risk {
	case "reversible", "controlled", "high":
	default:
		return errors.New("profile.risk: must be one of reversible, controlled, high")
	}
	if profile.ParallelAgents && len(profile.Ends) < 2 {
		return errors.New("profile.parallel_agents: requires at least 2 ends")
	}
	if len(profile.Ends) == 0 {
		return errors.New("profile.ends: requires at least 1 end")
	}
	endNames := make(map[string]struct{}, len(profile.Ends))
	endPaths := make(map[string]struct{}, len(profile.Ends))
	for i, end := range profile.Ends {
		if strings.TrimSpace(end.Name) == "" {
			return fmt.Errorf("profile.ends[%d].name: is required", i)
		}
		if !endNamePattern.MatchString(end.Name) {
			return fmt.Errorf("profile.ends[%d].name: must use letters, numbers, dot, underscore, or hyphen", i)
		}
		for _, value := range []struct {
			field string
			text  string
		}{
			{fmt.Sprintf("profile.ends[%d].display_name", i), end.DisplayName},
			{fmt.Sprintf("profile.ends[%d].stack", i), end.Stack},
		} {
			if err := validatePortableProfileText(value.field, value.text); err != nil {
				return err
			}
		}
		for _, group := range []struct {
			field  string
			values []string
		}{
			{"implementation_vocabulary", end.ImplementationVocabulary},
			{"milestones", end.Milestones},
			{"high_risk_items", end.HighRiskItems},
		} {
			for valueIndex, value := range group.values {
				if err := validatePortableProfileText(fmt.Sprintf("profile.ends[%d].%s[%d]", i, group.field, valueIndex), value); err != nil {
					return err
				}
			}
		}
		if strings.Contains(end.Path, `\`) && !windowsAbsolutePath(end.Path) {
			return fmt.Errorf("profile.ends[%d].path: must use portable forward slashes", i)
		}
		if filepath.IsAbs(end.Path) || windowsAbsolutePath(end.Path) || strings.HasPrefix(end.Path, `\`) {
			return fmt.Errorf("profile.ends[%d].path: must be repository-relative", i)
		}
		for _, segment := range strings.Split(end.Path, "/") {
			if windowsReservedSegment(segment) {
				return fmt.Errorf("profile.ends[%d].path: contains Windows-reserved segment %s", i, segment)
			}
		}
		if !repositoryRelativePath(end.Path) {
			return fmt.Errorf("profile.ends[%d].path: must be repository-relative", i)
		}
		for existing := range endNames {
			if strings.EqualFold(existing, end.Name) {
				return fmt.Errorf("profile.ends[%d].name: collides on macOS/Windows", i)
			}
		}
		endNames[end.Name] = struct{}{}
		normalizedPath := pathpkg.Clean(end.Path)
		for existing := range endPaths {
			if strings.EqualFold(existing, normalizedPath) {
				return fmt.Errorf("profile.ends[%d].path: collides on macOS/Windows", i)
			}
		}
		endPaths[normalizedPath] = struct{}{}
		for documentIndex, document := range end.Docs {
			if !repositoryRelativePath(document) {
				return fmt.Errorf("profile.ends[%d].docs[%d]: must be repository-relative", i, documentIndex)
			}
		}
	}
	if len(profile.Humans) == 0 {
		return errors.New("profile.humans: requires at least 1 human")
	}
	humanIDs := make(map[string]struct{}, len(profile.Humans))
	for i, human := range profile.Humans {
		if strings.TrimSpace(human.ID) == "" {
			return fmt.Errorf("profile.humans[%d].id: is required", i)
		}
		if err := validatePortableProfileText(fmt.Sprintf("profile.humans[%d].id", i), human.ID); err != nil {
			return err
		}
		if _, exists := humanIDs[human.ID]; exists {
			return fmt.Errorf("profile.humans[%d].id: is duplicated", i)
		}
		humanIDs[human.ID] = struct{}{}
		if len(human.Roles) == 0 {
			return fmt.Errorf("profile.humans[%d].roles: requires at least 1 role", i)
		}
		roles := make(map[string]struct{}, len(human.Roles))
		for roleIndex, role := range human.Roles {
			if err := validatePortableProfileText(fmt.Sprintf("profile.humans[%d].roles[%d]", i, roleIndex), role); err != nil {
				return err
			}
			if _, exists := roles[role]; exists {
				return fmt.Errorf("profile.humans[%d].roles[%d]: is duplicated", i, roleIndex)
			}
			roles[role] = struct{}{}
		}
	}
	return nil
}

func validGitBranchName(name string) bool {
	if name == "@" || strings.HasPrefix(name, "-") || strings.HasPrefix(name, "refs/") ||
		strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") ||
		strings.Contains(name, "//") || strings.Contains(name, "..") || strings.Contains(name, "@{") {
		return false
	}
	for _, character := range name {
		if character <= ' ' || character == 0x7f || strings.ContainsRune("~^:?*[\\", character) {
			return false
		}
	}
	for _, component := range strings.Split(name, "/") {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return true
}

func validatePortableProfileText(field, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if filepath.IsAbs(trimmed) || windowsAbsolutePath(trimmed) || strings.HasPrefix(trimmed, `\`) ||
		windowsPathInText.MatchString(value) || unixMachinePathInText.MatchString(value) {
		return fmt.Errorf("%s: must not contain a machine absolute path; use a repository-relative reference, repository identity, or URL", field)
	}
	return nil
}

func repositoryRelativePath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, `\`) || filepath.IsAbs(path) || strings.HasPrefix(path, `\`) {
		return false
	}
	if windowsAbsolutePath(path) {
		return false
	}
	cleaned := pathpkg.Clean(path)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned != path {
		return false
	}
	for _, segment := range strings.Split(cleaned, "/") {
		if segment == "" || segment == "." || strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") ||
			windowsInvalidCharacter.MatchString(segment) || windowsReservedSegment(segment) {
			return false
		}
	}
	return true
}

func windowsAbsolutePath(value string) bool {
	return len(value) >= 3 && value[1] == ':' && (value[2] == '\\' || value[2] == '/')
}

func windowsReservedSegment(segment string) bool {
	name := strings.ToUpper(strings.TrimSpace(segment))
	if dot := strings.IndexByte(name, '.'); dot >= 0 {
		name = name[:dot]
	}
	switch name {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(name) == 4 && (strings.HasPrefix(name, "COM") || strings.HasPrefix(name, "LPT")) && name[3] >= '1' && name[3] <= '9' {
		return true
	}
	return false
}
