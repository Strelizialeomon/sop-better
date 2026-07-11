package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"unicode"

	"github.com/Strelizialeomon/sop-better/internal/config"
)

type Lock struct {
	SchemaVersion    int          `json:"schema_version"`
	SOPVersion       string       `json:"sop_version"`
	GeneratorVersion string       `json:"generator_version"`
	RulesVersion     string       `json:"rules_version"`
	ProfileHash      string       `json:"profile_hash"`
	Outputs          []LockOutput `json:"outputs"`
}

type LockOutput struct {
	ID          string   `json:"id"`
	Target      string   `json:"target"`
	Management  string   `json:"management"`
	MarkerStyle string   `json:"marker_style,omitempty"`
	Components  []string `json:"components"`
	Hash        string   `json:"hash"`
}

type RenderedOutput struct {
	Target  string
	Content []byte
	Lock    LockOutput
}

func Render(profile config.Profile, manifest config.Manifest, assetRoot string, generatorVersion string) ([]RenderedOutput, Lock, error) {
	profileHash, err := profileDigest(profile)
	if err != nil {
		return nil, Lock{}, err
	}
	profileValues, err := profileMap(profile)
	if err != nil {
		return nil, Lock{}, err
	}

	outputs := make([]RenderedOutput, 0, len(manifest.Outputs))
	for _, outputSpec := range manifest.Outputs {
		active, err := conditionMatches(outputSpec.When, profile)
		if err != nil {
			return nil, Lock{}, fmt.Errorf("output %s: %w", outputSpec.ID, err)
		}
		if !active {
			continue
		}

		contexts, err := outputContexts(outputSpec, profile, profileValues)
		if err != nil {
			return nil, Lock{}, fmt.Errorf("output %s: %w", outputSpec.ID, err)
		}
		for _, values := range contexts {
			output, err := renderOutput(outputSpec, profile, manifest, values, assetRoot)
			if err != nil {
				return nil, Lock{}, err
			}
			outputs = append(outputs, output)
		}
	}

	lock := Lock{
		SchemaVersion:    1,
		SOPVersion:       manifest.SOPVersion,
		GeneratorVersion: generatorVersion,
		RulesVersion:     manifest.RulesVersion,
		ProfileHash:      profileHash,
		Outputs:          make([]LockOutput, 0, len(outputs)),
	}
	for _, output := range outputs {
		lock.Outputs = append(lock.Outputs, output.Lock)
	}
	return outputs, lock, nil
}

func profileDigest(profile config.Profile) (string, error) {
	data, err := json.Marshal(profile)
	if err != nil {
		return "", fmt.Errorf("encode profile for lock: %w", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func encodeLock(lock Lock) ([]byte, error) {
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode .sop/lock.json: %w", err)
	}
	return append(data, '\n'), nil
}

func outputContexts(output config.OutputSpec, profile config.Profile, base map[string]any) ([]map[string]any, error) {
	switch output.ForEach {
	case "":
		return []map[string]any{base}, nil
	case "ends":
		contexts := make([]map[string]any, 0, len(profile.Ends))
		for _, end := range profile.Ends {
			endData, err := json.Marshal(end)
			if err != nil {
				return nil, fmt.Errorf("encode end %s: %w", end.Name, err)
			}
			var endValues map[string]any
			if err := json.Unmarshal(endData, &endValues); err != nil {
				return nil, fmt.Errorf("decode end %s: %w", end.Name, err)
			}
			values := make(map[string]any, len(base)+1)
			for key, value := range base {
				values[key] = value
			}
			values["current_end"] = endValues
			contexts = append(contexts, values)
		}
		return contexts, nil
	default:
		return nil, fmt.Errorf("unknown for_each %q", output.ForEach)
	}
}

func renderOutput(output config.OutputSpec, profile config.Profile, manifest config.Manifest, values map[string]any, assetRoot string) (RenderedOutput, error) {
	if output.Management != "block" {
		return RenderedOutput{}, fmt.Errorf("output %s: full-file management is not supported; use a self-authenticating managed block", output.ID)
	}
	id, err := renderInline(output.ID, manifest.Slots, values)
	if err != nil {
		return RenderedOutput{}, fmt.Errorf("output %s id: %w", output.ID, err)
	}
	target, err := renderInline(output.Target, manifest.Slots, values)
	if err != nil {
		return RenderedOutput{}, fmt.Errorf("output %s target: %w", output.ID, err)
	}
	if !safeProjectTarget(target) {
		return RenderedOutput{}, fmt.Errorf("output %s: rendered target must be repository-relative", id)
	}

	var parts []string
	var componentIDs []string
	for _, reference := range output.Components {
		include, err := conditionMatches(reference.When, profile)
		if err != nil {
			return RenderedOutput{}, fmt.Errorf("output %s component %s: %w", id, reference.ID, err)
		}
		if !include {
			continue
		}
		component := manifest.Components[reference.ID]
		rendered, err := renderComponent(component, manifest.Slots, values, assetRoot)
		if err != nil {
			return RenderedOutput{}, fmt.Errorf("component %s: %w", reference.ID, err)
		}
		parts = append(parts, rendered)
		componentIDs = append(componentIDs, reference.ID)
	}

	body := canonicalText(strings.Join(parts, "\n"))
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(body)))
	content := body
	markerStyle := output.MarkerStyle
	if markerStyle == "" {
		markerStyle = "html"
	}
	if output.Management == "block" {
		content, err = wrapManagedBlock(id, manifest.SOPVersion, hash, markerStyle, body)
		if err != nil {
			return RenderedOutput{}, err
		}
	}
	return RenderedOutput{
		Target:  filepath.FromSlash(target),
		Content: []byte(content),
		Lock: LockOutput{
			ID:          id,
			Target:      filepath.ToSlash(target),
			Management:  output.Management,
			MarkerStyle: markerStyle,
			Components:  componentIDs,
			Hash:        hash,
		},
	}, nil
}

func wrapManagedBlock(id string, version string, hash string, markerStyle string, body string) (string, error) {
	switch markerStyle {
	case "html":
		return fmt.Sprintf(
			"<!-- sop-better:begin id=%s version=%s hash=%s -->\n%s<!-- sop-better:end id=%s -->\n",
			id, version, hash, body, id,
		), nil
	case "hash":
		return fmt.Sprintf(
			"# sop-better:begin id=%s version=%s hash=%s\n%s# sop-better:end id=%s\n",
			id, version, hash, body, id,
		), nil
	default:
		return "", fmt.Errorf("output %s: unknown marker_style %q", id, markerStyle)
	}
}

var inlineSlotPattern = regexp.MustCompile(`\{\{([a-zA-Z0-9_]+)\}\}`)

func renderInline(template string, slots map[string]config.SlotSpec, values map[string]any) (string, error) {
	rendered := template
	for _, match := range inlineSlotPattern.FindAllStringSubmatch(template, -1) {
		name := match[1]
		slot, ok := slots[name]
		if !ok {
			return "", fmt.Errorf("slot %s is not registered", name)
		}
		value, err := jsonPointer(values, slot.Source)
		if err != nil {
			return "", fmt.Errorf("slot %s: %w", name, err)
		}
		text, err := formatSlot(value, slot)
		if err != nil {
			return "", fmt.Errorf("slot %s: %w", name, err)
		}
		rendered = strings.ReplaceAll(rendered, match[0], text)
	}
	return rendered, nil
}

func Write(projectRoot string, outputs []RenderedOutput, lock Lock, profile config.Profile, profileData []byte) error {
	return withProjectOperationLock(projectRoot, func() error {
		return writeLocked(projectRoot, outputs, lock, profile, profileData)
	})
}

func writeLocked(projectRoot string, outputs []RenderedOutput, lock Lock, profile config.Profile, profileData []byte) error {
	if err := recoverTransaction(projectRoot); err != nil {
		return fmt.Errorf("recover interrupted transaction before render: %w", err)
	}
	if err := rejectDeprecatedRuntimeResidues(projectRoot, profile); err != nil {
		return err
	}
	if err := ValidateCandidateOutputs(projectRoot, outputs, profile); err != nil {
		return err
	}
	if len(profileData) == 0 {
		return errors.New("candidate profile data is empty")
	}
	lockData, err := encodeLock(lock)
	if err != nil {
		return err
	}
	currentLockData, readLockErr := os.ReadFile(filepath.Join(projectRoot, ".sop", "lock.json"))
	if readLockErr != nil && !errors.Is(readLockErr, os.ErrNotExist) {
		return fmt.Errorf("read .sop/lock.json: %w", readLockErr)
	}
	trustedPreviousLock, err := isTrustedManagedLock(projectRoot, currentLockData)
	if err != nil {
		return err
	}
	currentProfileData, readProfileErr := os.ReadFile(filepath.Join(projectRoot, ".sop", "profile.json"))
	if readProfileErr != nil && !errors.Is(readProfileErr, os.ErrNotExist) {
		return fmt.Errorf("read .sop/profile.json: %w", readProfileErr)
	}
	if len(currentLockData) > 0 && !trustedPreviousLock {
		if bytes.Equal(currentLockData, lockData) && bytes.Equal(currentProfileData, profileData) {
			if err := checkProjectState(projectRoot, profile, lock); err != nil {
				return fmt.Errorf("current project cannot establish this machine's trusted managed lock: %w", err)
			}
			return persistTrustedManagedLock(projectRoot, lockData)
		}
		return errors.New("current lock is not backed by this machine's trusted managed lock; project was not changed. Run render once with the unchanged current profile and matching release to establish trust")
	}
	changes, err := prepareProjectChanges(projectRoot, outputs, trustedPreviousLock)
	if err != nil {
		return err
	}
	if len(changes) == 0 && bytes.Equal(currentLockData, lockData) && bytes.Equal(currentProfileData, profileData) {
		return persistTrustedManagedLock(projectRoot, lockData)
	}
	checkpointPath, err := createCheckpoint(projectRoot, outputs)
	if err != nil {
		return fmt.Errorf("create project checkpoint: %w", err)
	}
	files := make([]transactionFile, 0, len(changes)+2)
	if !bytes.Equal(currentProfileData, profileData) {
		files = append(files, transactionFile{Target: ".sop/profile.json", Content: profileData, Mode: 0o644})
	}
	for _, change := range changes {
		files = append(files, transactionFile{Target: change.Target, Content: change.Content, Mode: 0o644, Remove: change.Remove})
	}
	files = append(files, transactionFile{Target: ".sop/lock.json", Content: lockData, Mode: 0o644})
	if err := applyTransactionLocked(projectRoot, files, transactionOptions{BeforeCommit: func() error {
		writtenProfile, err := config.LoadProfile(filepath.Join(projectRoot, ".sop", "profile.json"))
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(writtenProfile, profile) {
			return errors.New("written profile does not match the render candidate")
		}
		return checkProjectState(projectRoot, profile, lock)
	}}); err != nil {
		discardCheckpoint(checkpointPath)
		return err
	}
	if checkpointPath != "" {
		if err := pruneCheckpoints(filepath.Dir(checkpointPath), 2); err != nil {
			return fmt.Errorf("project render committed, but checkpoint cleanup failed: %w", err)
		}
	}
	if err := persistTrustedManagedLock(projectRoot, lockData); err != nil {
		return fmt.Errorf("project render committed, but managed lock provenance was not recorded: %w", err)
	}
	return nil
}

type projectChange struct {
	Target  string
	Content []byte
	Remove  bool
}

func prepareProjectChanges(projectRoot string, outputs []RenderedOutput, trustedPreviousLock bool) ([]projectChange, error) {
	previousLock, hasPreviousLock, err := readLock(projectRoot)
	if err != nil {
		return nil, err
	}
	previousByID := make(map[string]LockOutput, len(previousLock.Outputs))
	for _, output := range previousLock.Outputs {
		previousByID[output.ID] = output
	}

	changes := make([]projectChange, 0, len(outputs)+len(previousLock.Outputs))
	activeIDs := make(map[string]struct{}, len(outputs))
	for _, output := range outputs {
		activeIDs[output.Lock.ID] = struct{}{}
		if previous, tracked := previousByID[output.Lock.ID]; tracked && previous.Target != output.Lock.Target {
			return nil, fmt.Errorf(
				"managed output %s cannot move from %s to %s; retire the old output id and introduce a new output id so cleanup and rollback stay explicit",
				output.Lock.ID, previous.Target, output.Lock.Target,
			)
		}
		if err := rejectManagedSymlinkTraversal(projectRoot, output.Lock.Target); err != nil {
			return nil, err
		}
		path := filepath.Join(projectRoot, output.Target)
		current, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			changes = append(changes, projectChange{Target: output.Lock.Target, Content: output.Content})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", output.Lock.Target, err)
		}

		previous, tracked := previousByID[output.Lock.ID]
		switch output.Lock.Management {
		case "block":
			body, markerHash, ok := managedBlock(current, output.Lock.ID, output.Lock.MarkerStyle)
			if !ok {
				return nil, fmt.Errorf("%s: exists without managed block %s", output.Lock.Target, output.Lock.ID)
			}
			if !hasPreviousLock || !tracked || previous.Target != output.Lock.Target {
				return nil, fmt.Errorf("%s: managed block %s is not tracked by .sop/lock.json", output.Lock.Target, output.Lock.ID)
			}
			computedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalText(body))))
			if markerHash != previous.Hash || computedHash != previous.Hash {
				return nil, fmt.Errorf(
					"%s: managed block %s was modified locally\nLOCAL/CANDIDATE %s\n--- local/%s\n+++ candidate/%s\n%s",
					output.Lock.Target, output.Lock.ID, output.Lock.Target, output.Lock.Target, output.Lock.Target,
					lineDiff(current, output.Content),
				)
			}
			if markerVersion, ok := managedMarkerVersion(current, output.Lock.ID, output.Lock.MarkerStyle); !ok || markerVersion != previousLock.SOPVersion {
				return nil, fmt.Errorf("%s: managed block %s marker version %s, expected %s", output.Lock.Target, output.Lock.ID, markerVersion, previousLock.SOPVersion)
			}
			merged, err := replaceManagedBlock(current, output.Content, output.Lock.ID, output.Lock.MarkerStyle)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", output.Lock.Target, err)
			}
			output.Content = merged
		case "full":
			return nil, fmt.Errorf("%s: full-file management for output %s is unsupported because ownership cannot be proven; user file was preserved", output.Lock.Target, output.Lock.ID)
		default:
			return nil, fmt.Errorf("%s: unknown management %q", output.Lock.Target, output.Lock.Management)
		}
		if !bytes.Equal(current, output.Content) {
			changes = append(changes, projectChange{Target: output.Lock.Target, Content: output.Content})
		}
	}

	for _, previous := range previousLock.Outputs {
		if _, active := activeIDs[previous.ID]; active {
			continue
		}
		if !trustedPreviousLock {
			return nil, fmt.Errorf(
				"%s: stale output %s is not backed by this machine's trusted managed lock; user file was preserved. Run render once with the current profile and matching release before previewing a removal",
				previous.Target, previous.ID,
			)
		}
		if err := rejectManagedSymlinkTraversal(projectRoot, previous.Target); err != nil {
			return nil, err
		}
		path := filepath.Join(projectRoot, filepath.FromSlash(previous.Target))
		current, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("%s: stale managed output %s does not exist", previous.Target, previous.ID)
			}
			return nil, fmt.Errorf("read %s: %w", previous.Target, err)
		}
		switch previous.Management {
		case "block":
			body, markerHash, ok := managedBlock(current, previous.ID, previous.MarkerStyle)
			if !ok {
				return nil, fmt.Errorf("%s: stale managed block %s is missing or malformed", previous.Target, previous.ID)
			}
			computedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalText(body))))
			if markerHash != previous.Hash || computedHash != previous.Hash {
				return nil, fmt.Errorf("%s: stale managed block %s was modified locally", previous.Target, previous.ID)
			}
			if markerVersion, ok := managedMarkerVersion(current, previous.ID, previous.MarkerStyle); !ok || markerVersion != previousLock.SOPVersion {
				return nil, fmt.Errorf("%s: stale managed block %s marker version %s, expected %s", previous.Target, previous.ID, markerVersion, previousLock.SOPVersion)
			}
			remaining, err := replaceManagedBlock(current, nil, previous.ID, previous.MarkerStyle)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", previous.Target, err)
			}
			if strings.TrimSpace(string(remaining)) == "" {
				changes = append(changes, projectChange{Target: previous.Target, Remove: true})
			} else {
				changes = append(changes, projectChange{Target: previous.Target, Content: remaining})
			}
		case "full":
			return nil, fmt.Errorf("%s: stale full-file output %s has no trusted ownership proof; user file was preserved and migration must be explicit", previous.Target, previous.ID)
		default:
			return nil, fmt.Errorf("%s: unknown management %q", previous.Target, previous.Management)
		}
	}
	return changes, nil
}

func readLock(projectRoot string) (Lock, bool, error) {
	data, err := os.ReadFile(filepath.Join(projectRoot, ".sop", "lock.json"))
	if errors.Is(err, os.ErrNotExist) {
		return Lock{}, false, nil
	}
	if err != nil {
		return Lock{}, false, fmt.Errorf("read .sop/lock.json: %w", err)
	}
	lock, err := decodeLock(data)
	if err != nil {
		return Lock{}, false, fmt.Errorf("parse .sop/lock.json: %w", err)
	}
	return lock, true, nil
}

func replaceManagedBlock(current []byte, candidate []byte, id string, markerStyle string) ([]byte, error) {
	pattern, ok := managedBlockPattern(id, markerStyle)
	if !ok {
		return nil, fmt.Errorf("managed block %s uses unknown marker style %q", id, markerStyle)
	}
	location := pattern.FindIndex(current)
	if location == nil {
		return nil, fmt.Errorf("managed block %s is missing or malformed", id)
	}
	candidate = matchExistingLineEndings(current, candidate)
	merged := make([]byte, 0, len(current)-location[1]+location[0]+len(candidate))
	merged = append(merged, current[:location[0]]...)
	merged = append(merged, candidate...)
	merged = append(merged, current[location[1]:]...)
	return merged, nil
}

func matchExistingLineEndings(current []byte, candidate []byte) []byte {
	currentText := string(current)
	if !strings.Contains(currentText, "\r\n") || strings.Contains(strings.ReplaceAll(currentText, "\r\n", ""), "\n") {
		return candidate
	}
	normalized := strings.ReplaceAll(string(candidate), "\r\n", "\n")
	return []byte(strings.ReplaceAll(normalized, "\n", "\r\n"))
}

func renderComponent(component config.ComponentSpec, slots map[string]config.SlotSpec, profile map[string]any, assetRoot string) (string, error) {
	templateData, err := os.ReadFile(filepath.Join(assetRoot, filepath.FromSlash(component.Template)))
	if err != nil {
		return "", err
	}
	rendered := string(templateData)
	for _, name := range component.Slots {
		slot := slots[name]
		value, err := jsonPointer(profile, slot.Source)
		if err != nil {
			if slot.Required {
				return "", fmt.Errorf("slot %s: %w", name, err)
			}
			value = ""
		}
		text, err := formatSlot(value, slot)
		if err != nil {
			return "", fmt.Errorf("slot %s: %w", name, err)
		}
		rendered = strings.ReplaceAll(rendered, "{{"+name+"}}", text)
	}
	return canonicalText(rendered), nil
}

func profileMap(profile config.Profile) (map[string]any, error) {
	data, err := json.Marshal(profile)
	if err != nil {
		return nil, fmt.Errorf("encode profile: %w", err)
	}
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("decode profile values: %w", err)
	}
	switch profile.Risk {
	case "reversible":
		values["risk_guidance"] = "局部可回滚工作由 agent 直接完成，照常独立 review；只在既有高风险闸命中时回 owner。"
	case "controlled":
		values["risk_guidance"] = "实现可自决，但合并前必须强化 review，并把风险与回滚证据交 owner 确认。"
	case "high":
		values["risk_guidance"] = "spec、实现、合并三处都必须强化 review；执行或合并前回 owner 明确确认，不自动合。"
	default:
		return nil, fmt.Errorf("profile.risk %q has no runtime guidance", profile.Risk)
	}
	return values, nil
}

func jsonPointer(root map[string]any, pointer string) (any, error) {
	if pointer == "" || pointer[0] != '/' {
		return nil, errors.New("source must be a JSON pointer")
	}
	var current any = root
	for _, part := range strings.Split(pointer[1:], "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		object, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s is not an object", pointer)
		}
		current, ok = object[part]
		if !ok {
			return nil, fmt.Errorf("%s does not exist", pointer)
		}
	}
	return current, nil
}

func formatSlot(value any, slot config.SlotSpec) (string, error) {
	switch slot.Type {
	case "string":
		text, ok := value.(string)
		if !ok {
			return "", errors.New("must resolve to string")
		}
		if err := validateSlotText(text, slot.Format); err != nil {
			return "", err
		}
		return text, nil
	case "string_list":
		values, ok := value.([]any)
		if !ok {
			return "", errors.New("must resolve to string list")
		}
		items := make([]string, 0, len(values))
		for _, value := range values {
			text, ok := value.(string)
			if !ok {
				return "", errors.New("must contain only strings")
			}
			if err := validateSlotText(text, "inline"); err != nil {
				return "", err
			}
			items = append(items, text)
		}
		separator := " / "
		if slot.Format == "lines" {
			separator = "\n"
		}
		return strings.Join(items, separator), nil
	default:
		return "", fmt.Errorf("unsupported type %q", slot.Type)
	}
}

func validateSlotText(text, format string) error {
	for _, character := range text {
		if unicode.IsControl(character) {
			return fmt.Errorf("%s value must not contain control characters", format)
		}
	}
	return nil
}

func conditionMatches(condition string, profile config.Profile) (bool, error) {
	switch condition {
	case "", "always":
		return true, nil
	case "collaborators":
		return len(profile.Humans) >= 2, nil
	case "multiend":
		return len(profile.Ends) >= 2, nil
	case "parallel":
		return profile.ParallelAgents && len(profile.Ends) >= 2, nil
	case "serial":
		return !profile.ParallelAgents, nil
	case "collaborators_or_parallel":
		return len(profile.Humans) >= 2 || profile.ParallelAgents, nil
	default:
		return false, fmt.Errorf("unknown condition %q", condition)
	}
}

func canonicalText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.TrimRight(value, "\n") + "\n"
}
