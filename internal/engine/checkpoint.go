package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Strelizialeomon/sop-better/internal/config"
	"github.com/Strelizialeomon/sop-better/internal/platform"
	"github.com/Strelizialeomon/sop-better/internal/projectid"
)

const checkpointFormat = 1

var checkpointIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
var checkpointContentPattern = regexp.MustCompile(`^files/[0-9]{4,}$`)
var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type checkpoint struct {
	Format        int               `json:"format"`
	ID            string            `json:"id"`
	CreatedAt     string            `json:"created_at"`
	ProjectID     string            `json:"project_id"`
	LockSHA256    string            `json:"lock_sha256"`
	ProfileSHA256 string            `json:"profile_sha256"`
	Entries       []checkpointEntry `json:"entries"`
}

type checkpointEntry struct {
	ID          string `json:"id"`
	Target      string `json:"target"`
	Management  string `json:"management"`
	MarkerStyle string `json:"marker_style,omitempty"`
	Existed     bool   `json:"existed"`
	ContentFile string `json:"content_file,omitempty"`
	Mode        uint32 `json:"mode,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
}

func createCheckpoint(projectRoot string, next []RenderedOutput) (string, error) {
	previousLock, exists, err := readLock(projectRoot)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", nil
	}
	stateHome, err := projectStateHome()
	if err != nil {
		return "", err
	}
	projectID, err := projectIdentifier(projectRoot)
	if err != nil {
		return "", err
	}
	lockData, err := os.ReadFile(filepath.Join(projectRoot, ".sop", "lock.json"))
	if err != nil {
		return "", fmt.Errorf("read lock for checkpoint: %w", err)
	}
	lockHash := sha256.Sum256(lockData)
	id := fmt.Sprintf("%d-%x", time.Now().UTC().UnixNano(), lockHash[:4])
	parent := filepath.Join(stateHome, "projects", projectID, "checkpoints")
	temporary := filepath.Join(parent, "."+id+".tmp")
	final := filepath.Join(parent, id)
	if err := os.MkdirAll(temporary, 0o700); err != nil {
		return "", fmt.Errorf("create checkpoint staging: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(temporary)
		}
	}()
	if err := os.WriteFile(filepath.Join(temporary, "lock.json"), lockData, 0o600); err != nil {
		return "", fmt.Errorf("write checkpoint lock: %w", err)
	}
	profileData, err := os.ReadFile(filepath.Join(projectRoot, ".sop", "profile.json"))
	if err != nil {
		return "", fmt.Errorf("read profile for checkpoint: %w", err)
	}
	checkpointProfile, err := config.ParseProfile(profileData)
	if err != nil {
		return "", fmt.Errorf("validate profile for checkpoint: %w", err)
	}
	checkpointProfileDigest, err := profileDigest(checkpointProfile)
	if err != nil {
		return "", err
	}
	if previousLock.ProfileHash == "" || checkpointProfileDigest != previousLock.ProfileHash {
		return "", errors.New(".sop/profile.json changed after the previous render; restore it, then preview and commit the candidate with render --profile so rollback state is preserved")
	}
	if err := os.WriteFile(filepath.Join(temporary, "profile.json"), profileData, 0o600); err != nil {
		return "", fmt.Errorf("write checkpoint profile: %w", err)
	}
	profileHash := sha256.Sum256(profileData)

	byID := make(map[string]LockOutput, len(previousLock.Outputs)+len(next))
	for _, output := range previousLock.Outputs {
		byID[output.ID] = output
	}
	for _, output := range next {
		if _, ok := byID[output.Lock.ID]; !ok {
			byID[output.Lock.ID] = output.Lock
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	metadata := checkpoint{
		Format:        checkpointFormat,
		ID:            id,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		ProjectID:     projectID,
		LockSHA256:    fmt.Sprintf("%x", lockHash),
		ProfileSHA256: fmt.Sprintf("%x", profileHash),
		Entries:       make([]checkpointEntry, 0, len(ids)),
	}
	for index, id := range ids {
		output := byID[id]
		entry := checkpointEntry{
			ID:          output.ID,
			Target:      output.Target,
			Management:  output.Management,
			MarkerStyle: output.MarkerStyle,
		}
		path := filepath.Join(projectRoot, filepath.FromSlash(output.Target))
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			metadata.Entries = append(metadata.Entries, entry)
			continue
		}
		if err != nil {
			return "", fmt.Errorf("checkpoint inspect %s: %w", output.Target, err)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("checkpoint target %s is not a regular file", output.Target)
		}
		current, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("checkpoint read %s: %w", output.Target, err)
		}
		stored := current
		if output.Management == "block" {
			pattern, ok := managedBlockPattern(output.ID, output.MarkerStyle)
			if !ok {
				return "", fmt.Errorf("checkpoint output %s has unknown marker style %q", output.ID, output.MarkerStyle)
			}
			stored = pattern.Find(current)
			if stored == nil {
				return "", fmt.Errorf("checkpoint output %s is missing from %s", output.ID, output.Target)
			}
		}
		contentFile := filepath.Join("files", fmt.Sprintf("%04d", index))
		contentPath := filepath.Join(temporary, contentFile)
		if err := os.MkdirAll(filepath.Dir(contentPath), 0o700); err != nil {
			return "", fmt.Errorf("create checkpoint content directory: %w", err)
		}
		if err := os.WriteFile(contentPath, stored, 0o600); err != nil {
			return "", fmt.Errorf("write checkpoint content: %w", err)
		}
		entry.Existed = true
		entry.ContentFile = filepath.ToSlash(contentFile)
		entry.Mode = uint32(info.Mode().Perm())
		contentHash := sha256.Sum256(stored)
		entry.SHA256 = fmt.Sprintf("%x", contentHash)
		metadata.Entries = append(metadata.Entries, entry)
	}
	metadataData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode checkpoint: %w", err)
	}
	metadataData = append(metadataData, '\n')
	if err := os.WriteFile(filepath.Join(temporary, "checkpoint.json"), metadataData, 0o600); err != nil {
		return "", fmt.Errorf("write checkpoint metadata: %w", err)
	}
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", fmt.Errorf("create checkpoint directory: %w", err)
	}
	if err := os.Rename(temporary, final); err != nil {
		return "", fmt.Errorf("commit checkpoint: %w", err)
	}
	cleanup = false
	return final, nil
}

type RollbackResult struct {
	Profile config.Profile
	Lock    Lock
}

type ProjectCheckpoint struct {
	ID         string
	CreatedAt  string
	SOPVersion string
	Status     string
	Problem    string
}

func ListProjectCheckpoints(projectRoot string) ([]ProjectCheckpoint, error) {
	var checkpoints []ProjectCheckpoint
	err := withProjectOperationLock(projectRoot, func() error {
		if err := ensureProjectIdle(projectRoot); err != nil {
			return err
		}
		stateHome, err := projectStateHome()
		if err != nil {
			return err
		}
		projectID, err := projectIdentifier(projectRoot)
		if err != nil {
			return err
		}
		parent := filepath.Join(stateHome, "projects", projectID, "checkpoints")
		entries, err := os.ReadDir(parent)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("list project checkpoints: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || !checkpointIDPattern.MatchString(entry.Name()) {
				continue
			}
			checkpoint := inspectProjectCheckpoint(filepath.Join(parent, entry.Name()), entry.Name(), projectID, projectRoot)
			checkpoints = append(checkpoints, checkpoint)
		}
		sort.Slice(checkpoints, func(i, j int) bool { return checkpoints[i].ID > checkpoints[j].ID })
		return nil
	})
	return checkpoints, err
}

func inspectProjectCheckpoint(root, id, projectID, projectRoot string) ProjectCheckpoint {
	result := ProjectCheckpoint{ID: id, Status: "damaged"}
	metadataData, err := os.ReadFile(filepath.Join(root, "checkpoint.json"))
	if err != nil {
		result.Problem = fmt.Sprintf("read metadata: %v", err)
		return result
	}
	metadata, err := decodeCheckpoint(metadataData)
	if err != nil {
		result.Problem = fmt.Sprintf("parse metadata: %v", err)
		return result
	}
	result.CreatedAt = metadata.CreatedAt
	if metadata.Format != checkpointFormat || metadata.ID != id || metadata.ProjectID != projectID {
		result.Problem = "metadata identity does not match this project"
		return result
	}
	lockData, err := os.ReadFile(filepath.Join(root, "lock.json"))
	if err != nil {
		result.Problem = fmt.Sprintf("read lock: %v", err)
		return result
	}
	if fmt.Sprintf("%x", sha256.Sum256(lockData)) != metadata.LockSHA256 {
		result.Problem = "lock checksum mismatch"
		return result
	}
	lock, err := decodeLock(lockData)
	if err != nil {
		result.Problem = fmt.Sprintf("parse lock: %v", err)
		return result
	}
	if lock.SchemaVersion != 1 || strings.TrimSpace(lock.GeneratorVersion) == "" || strings.TrimSpace(lock.RulesVersion) == "" {
		result.Problem = "lock contract is incomplete or unsupported"
		return result
	}
	result.SOPVersion = lock.SOPVersion
	profileData, err := os.ReadFile(filepath.Join(root, "profile.json"))
	if err != nil {
		result.Problem = fmt.Sprintf("read profile: %v", err)
		return result
	}
	if fmt.Sprintf("%x", sha256.Sum256(profileData)) != metadata.ProfileSHA256 {
		result.Problem = "profile checksum mismatch"
		return result
	}
	profile, err := config.ParseProfile(profileData)
	if err != nil {
		result.Problem = fmt.Sprintf("parse profile: %v", err)
		return result
	}
	if profile.SchemaVersion != 1 || profile.SOPVersion != lock.SOPVersion {
		result.Problem = "profile and lock versions are incompatible"
		return result
	}
	profileHash, err := profileDigest(profile)
	if err != nil || lock.ProfileHash == "" || profileHash != lock.ProfileHash {
		result.Problem = "profile does not match checkpoint lock"
		return result
	}
	if err := validateCheckpointMetadata(root, projectRoot, metadata, lock); err != nil {
		result.Problem = fmt.Sprintf("unsafe or inconsistent metadata: %v", err)
		return result
	}
	outputs := make(map[string]LockOutput, len(lock.Outputs))
	for _, output := range lock.Outputs {
		outputs[output.ID] = output
	}
	for _, entry := range metadata.Entries {
		if !entry.Existed {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(entry.ContentFile)))
		if err != nil {
			result.Problem = fmt.Sprintf("read content for %s: %v", entry.Target, err)
			return result
		}
		if fmt.Sprintf("%x", sha256.Sum256(content)) != entry.SHA256 {
			result.Problem = fmt.Sprintf("content checksum mismatch for %s", entry.Target)
			return result
		}
		output, ok := outputs[entry.ID]
		if !ok || output.Target != entry.Target || output.Management != entry.Management || output.MarkerStyle != entry.MarkerStyle {
			result.Problem = fmt.Sprintf("content %s does not match checkpoint lock", entry.Target)
			return result
		}
		if err := verifyCheckpointContent(content, output, lock.SOPVersion); err != nil {
			result.Problem = fmt.Sprintf("content %s: %v", entry.Target, err)
			return result
		}
	}
	result.Status = "ready"
	result.Problem = ""
	return result
}

func RollbackProject(projectRoot string, checkpointID string, profile config.Profile, manifest config.Manifest) (RollbackResult, error) {
	var result RollbackResult
	err := withProjectOperationLock(projectRoot, func() error {
		return rollbackProjectLocked(projectRoot, checkpointID, profile, manifest, &result)
	})
	return result, err
}

func rollbackProjectLocked(projectRoot string, checkpointID string, profile config.Profile, manifest config.Manifest, result *RollbackResult) error {
	if !checkpointIDPattern.MatchString(checkpointID) {
		return errors.New("checkpoint id contains unsupported characters")
	}
	if err := recoverTransaction(projectRoot); err != nil {
		return fmt.Errorf("recover interrupted transaction before rollback: %w", err)
	}
	stateHome, err := projectStateHome()
	if err != nil {
		return err
	}
	projectID, err := projectIdentifier(projectRoot)
	if err != nil {
		return err
	}
	root := filepath.Join(stateHome, "projects", projectID, "checkpoints", checkpointID)
	metadataData, err := os.ReadFile(filepath.Join(root, "checkpoint.json"))
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("checkpoint %s does not exist for this project", checkpointID)
	}
	if err != nil {
		return fmt.Errorf("read checkpoint %s: %w", checkpointID, err)
	}
	metadata, err := decodeCheckpoint(metadataData)
	if err != nil {
		return fmt.Errorf("parse checkpoint %s: %w", checkpointID, err)
	}
	if metadata.Format != checkpointFormat || metadata.ID != checkpointID || metadata.ProjectID != projectID {
		return fmt.Errorf("checkpoint %s metadata does not match this project", checkpointID)
	}
	checkpointLock, err := os.ReadFile(filepath.Join(root, "lock.json"))
	if err != nil {
		return fmt.Errorf("read checkpoint lock: %w", err)
	}
	checkpointLockHash := fmt.Sprintf("%x", sha256.Sum256(checkpointLock))
	if metadata.LockSHA256 == "" || checkpointLockHash != metadata.LockSHA256 {
		return errors.New("checkpoint lock checksum mismatch; project was not changed")
	}
	targetLock, err := decodeLock(checkpointLock)
	if err != nil {
		return fmt.Errorf("parse checkpoint lock: %w; project was not changed", err)
	}
	if targetLock.SchemaVersion != 1 {
		return fmt.Errorf("checkpoint %s uses unsupported lock schema %d; project was not changed", checkpointID, targetLock.SchemaVersion)
	}
	if strings.TrimSpace(targetLock.GeneratorVersion) == "" || strings.TrimSpace(targetLock.RulesVersion) == "" {
		return fmt.Errorf("checkpoint %s lock is missing generator or rules version; project was not changed", checkpointID)
	}
	if !manifest.SupportsProfileSchema(profile.SchemaVersion) || profile.SOPVersion != manifest.SOPVersion {
		return errors.New("current profile is incompatible with checkpoint engine/assets; project was not changed")
	}
	targetProfileData, err := os.ReadFile(filepath.Join(root, "profile.json"))
	if err != nil {
		return fmt.Errorf("read checkpoint profile: %w; project was not changed", err)
	}
	targetProfileHash := fmt.Sprintf("%x", sha256.Sum256(targetProfileData))
	if !sha256Pattern.MatchString(metadata.ProfileSHA256) || targetProfileHash != metadata.ProfileSHA256 {
		return errors.New("checkpoint profile checksum mismatch; project was not changed")
	}
	targetProfile, err := config.ParseProfile(targetProfileData)
	if err != nil {
		return fmt.Errorf("parse checkpoint profile: %w; project was not changed", err)
	}
	if !manifest.SupportsProfileSchema(targetProfile.SchemaVersion) {
		return fmt.Errorf("checkpoint %s profile schema %d is unsupported by the current engine; project was not changed", checkpointID, targetProfile.SchemaVersion)
	}
	if targetProfile.SOPVersion != targetLock.SOPVersion {
		return fmt.Errorf("checkpoint %s profile and lock SOP versions differ; project was not changed", checkpointID)
	}
	targetProfileDigest, err := profileDigest(targetProfile)
	if err != nil {
		return fmt.Errorf("hash checkpoint profile: %w; project was not changed", err)
	}
	if targetLock.ProfileHash == "" || targetProfileDigest != targetLock.ProfileHash {
		return fmt.Errorf("checkpoint %s profile does not match its lock; project was not changed", checkpointID)
	}
	if err := validateCheckpointMetadata(root, projectRoot, metadata, targetLock); err != nil {
		return fmt.Errorf("checkpoint %s is unsafe or inconsistent: %w; project was not changed", checkpointID, err)
	}
	targetByID := make(map[string]LockOutput, len(targetLock.Outputs))
	for _, output := range targetLock.Outputs {
		if _, exists := targetByID[output.ID]; exists {
			return fmt.Errorf("checkpoint lock output %s is duplicated; project was not changed", output.ID)
		}
		targetByID[output.ID] = output
	}
	currentLock, exists, err := readLock(projectRoot)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("current .sop/lock.json does not exist; project was not changed")
	}
	currentLockData, err := os.ReadFile(filepath.Join(projectRoot, ".sop", "lock.json"))
	if err != nil {
		return fmt.Errorf("read current lock provenance input: %w; project was not changed", err)
	}
	trustedCurrentLock, err := isTrustedManagedLock(projectRoot, currentLockData)
	if err != nil {
		return fmt.Errorf("verify current managed lock provenance: %w; project was not changed", err)
	}
	if !trustedCurrentLock {
		return errors.New("current lock is not backed by this machine's trusted managed lock; project was not changed")
	}
	currentByID := make(map[string]LockOutput, len(currentLock.Outputs))
	for _, output := range currentLock.Outputs {
		currentByID[output.ID] = output
	}

	files := make([]transactionFile, 0, len(metadata.Entries)+1)
	seenEntries := make(map[string]struct{}, len(metadata.Entries))
	for _, entry := range metadata.Entries {
		if _, exists := seenEntries[entry.ID]; exists {
			return fmt.Errorf("checkpoint entry %s is duplicated; project was not changed", entry.ID)
		}
		seenEntries[entry.ID] = struct{}{}
		currentOutput, ok := currentByID[entry.ID]
		if ok && currentOutput.Target != entry.Target {
			return fmt.Errorf("%s: current lock tracks managed output %s at %s", entry.Target, entry.ID, currentOutput.Target)
		}
		path := filepath.Join(projectRoot, filepath.FromSlash(entry.Target))
		current, readErr := os.ReadFile(path)
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return fmt.Errorf("read current %s: %w", entry.Target, readErr)
		}

		if !entry.Existed {
			if !ok || errors.Is(readErr, os.ErrNotExist) {
				continue
			}
			remaining, remove, err := removeVerifiedCurrentOutput(current, currentOutput, currentLock.SOPVersion)
			if err != nil {
				return fmt.Errorf("%s: %w; project was not changed", entry.Target, err)
			}
			files = append(files, transactionFile{Target: entry.Target, Content: remaining, Mode: os.FileMode(entry.Mode), Remove: remove})
			continue
		}
		stored, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(entry.ContentFile)))
		if err != nil {
			return fmt.Errorf("read checkpoint content for %s: %w", entry.Target, err)
		}
		storedHash := fmt.Sprintf("%x", sha256.Sum256(stored))
		if entry.SHA256 == "" || storedHash != entry.SHA256 {
			return fmt.Errorf("checkpoint content checksum mismatch for %s; project was not changed", entry.Target)
		}
		targetOutput, trackedByTarget := targetByID[entry.ID]
		if !trackedByTarget || targetOutput.Target != entry.Target || targetOutput.Management != entry.Management {
			return fmt.Errorf("checkpoint content %s does not match checkpoint lock; project was not changed", entry.Target)
		}
		if err := verifyCheckpointContent(stored, targetOutput, targetLock.SOPVersion); err != nil {
			return fmt.Errorf("checkpoint content %s: %w; project was not changed", entry.Target, err)
		}
		content := stored
		if ok {
			if _, _, err := removeVerifiedCurrentOutput(current, currentOutput, currentLock.SOPVersion); err != nil {
				return fmt.Errorf("%s: %w; project was not changed", entry.Target, err)
			}
			if entry.Management == "block" {
				content, err = replaceManagedBlock(current, stored, currentOutput.ID, currentOutput.MarkerStyle)
				if err != nil {
					return fmt.Errorf("%s: %w", entry.Target, err)
				}
			}
		} else if readErr == nil {
			if entry.Management != "block" {
				return fmt.Errorf("%s: untracked local file blocks restoring full managed output %s", entry.Target, entry.ID)
			}
			content = appendManagedBlock(current, stored)
		}
		files = append(files, transactionFile{Target: entry.Target, Content: content, Mode: os.FileMode(entry.Mode)})
	}
	for _, currentOutput := range currentLock.Outputs {
		if _, represented := seenEntries[currentOutput.ID]; represented {
			continue
		}
		path := filepath.Join(projectRoot, filepath.FromSlash(currentOutput.Target))
		current, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read later-only output %s: %w; project was not changed", currentOutput.Target, err)
		}
		remaining, remove, err := removeVerifiedCurrentOutput(current, currentOutput, currentLock.SOPVersion)
		if err != nil {
			return fmt.Errorf("%s: %w; project was not changed", currentOutput.Target, err)
		}
		files = append(files, transactionFile{Target: currentOutput.Target, Content: remaining, Mode: 0o644, Remove: remove})
	}
	for id := range targetByID {
		if _, represented := seenEntries[id]; !represented {
			return fmt.Errorf("checkpoint lock output %s has no checkpoint entry; project was not changed", id)
		}
	}
	if err := validateRollbackProjection(projectRoot, targetLock, files, targetProfile); err != nil {
		return fmt.Errorf("checkpoint projection is invalid: %w; project was not changed", err)
	}
	currentProfileData, err := os.ReadFile(filepath.Join(projectRoot, ".sop", "profile.json"))
	if err != nil {
		return fmt.Errorf("read current profile before rollback: %w; project was not changed", err)
	}
	if !bytes.Equal(currentProfileData, targetProfileData) {
		files = append(files, transactionFile{Target: ".sop/profile.json", Content: targetProfileData, Mode: 0o644})
	}
	files = append(files, transactionFile{Target: ".sop/lock.json", Content: checkpointLock, Mode: 0o644})
	if err := applyTransactionLocked(projectRoot, files, transactionOptions{BeforeCommit: func() error {
		loadedProfile, err := config.LoadProfile(filepath.Join(projectRoot, ".sop", "profile.json"))
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(loadedProfile, targetProfile) {
			return errors.New("rolled-back profile does not match checkpoint profile")
		}
		return checkProjectState(projectRoot, targetProfile, targetLock)
	}}); err != nil {
		return err
	}
	if err := persistTrustedManagedLock(projectRoot, checkpointLock); err != nil {
		return fmt.Errorf("project rollback committed, but managed lock provenance was not recorded: %w", err)
	}
	*result = RollbackResult{Profile: targetProfile, Lock: targetLock}
	return nil
}

func decodeCheckpoint(data []byte) (checkpoint, error) {
	var metadata checkpoint
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&metadata); err != nil {
		return checkpoint{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return metadata, nil
	} else if err != nil {
		return checkpoint{}, err
	}
	return checkpoint{}, errors.New("unexpected trailing JSON value")
}

func validateCheckpointMetadata(checkpointRoot string, projectRoot string, metadata checkpoint, targetLock Lock) error {
	lockByID := make(map[string]LockOutput, len(targetLock.Outputs))
	lockTargets := make(map[string]string, len(targetLock.Outputs))
	for _, output := range targetLock.Outputs {
		if !checkpointIDPattern.MatchString(output.ID) {
			return fmt.Errorf("lock output id %q contains unsupported characters", output.ID)
		}
		if !safeProjectTarget(output.Target) {
			return fmt.Errorf("lock output %s target must be repository-relative", output.ID)
		}
		foldedTarget := strings.ToLower(output.Target)
		if previous, exists := lockTargets[foldedTarget]; exists {
			return fmt.Errorf("lock outputs %s and %s share target %s", previous, output.ID, output.Target)
		}
		lockTargets[foldedTarget] = output.ID
		if _, exists := lockByID[output.ID]; exists {
			return fmt.Errorf("lock output id %s is duplicated", output.ID)
		}
		if err := validateCheckpointOutputShape(output.Management, output.MarkerStyle, output.Hash); err != nil {
			return fmt.Errorf("lock output %s: %w", output.ID, err)
		}
		lockByID[output.ID] = output
	}

	seenIDs := make(map[string]struct{}, len(metadata.Entries))
	seenTargets := make(map[string]string, len(metadata.Entries))
	seenContent := make(map[string]string, len(metadata.Entries))
	for _, entry := range metadata.Entries {
		if !checkpointIDPattern.MatchString(entry.ID) {
			return fmt.Errorf("entry id %q contains unsupported characters", entry.ID)
		}
		if _, exists := seenIDs[entry.ID]; exists {
			return fmt.Errorf("entry id %s is duplicated", entry.ID)
		}
		seenIDs[entry.ID] = struct{}{}
		if !safeProjectTarget(entry.Target) {
			return fmt.Errorf("entry %s target must be repository-relative", entry.ID)
		}
		if err := rejectManagedSymlinkTraversal(projectRoot, entry.Target); err != nil {
			return err
		}
		foldedTarget := strings.ToLower(entry.Target)
		if previous, exists := seenTargets[foldedTarget]; exists {
			return fmt.Errorf("entries %s and %s share target %s", previous, entry.ID, entry.Target)
		}
		seenTargets[foldedTarget] = entry.ID
		if err := validateCheckpointOutputShape(entry.Management, entry.MarkerStyle, entry.SHA256); err != nil {
			if !entry.Existed && entry.SHA256 == "" {
				if entry.Management != "block" && entry.Management != "full" {
					return fmt.Errorf("entry %s: %w", entry.ID, err)
				}
			} else {
				return fmt.Errorf("entry %s: %w", entry.ID, err)
			}
		}
		if output, tracked := lockByID[entry.ID]; tracked {
			if output.Target != entry.Target || output.Management != entry.Management || output.MarkerStyle != entry.MarkerStyle {
				return fmt.Errorf("entry %s does not match checkpoint lock", entry.ID)
			}
		} else if entry.Existed {
			return fmt.Errorf("existing entry %s is not tracked by checkpoint lock", entry.ID)
		}
		if !entry.Existed {
			if entry.ContentFile != "" || entry.SHA256 != "" {
				return fmt.Errorf("entry %s did not exist but contains stored content metadata", entry.ID)
			}
			continue
		}
		if !checkpointContentPattern.MatchString(entry.ContentFile) {
			return fmt.Errorf("entry %s content_file must be a checkpoint-local files/<number> path", entry.ID)
		}
		if previous, exists := seenContent[entry.ContentFile]; exists {
			return fmt.Errorf("entries %s and %s share content_file %s", previous, entry.ID, entry.ContentFile)
		}
		seenContent[entry.ContentFile] = entry.ID
		if err := rejectManagedSymlinkTraversal(checkpointRoot, entry.ContentFile); err != nil {
			return fmt.Errorf("entry %s content_file: %w", entry.ID, err)
		}
	}
	for id := range lockByID {
		if _, exists := seenIDs[id]; !exists {
			return fmt.Errorf("lock output %s has no checkpoint entry", id)
		}
	}
	return nil
}

func validateCheckpointOutputShape(management string, markerStyle string, hash string) error {
	switch management {
	case "block":
		if markerStyle != "html" && markerStyle != "hash" {
			return fmt.Errorf("block output has unsupported marker style %q", markerStyle)
		}
	case "full":
		if markerStyle != "" && markerStyle != "html" && markerStyle != "hash" {
			return fmt.Errorf("full output has unsupported marker style %q", markerStyle)
		}
	default:
		return fmt.Errorf("unsupported management %q", management)
	}
	if !sha256Pattern.MatchString(hash) {
		return errors.New("sha256 must contain 64 lowercase hexadecimal characters")
	}
	return nil
}

func validateRollbackProjection(projectRoot string, lock Lock, files []transactionFile, profile config.Profile) error {
	planned := make(map[string]transactionFile, len(files))
	virtualTargets := make(map[string]bool, len(lock.Outputs))
	for _, file := range files {
		target := filepath.ToSlash(file.Target)
		planned[target] = file
		if file.Remove {
			virtualTargets[target] = false
		}
	}
	for _, output := range lock.Outputs {
		virtualTargets[output.Target] = true
	}
	for _, output := range lock.Outputs {
		content := []byte(nil)
		if file, ok := planned[output.Target]; ok {
			if file.Remove {
				return fmt.Errorf("%s: target would be removed", output.Target)
			}
			content = file.Content
		} else {
			current, err := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(output.Target)))
			if err != nil {
				return fmt.Errorf("read %s: %w", output.Target, err)
			}
			content = current
		}
		body := string(content)
		if output.Management == "block" {
			managedBody, _, ok := managedBlock(content, output.ID, output.MarkerStyle)
			if !ok {
				return fmt.Errorf("%s: managed block %s is missing", output.Target, output.ID)
			}
			body = managedBody
		}
		if err := verifyCheckpointContent(extractManagedContent(content, output), output, lock.SOPVersion); err != nil {
			return fmt.Errorf("%s: %w", output.Target, err)
		}
		if err := checkGeneratedText(projectRoot, output, body, profile, virtualTargets); err != nil {
			return err
		}
	}
	return nil
}

func extractManagedContent(content []byte, output LockOutput) []byte {
	if output.Management != "block" {
		return content
	}
	pattern, ok := managedBlockPattern(output.ID, output.MarkerStyle)
	if !ok {
		return content
	}
	return pattern.Find(content)
}

func verifyCheckpointContent(content []byte, output LockOutput, sopVersion string) error {
	switch output.Management {
	case "block":
		body, markerHash, ok := managedBlock(content, output.ID, output.MarkerStyle)
		if !ok {
			return fmt.Errorf("managed block %s is missing or malformed", output.ID)
		}
		computed := fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalText(body))))
		if markerHash != output.Hash || computed != output.Hash {
			return fmt.Errorf("managed block %s hash mismatch", output.ID)
		}
		if markerVersion, ok := managedMarkerVersion(content, output.ID, output.MarkerStyle); !ok || markerVersion != sopVersion {
			return fmt.Errorf("managed block %s marker version %s, expected %s", output.ID, markerVersion, sopVersion)
		}
		return nil
	case "full":
		computed := fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalText(string(content)))))
		if computed != output.Hash {
			return errors.New("managed file hash mismatch")
		}
		return nil
	default:
		return fmt.Errorf("unknown management %q", output.Management)
	}
}

func removeVerifiedCurrentOutput(current []byte, output LockOutput, sopVersion string) ([]byte, bool, error) {
	switch output.Management {
	case "block":
		body, markerHash, ok := managedBlock(current, output.ID, output.MarkerStyle)
		if !ok {
			return nil, false, fmt.Errorf("current managed block %s is missing or malformed", output.ID)
		}
		computed := fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalText(body))))
		if markerHash != output.Hash || computed != output.Hash {
			return nil, false, fmt.Errorf("current managed block %s was modified locally", output.ID)
		}
		if markerVersion, ok := managedMarkerVersion(current, output.ID, output.MarkerStyle); !ok || markerVersion != sopVersion {
			return nil, false, fmt.Errorf("current managed block %s marker version %s, expected %s", output.ID, markerVersion, sopVersion)
		}
		remaining, err := replaceManagedBlock(current, nil, output.ID, output.MarkerStyle)
		if err != nil {
			return nil, false, err
		}
		return remaining, strings.TrimSpace(string(remaining)) == "", nil
	case "full":
		computed := fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalText(string(current)))))
		if computed != output.Hash {
			return nil, false, errors.New("current managed file was modified locally")
		}
		return nil, true, nil
	default:
		return nil, false, fmt.Errorf("current output %s has unknown management %q", output.ID, output.Management)
	}
}

func appendManagedBlock(current []byte, block []byte) []byte {
	if len(current) == 0 {
		return append([]byte(nil), block...)
	}
	combined := append([]byte(nil), current...)
	if !strings.HasSuffix(string(combined), "\n") {
		combined = append(combined, '\n')
	}
	combined = append(combined, '\n')
	return append(combined, block...)
}

func projectStateHome() (string, error) {
	stateHome, err := platform.StateHome()
	if err != nil {
		return "", fmt.Errorf("resolve SOP_STATE_HOME: %w", err)
	}
	return stateHome, nil
}

func projectIdentifier(projectRoot string) (string, error) {
	return projectid.Identifier(projectRoot)
}

func discardCheckpoint(path string) {
	if path != "" {
		_ = os.RemoveAll(path)
	}
}

func pruneCheckpoints(parent string, keep int) error {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return fmt.Errorf("list checkpoints: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() && checkpointIDPattern.MatchString(entry.Name()) && !strings.HasPrefix(entry.Name(), ".") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for len(names) > keep {
		if err := os.RemoveAll(filepath.Join(parent, names[0])); err != nil {
			return fmt.Errorf("prune checkpoint %s: %w", names[0], err)
		}
		names = names[1:]
	}
	return nil
}
