package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/Strelizialeomon/sop-better/internal/config"
	"github.com/Strelizialeomon/sop-better/internal/projectid"
)

var (
	originReferencePattern = regexp.MustCompile(`\borigin/([A-Za-z0-9_-]+(?:[./][A-Za-z0-9_-]+)*)`)
	machinePathPattern     = regexp.MustCompile(`(?i)(?:[A-Z]:[\\/]|\\\\[^\\\s]+\\[^\\\s]+(?:\\|$)|/(?:Users|home)/[^/\s]+/)`)
	markdownLinkPattern    = regexp.MustCompile(`!?\[[^\]]*\]\(([^)\s]+)(?:\s+[^)]*)?\)`)
	parallelScopePattern   = regexp.MustCompile(`(?im)^\s*scope\s*:`)
)

func CheckProject(projectRoot string, profile config.Profile, expected Lock) error {
	return withProjectOperationLock(projectRoot, func() error {
		if err := ensureProjectIdle(projectRoot); err != nil {
			return err
		}
		return checkProjectState(projectRoot, profile, expected)
	})
}

// CheckProjectUnderHeldLock is the read-only check path used by the release
// manager after it has acquired this project's native operation lock. The
// physical project identity prevents a caller from claiming a different lock.
func CheckProjectUnderHeldLock(projectRoot, expectedProjectID string, profile config.Profile, expected Lock) error {
	actualProjectID, err := projectid.Identifier(projectRoot)
	if err != nil {
		return err
	}
	if actualProjectID != expectedProjectID {
		return errors.New("manager-held project lock identity does not match project root")
	}
	if err := ensureProjectIdle(projectRoot); err != nil {
		return err
	}
	return checkProjectState(projectRoot, profile, expected)
}

func checkProjectState(projectRoot string, profile config.Profile, expected Lock) error {
	if err := rejectDeprecatedRuntimeResidues(projectRoot, profile); err != nil {
		return err
	}
	if profile.SOPVersion != expected.SOPVersion {
		return fmt.Errorf("profile.sop_version %s does not match lock SOP version %s", profile.SOPVersion, expected.SOPVersion)
	}
	lockPath := filepath.Join(projectRoot, ".sop", "lock.json")
	lockData, err := os.ReadFile(lockPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New(".sop/lock.json: file does not exist")
		}
		return fmt.Errorf("read .sop/lock.json: %w", err)
	}

	lock, err := decodeLock(lockData)
	if err != nil {
		return fmt.Errorf("parse .sop/lock.json: %w", err)
	}
	if err := reconcileLock(lock, expected); err != nil {
		return err
	}
	seenIDs := make(map[string]struct{}, len(lock.Outputs))
	seenTargets := make(map[string]string, len(lock.Outputs))
	for _, output := range lock.Outputs {
		if _, exists := seenIDs[output.ID]; exists {
			return fmt.Errorf("lock output id %s is duplicated", output.ID)
		}
		seenIDs[output.ID] = struct{}{}
		if previousID, exists := seenTargets[output.Target]; exists {
			return fmt.Errorf("lock outputs %s and %s share target %s", previousID, output.ID, output.Target)
		}
		seenTargets[output.Target] = output.ID
		if !safeProjectTarget(output.Target) {
			return fmt.Errorf("lock output %s: target must be repository-relative", output.ID)
		}
		if err := rejectManagedSymlinkTraversal(projectRoot, output.Target); err != nil {
			return err
		}
		path := filepath.Join(projectRoot, filepath.FromSlash(output.Target))
		content, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("%s: managed output does not exist", output.Target)
			}
			return fmt.Errorf("read %s: %w", output.Target, err)
		}
		body := string(content)
		if output.Management == "block" {
			pattern, validStyle := managedBlockPattern(output.ID, output.MarkerStyle)
			if !validStyle {
				return fmt.Errorf("%s: managed block %s uses unknown marker style %q", output.Target, output.ID, output.MarkerStyle)
			}
			count := len(pattern.FindAllIndex(content, -1))
			if count != 1 {
				return fmt.Errorf("%s: managed block %s appears %d times", output.Target, output.ID, count)
			}
			managedBody, markerHash, ok := managedBlock(content, output.ID, output.MarkerStyle)
			if !ok {
				return fmt.Errorf("%s: managed block %s is missing or malformed", output.Target, output.ID)
			}
			computedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalText(managedBody))))
			if markerHash != output.Hash || computedHash != output.Hash {
				return fmt.Errorf("%s: managed block %s hash mismatch", output.Target, output.ID)
			}
			markerVersion, ok := managedMarkerVersion(content, output.ID, output.MarkerStyle)
			if !ok || markerVersion != lock.SOPVersion {
				return fmt.Errorf("%s: managed block %s marker version %s, expected %s", output.Target, output.ID, markerVersion, lock.SOPVersion)
			}
			body = managedBody
		} else if output.Management == "full" {
			return fmt.Errorf("%s: full-file management for output %s is unsupported because ownership cannot be proven", output.Target, output.ID)
		} else {
			return fmt.Errorf("%s: unknown management %q", output.Target, output.Management)
		}
		if err := checkGeneratedText(projectRoot, output, body, profile, nil); err != nil {
			return err
		}
	}
	return nil
}

func decodeLock(data []byte) (Lock, error) {
	var lock Lock
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&lock); err != nil {
		return Lock{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		for _, output := range lock.Outputs {
			if output.Management != "block" {
				return Lock{}, fmt.Errorf("lock output %s at %s uses unsupported management %q; only self-authenticating managed blocks are allowed", output.ID, output.Target, output.Management)
			}
		}
		return lock, nil
	} else if err != nil {
		return Lock{}, err
	}
	return Lock{}, errors.New("unexpected trailing JSON value")
}

func reconcileLock(actual Lock, expected Lock) error {
	if actual.SchemaVersion != expected.SchemaVersion {
		return fmt.Errorf("lock.schema_version %d, expected %d", actual.SchemaVersion, expected.SchemaVersion)
	}
	if actual.SOPVersion != expected.SOPVersion {
		return fmt.Errorf("lock.sop_version %s, expected %s", actual.SOPVersion, expected.SOPVersion)
	}
	if actual.GeneratorVersion != expected.GeneratorVersion {
		return fmt.Errorf("lock.generator_version %s, expected %s", actual.GeneratorVersion, expected.GeneratorVersion)
	}
	if actual.RulesVersion != expected.RulesVersion {
		return fmt.Errorf("lock.rules_version %s, expected %s", actual.RulesVersion, expected.RulesVersion)
	}
	if actual.ProfileHash != expected.ProfileHash {
		return errors.New("lock.profile_hash does not match the current profile")
	}
	if len(actual.Outputs) != len(expected.Outputs) {
		return fmt.Errorf("lock outputs: got %d, expected %d", len(actual.Outputs), len(expected.Outputs))
	}
	actualByID := make(map[string]LockOutput, len(actual.Outputs))
	for _, output := range actual.Outputs {
		if _, exists := actualByID[output.ID]; exists {
			return fmt.Errorf("lock output id %s is duplicated", output.ID)
		}
		actualByID[output.ID] = output
	}
	for _, wanted := range expected.Outputs {
		got, ok := actualByID[wanted.ID]
		if !ok {
			return fmt.Errorf("lock output %s is missing", wanted.ID)
		}
		if got.Target != wanted.Target || got.Management != wanted.Management || got.MarkerStyle != wanted.MarkerStyle {
			return fmt.Errorf("lock output %s metadata does not match current profile and manifest", wanted.ID)
		}
		if !slices.Equal(got.Components, wanted.Components) {
			return fmt.Errorf("lock output %s components do not match current profile and manifest", wanted.ID)
		}
		if got.Hash != wanted.Hash {
			return fmt.Errorf("lock output %s hash does not match current profile and manifest", wanted.ID)
		}
	}
	return nil
}

func checkGeneratedText(projectRoot string, output LockOutput, body string, profile config.Profile, virtualTargets map[string]bool) error {
	lower := strings.ToLower(body)
	switch {
	case strings.Contains(body, "{{"):
		return fmt.Errorf("%s: unresolved template placeholder", output.Target)
	case strings.Contains(body, "无则删"):
		return fmt.Errorf("%s: contains template authoring note", output.Target)
	case machinePathPattern.MatchString(body):
		return fmt.Errorf("%s: contains machine-specific absolute path", output.Target)
	case strings.Contains(body, "STANDARD.md"):
		return fmt.Errorf("%s: generated project must not reference STANDARD.md", output.Target)
	case strings.Contains(lower, "claude.md") || strings.Contains(lower, ".claude"):
		return fmt.Errorf("%s: generated project contains deprecated Claude runtime reference", output.Target)
	case strings.Contains(body, "coordination.md"):
		return fmt.Errorf("%s: generated project must not reference coordination.md", output.Target)
	case strings.Contains(body, "无需 PR") || strings.Contains(body, "不开分支") || strings.Contains(body, "直推") || strings.Contains(body, "直接推主分支"):
		return fmt.Errorf("%s: contains obsolete direct-push rule", output.Target)
	}

	for _, match := range originReferencePattern.FindAllStringSubmatch(body, -1) {
		branch := match[1]
		if branch != profile.Project.DefaultBranch && branch != "HEAD" {
			return fmt.Errorf("%s: uses origin/%s instead of origin/%s", output.Target, branch, profile.Project.DefaultBranch)
		}
	}
	if strings.HasPrefix(output.ID, "end-agents-") && !profile.ParallelAgents {
		switch {
		case parallelScopePattern.MatchString(body):
			return fmt.Errorf("%s: serial end guidance contains parallel-only scope", output.Target)
		case strings.Contains(lower, "worktree"):
			return fmt.Errorf("%s: serial end guidance contains parallel-only worktree", output.Target)
		case strings.Contains(lower, "coordination"):
			return fmt.Errorf("%s: serial end guidance contains parallel-only coordination", output.Target)
		}
	}
	return checkMarkdownLinks(projectRoot, output.Target, body, virtualTargets)
}

func checkMarkdownLinks(projectRoot string, sourceTarget string, body string, virtualTargets map[string]bool) error {
	for _, match := range markdownLinkPattern.FindAllStringSubmatch(body, -1) {
		raw := strings.Trim(match[1], "<>")
		if raw == "" || strings.HasPrefix(raw, "#") || strings.Contains(raw, "://") ||
			strings.HasPrefix(raw, "mailto:") || strings.HasPrefix(raw, "tel:") {
			continue
		}
		withoutFragment, _, _ := strings.Cut(raw, "#")
		withoutQuery, _, _ := strings.Cut(withoutFragment, "?")
		decoded, err := url.PathUnescape(withoutQuery)
		if err != nil {
			return fmt.Errorf("%s: link %s has invalid escaping", sourceTarget, raw)
		}
		if decoded == "" {
			continue
		}
		resolved := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(sourceTarget), filepath.FromSlash(decoded))))
		if !safeProjectTarget(resolved) {
			return fmt.Errorf("%s: link %s escapes the repository", sourceTarget, raw)
		}
		if exists, known := virtualTargets[resolved]; known {
			if exists {
				continue
			}
			return fmt.Errorf("%s: link %s does not exist", sourceTarget, raw)
		}
		if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(resolved))); errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s: link %s does not exist", sourceTarget, raw)
		} else if err != nil {
			return fmt.Errorf("%s: inspect link %s: %w", sourceTarget, raw, err)
		}
	}
	return nil
}

func ValidateCandidateOutputs(projectRoot string, outputs []RenderedOutput, profile config.Profile) error {
	virtualTargets := make(map[string]bool, len(outputs))
	activeIDs := make(map[string]struct{}, len(outputs))
	for _, output := range outputs {
		if _, exists := activeIDs[output.Lock.ID]; exists {
			return fmt.Errorf("candidate lock output id %s is duplicated", output.Lock.ID)
		}
		activeIDs[output.Lock.ID] = struct{}{}
		if _, exists := virtualTargets[output.Lock.Target]; exists {
			return fmt.Errorf("candidate target %s is duplicated", output.Lock.Target)
		}
		virtualTargets[output.Lock.Target] = true
	}
	previousLock, exists, err := readLock(projectRoot)
	if err != nil {
		return err
	}
	if exists {
		for _, previous := range previousLock.Outputs {
			if _, active := activeIDs[previous.ID]; !active {
				virtualTargets[previous.Target] = false
			}
		}
	}

	for _, output := range outputs {
		if err := rejectManagedSymlinkTraversal(projectRoot, output.Lock.Target); err != nil {
			return err
		}
		body := string(output.Content)
		switch output.Lock.Management {
		case "block":
			pattern, ok := managedBlockPattern(output.Lock.ID, output.Lock.MarkerStyle)
			if !ok || len(pattern.FindAllIndex(output.Content, -1)) != 1 {
				return fmt.Errorf("candidate %s: managed block %s is malformed", output.Lock.Target, output.Lock.ID)
			}
			managedBody, markerHash, ok := managedBlock(output.Content, output.Lock.ID, output.Lock.MarkerStyle)
			if !ok {
				return fmt.Errorf("candidate %s: managed block %s is malformed", output.Lock.Target, output.Lock.ID)
			}
			computed := fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalText(managedBody))))
			if markerHash != output.Lock.Hash || computed != output.Lock.Hash {
				return fmt.Errorf("candidate %s: managed block %s hash mismatch", output.Lock.Target, output.Lock.ID)
			}
			markerVersion, ok := managedMarkerVersion(output.Content, output.Lock.ID, output.Lock.MarkerStyle)
			if !ok || markerVersion != profile.SOPVersion {
				return fmt.Errorf("candidate %s: managed block %s marker version %s, expected %s", output.Lock.Target, output.Lock.ID, markerVersion, profile.SOPVersion)
			}
			body = managedBody
		case "full":
			computed := fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalText(body))))
			if computed != output.Lock.Hash {
				return fmt.Errorf("candidate %s: managed file hash mismatch", output.Lock.Target)
			}
		default:
			return fmt.Errorf("candidate %s: unknown management %q", output.Lock.Target, output.Lock.Management)
		}
		if err := checkGeneratedText(projectRoot, output.Lock, body, profile, virtualTargets); err != nil {
			return fmt.Errorf("candidate %w", err)
		}
	}
	return nil
}

func managedBlock(content []byte, id string, markerStyle string) (string, string, bool) {
	pattern, ok := managedBlockPattern(id, markerStyle)
	if !ok {
		return "", "", false
	}
	match := pattern.FindSubmatch(content)
	if len(match) != 3 {
		return "", "", false
	}
	return string(match[2]), string(match[1]), true
}

func managedMarkerVersion(content []byte, id string, markerStyle string) (string, bool) {
	if markerStyle == "" {
		markerStyle = "html"
	}
	var expression string
	switch markerStyle {
	case "html":
		expression = `<!-- sop-better:begin id=` + regexp.QuoteMeta(id) + ` version=([^ \r\n]+) hash=[0-9a-f]{64} -->`
	case "hash":
		expression = `# sop-better:begin id=` + regexp.QuoteMeta(id) + ` version=([^ \r\n]+) hash=[0-9a-f]{64}`
	default:
		return "", false
	}
	match := regexp.MustCompile(expression).FindSubmatch(content)
	if len(match) != 2 {
		return "", false
	}
	return string(match[1]), true
}

func managedBlockPattern(id string, markerStyle string) (*regexp.Regexp, bool) {
	if markerStyle == "" {
		markerStyle = "html"
	}
	var expression string
	switch markerStyle {
	case "html":
		expression = `(?s)<!-- sop-better:begin id=` + regexp.QuoteMeta(id) +
			` version=[^ ]+ hash=([0-9a-f]{64}) -->\r?\n(.*?)<!-- sop-better:end id=` + regexp.QuoteMeta(id) + ` -->(?:\r?\n|$)`
	case "hash":
		expression = `(?s)# sop-better:begin id=` + regexp.QuoteMeta(id) +
			` version=[^ ]+ hash=([0-9a-f]{64})\r?\n(.*?)# sop-better:end id=` + regexp.QuoteMeta(id) + `(?:\r?\n|$)`
	default:
		return nil, false
	}
	return regexp.MustCompile(expression), true
}

func safeProjectTarget(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) || strings.HasPrefix(path, `\`) {
		return false
	}
	if len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return false
	}
	normalized := filepath.ToSlash(filepath.Clean(strings.ReplaceAll(path, `\`, "/")))
	return normalized != ".." && !strings.HasPrefix(normalized, "../")
}
