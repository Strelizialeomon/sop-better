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
	Runtime        *Runtime `json:"runtime,omitempty"`
}

type Runtime struct {
	Mode                     string              `json:"mode"`
	Tracker                  string              `json:"tracker"`
	StartMode                string              `json:"start_mode"`
	AutoMerge                string              `json:"auto_merge"`
	EvidenceTrust            string              `json:"evidence_trust"`
	LeaseTimeoutSeconds      int                 `json:"lease_timeout_seconds"`
	HeartbeatIntervalSeconds int                 `json:"heartbeat_interval_seconds"`
	Trust                    RuntimeTrust        `json:"trust"`
	Checks                   map[string][]string `json:"checks"`
}

type RuntimeTrust struct {
	GitHub GitHubTrust `json:"github"`
}

type GitHubTrust struct {
	TrustedActorIDs []int64 `json:"trusted_actor_ids"`
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
	if profile.SchemaVersion != 1 && profile.SchemaVersion != 2 {
		return Profile{}, errors.New("profile.schema_version: must be 1 or 2")
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
	if profile.SchemaVersion != 1 && profile.SchemaVersion != 2 {
		return errors.New("profile.schema_version: must be 1 or 2")
	}
	if profile.SchemaVersion == 2 && profile.Runtime == nil {
		return errors.New("profile.runtime: is required for schema_version 2")
	}
	if profile.SchemaVersion == 1 && profile.Runtime != nil {
		return errors.New("profile.runtime: requires schema_version 2")
	}
	if profile.Runtime != nil {
		if err := profile.Runtime.Validate(); err != nil {
			return err
		}
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
	if profile.Runtime != nil {
		if profile.Risk != "reversible" {
			return errors.New("profile.risk: Loop MVP requires reversible risk")
		}
		if profile.ParallelAgents {
			return errors.New("profile.parallel_agents: Loop MVP does not support parallel_agents")
		}
		if len(profile.Ends) != 1 {
			return errors.New("profile.ends: Loop MVP requires exactly 1 end")
		}
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

func (runtime Runtime) Validate() error {
	if runtime.Mode != "loop-v1-experimental" {
		return errors.New("profile.runtime.mode: must be loop-v1-experimental")
	}
	if runtime.Tracker != "github" {
		return errors.New("profile.runtime.tracker: must be github")
	}
	if runtime.StartMode != "manual" {
		return errors.New("profile.runtime.start_mode: must be manual in loop-v1-experimental")
	}
	if runtime.AutoMerge != "disabled" {
		return errors.New("profile.runtime.auto_merge: must be disabled in the Loop MVP")
	}
	if runtime.EvidenceTrust != "cooperative-local" {
		return errors.New("profile.runtime.evidence_trust: must be cooperative-local")
	}
	if runtime.LeaseTimeoutSeconds <= 0 {
		return errors.New("profile.runtime.lease_timeout_seconds: must be positive")
	}
	if runtime.HeartbeatIntervalSeconds <= 0 {
		return errors.New("profile.runtime.heartbeat_interval_seconds: must be positive")
	}
	if runtime.HeartbeatIntervalSeconds > runtime.LeaseTimeoutSeconds/3 {
		return errors.New("profile.runtime.heartbeat_interval_seconds: must not exceed one third of lease_timeout_seconds")
	}
	if len(runtime.Trust.GitHub.TrustedActorIDs) == 0 {
		return errors.New("profile.runtime.trust.github.trusted_actor_ids: requires at least 1 trusted actor")
	}
	if err := validatePositiveUniqueIDs("profile.runtime.trust.github.trusted_actor_ids", runtime.Trust.GitHub.TrustedActorIDs); err != nil {
		return err
	}
	if len(runtime.Checks) == 0 {
		return errors.New("profile.runtime.checks: requires at least 1 check group")
	}
	groups := make([]string, 0, len(runtime.Checks))
	for group := range runtime.Checks {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	for _, group := range groups {
		commands := runtime.Checks[group]
		if len(commands) == 0 {
			return fmt.Errorf("profile.runtime.checks.%s: requires at least 1 command", group)
		}
		for index, command := range commands {
			if strings.TrimSpace(command) == "" || strings.ContainsAny(command, "\r\n") {
				return fmt.Errorf("profile.runtime.checks.%s[%d]: must be one non-empty line", group, index)
			}
		}
	}
	return nil
}

func validatePositiveUniqueIDs(field string, ids []int64) error {
	seen := make(map[int64]struct{}, len(ids))
	for index, id := range ids {
		if id <= 0 {
			return fmt.Errorf("%s[%d]: must be positive", field, index)
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("%s[%d]: is duplicated", field, index)
		}
		seen[id] = struct{}{}
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
