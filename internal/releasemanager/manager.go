package releasemanager

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"github.com/Strelizialeomon/sop-better/internal/config"
	"github.com/Strelizialeomon/sop-better/internal/platform"
	"github.com/Strelizialeomon/sop-better/internal/projectid"
	"github.com/Strelizialeomon/sop-better/internal/releasebundle"
	"github.com/Strelizialeomon/sop-better/internal/state"
)

var BuildVersion = "0.1.0-dev"
var ErrSimulatedCrash = errors.New("simulated release manager crash")

type incompatibleOutputTargetsError struct {
	moves []releasebundle.OutputTargetMove
}

func (err incompatibleOutputTargetsError) Error() string {
	return "release is incompatible: a published output ID changed target; retire the old ID and add a new ID instead"
}

const managerEngineProtocol = 1

type runtimeDescription struct {
	Component string `json:"component"`
	Version   string `json:"version"`
	Protocol  int    `json:"protocol"`
}

type Manager struct {
	StateHome     string
	ReleaseSource string
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	ProjectRoot   string
	Plugin        PluginController
	Confirmer     Confirmer
	AfterEvent    func(string) error
}

type ProcessExit struct {
	Code int
}

func (exit ProcessExit) Error() string {
	return fmt.Sprintf("process exited with code %d", exit.Code)
}

func ExitCode(err error) (int, bool) {
	var processExit ProcessExit
	if errors.As(err, &processExit) {
		return processExit.Code, true
	}
	return 0, false
}

func (manager Manager) Run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: sopctl-manager <release|project-command>")
	}
	if len(args) == 2 && args[0] == "__describe" && args[1] == "--json" {
		return json.NewEncoder(manager.stdout()).Encode(runtimeDescription{
			Component: "manager", Version: BuildVersion, Protocol: managerEngineProtocol,
		})
	}
	if err := manager.validateBootstrapHandoff(); err != nil {
		return err
	}
	if err := manager.recoverPending(context.Background()); err != nil {
		return fmt.Errorf("recover pending release transaction: %w", err)
	}
	if args[0] != "release" {
		return manager.forwardEngine(args)
	}
	if len(args) < 2 {
		return errors.New("usage: sopctl-manager release <check|diff|upgrade|rollback>")
	}
	switch args[1] {
	case "check":
		return manager.releaseCheck()
	case "diff":
		return manager.releaseDiff(args[2:])
	case "upgrade":
		return manager.releaseUpgrade(args[2:])
	case "rollback":
		return manager.releaseRollback(args[2:])
	default:
		return fmt.Errorf("release command %q is not implemented", args[1])
	}
}

func (manager Manager) recoverPending(ctx context.Context) error {
	_, exists, err := readJournal(manager.StateHome)
	if err != nil || !exists {
		return err
	}
	lock, err := state.AcquireLock(manager.StateHome)
	if err != nil {
		return err
	}
	defer lock.Close()
	return manager.Recover(ctx)
}

func (manager Manager) releaseUpgrade(args []string) error {
	flags := flag.NewFlagSet("release upgrade", flag.ContinueOnError)
	flags.SetOutput(manager.stderr())
	target := flags.String("to", "", "target release version")
	projectRoot := flags.String("project-root", manager.ProjectRoot, "project root for target-version read-only preview")
	profile := flags.String("profile", "", "target-compatible candidate profile for read-only preview")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return errors.New("release upgrade: --to <version> is required")
	}
	previewManager := manager
	previewManager.ProjectRoot = *projectRoot
	manifest, expectedBefore, err := previewManager.writeDiff(*target, *profile)
	if err != nil {
		return err
	}
	if compareVersions(*target, expectedBefore.Version) <= 0 {
		return fmt.Errorf("release upgrade target %s must be newer than current %s; use release rollback for the recorded previous version", *target, expectedBefore.Version)
	}
	if err := manager.confirm(*target); err != nil {
		return err
	}
	sourceRoot, err := manager.bundleRoot(*target)
	if err != nil {
		return err
	}
	if err := manager.switchRelease(context.Background(), "upgrade", sourceRoot, manifest, expectedBefore); err != nil {
		return err
	}
	manager.writeRestartNotice(*target)
	return nil
}

func (manager Manager) releaseRollback(args []string) (returnErr error) {
	flags := flag.NewFlagSet("release rollback", flag.ContinueOnError)
	flags.SetOutput(manager.stderr())
	explicitTarget := flags.String("to", "", "installed older release version")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("release rollback accepts only --to VERSION")
	}
	current, err := state.ReadCurrent(manager.StateHome)
	if err != nil {
		return err
	}
	targetVersion := *explicitTarget
	if targetVersion == "" {
		targetVersion = current.Previous
		if targetVersion == "" {
			return manager.rollbackTargetGuidance(current, "no previous version is recorded")
		}
	} else if !state.ValidVersion(targetVersion) {
		return errors.New("release rollback target must be strict semver")
	}
	if compareVersions(targetVersion, current.Version) >= 0 {
		return manager.rollbackTargetGuidance(current, fmt.Sprintf("target %s is not older than current %s", targetVersion, current.Version))
	}
	root, manifest, err := manager.inspectInstalledRollbackTarget(current.Version, targetVersion)
	if err != nil {
		return manager.rollbackTargetGuidance(current, fmt.Sprintf("target %s is not a healthy installed release: %v", targetVersion, err))
	}
	projectRoot, err := manager.resolveProjectRoot()
	if err != nil {
		return err
	}
	compatibilityManager := manager
	compatibilityManager.ProjectRoot = projectRoot
	if err := compatibilityManager.checkProjectCompatibility(manifest); err != nil {
		return err
	}
	projectLock, err := manager.acquireProjectOperationLock(projectRoot)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := projectLock.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("release project operation lock: %w", closeErr)
		}
	}()
	if err := compatibilityManager.checkProjectCompatibility(manifest); err != nil {
		return err
	}
	if err := compatibilityManager.checkProjectWithTargetEngine(root, manifest); err != nil {
		return err
	}
	_, diffCurrent, err := compatibilityManager.writeManifestDiff(targetVersion, root, "", false)
	if err != nil {
		return err
	}
	if diffCurrent != current {
		return errors.New("current release changed while preparing rollback; rerun diff and confirmation")
	}
	if err := manager.confirm(targetVersion); err != nil {
		return err
	}
	if err := manager.switchRelease(context.Background(), "rollback", root, manifest, current); err != nil {
		return err
	}
	manager.writeRestartNotice(targetVersion)
	return nil
}

func (manager Manager) inspectInstalledRollbackTarget(currentVersion, targetVersion string) (string, releasebundle.Manifest, error) {
	if !state.ValidVersion(targetVersion) {
		return "", releasebundle.Manifest{}, errors.New("version is not strict semver")
	}
	if compareVersions(targetVersion, currentVersion) >= 0 {
		return "", releasebundle.Manifest{}, errors.New("version is not older than current")
	}
	root := filepath.Join(manager.StateHome, "versions", targetVersion)
	manifest, err := releasebundle.Inspect(root)
	if err != nil {
		return "", releasebundle.Manifest{}, err
	}
	if manifest.Version != targetVersion {
		return "", releasebundle.Manifest{}, fmt.Errorf("installed directory contains version %s", manifest.Version)
	}
	if err := validateTargetRuntime(manifest); err != nil {
		return "", releasebundle.Manifest{}, err
	}
	if err := inspectTargetExecutables(root, manifest); err != nil {
		return "", releasebundle.Manifest{}, err
	}
	return root, manifest, nil
}

func (manager Manager) healthyInstalledRollbackTargets(currentVersion string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(manager.StateHome, "versions"))
	if err != nil {
		return nil, fmt.Errorf("list installed releases: %w", err)
	}
	targets := make([]string, 0, len(entries))
	for _, entry := range entries {
		version := entry.Name()
		if !entry.IsDir() || !state.ValidVersion(version) || compareVersions(version, currentVersion) >= 0 {
			continue
		}
		if _, _, err := manager.inspectInstalledRollbackTarget(currentVersion, version); err == nil {
			targets = append(targets, version)
		}
	}
	sort.Slice(targets, func(i, j int) bool { return compareVersions(targets[i], targets[j]) > 0 })
	return targets, nil
}

func (manager Manager) rollbackTargetGuidance(current state.Current, reason string) error {
	targets, err := manager.healthyInstalledRollbackTargets(current.Version)
	if err != nil {
		return fmt.Errorf("release rollback: %s; scan verified installed rollback targets: %w", reason, err)
	}
	available := "none"
	if len(targets) > 0 {
		available = strings.Join(targets, ", ")
	}
	return fmt.Errorf("release rollback: %s; healthy installed older targets: %s; rerun release rollback --to VERSION", reason, available)
}

func (manager Manager) acquireProjectOperationLock(projectRoot string) (*state.Lock, error) {
	projectID, err := projectid.Identifier(projectRoot)
	if err != nil {
		return nil, err
	}
	lockPath := filepath.Join(manager.StateHome, "projects", projectID, "operation.lock")
	lock, err := state.AcquireFileLock(lockPath)
	if errors.Is(err, state.ErrLockBusy) {
		return nil, errors.New("project operation is already running; release command did not change the installation")
	}
	if err != nil {
		return nil, fmt.Errorf("acquire project operation lock: %w", err)
	}
	return lock, nil
}

func (manager Manager) checkProjectWithTargetEngine(versionRoot string, manifest releasebundle.Manifest) error {
	profileExists, err := regularOrMissing(filepath.Join(manager.ProjectRoot, ".sop", "profile.json"))
	if err != nil {
		return err
	}
	lockExists, err := regularOrMissing(filepath.Join(manager.ProjectRoot, ".sop", "lock.json"))
	if err != nil {
		return err
	}
	if !profileExists && !lockExists {
		return nil
	}
	if err := validateTargetRuntime(manifest); err != nil {
		return fmt.Errorf("rollback target runtime: %w", err)
	}
	if err := inspectTargetExecutables(versionRoot, manifest); err != nil {
		return fmt.Errorf("rollback target runtime preflight: %w", err)
	}
	projectID, err := projectid.Identifier(manager.ProjectRoot)
	if err != nil {
		return err
	}
	enginePath := filepath.Join(versionRoot, filepath.FromSlash(manifest.Executables.Engine.Path))
	command := exec.Command(enginePath, "check", "--project-root", manager.ProjectRoot)
	command.Env = environmentWithOverrides(os.Environ(), map[string]string{
		"SOP_STATE_HOME":        manager.StateHome,
		"SOP_RELEASE_VERSION":   manifest.Version,
		"SOP_ASSET_ROOT":        filepath.Join(versionRoot, "assets"),
		"SOP_PROJECT_LOCK_HELD": projectID,
	})
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("project is not valid under rollback target %s: %w: %s", manifest.Version, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (manager Manager) confirm(version string) error {
	if manager.Confirmer == nil {
		return errors.New("interactive confirmation is required")
	}
	confirmed, err := manager.Confirmer.Confirm(context.Background(), version)
	if err != nil {
		return err
	}
	if !confirmed {
		return errors.New("release change cancelled; current installation is unchanged")
	}
	return nil
}

func (manager Manager) switchRelease(ctx context.Context, operation, sourceRoot string, target releasebundle.Manifest, expectedBefore state.Current) (returnErr error) {
	if manager.Plugin == nil {
		return errors.New("plugin controller is required for release changes")
	}
	if err := validateTargetRuntime(target); err != nil {
		return err
	}
	lock, err := state.AcquireLock(manager.StateHome)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := manager.Recover(ctx); err != nil {
		return fmt.Errorf("recover previous release transaction: %w", err)
	}
	before, err := state.ReadCurrent(manager.StateHome)
	if err != nil {
		return err
	}
	if before != expectedBefore {
		return errors.New("current release changed after diff and confirmation; rerun the command")
	}
	if before.Version == target.Version {
		return errors.New("target release is already current")
	}
	installedRoot := filepath.Join(manager.StateHome, "versions", target.Version)
	_, statErr := os.Lstat(installedRoot)
	wasInstalled := statErr == nil
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	installed, err := releasebundle.Install(sourceRoot, installedRoot)
	if err != nil {
		return fmt.Errorf("install target release: %w", err)
	}
	if !reflect.DeepEqual(installed, target) {
		return rejectNewInstallation(installedRoot, wasInstalled, errors.New("release source changed after confirmation; current installation is unchanged"))
	}
	if err := inspectTargetExecutables(installedRoot, installed); err != nil {
		return rejectNewInstallation(installedRoot, wasInstalled, err)
	}
	if err := ensureFixedBootstrapMatchesTarget(manager.StateHome, installedRoot, installed); err != nil {
		return rejectNewInstallation(installedRoot, wasInstalled, err)
	}
	fromManifest, err := releasebundle.Inspect(filepath.Join(manager.StateHome, "versions", before.Version))
	if err != nil {
		return fmt.Errorf("verify current release: %w", err)
	}
	after := state.Current{Format: state.CurrentFormat, Version: target.Version, Previous: before.Version}
	journal := Journal{
		Operation:  operation,
		Phase:      "prepared",
		Before:     before,
		After:      after,
		From:       pluginRefFromManifest(fromManifest),
		To:         pluginRefFromManifest(installed),
		FromCommit: fromManifest.GitCommit,
		ToCommit:   installed.GitCommit,
	}
	if err := writeJournal(manager.StateHome, journal); err != nil {
		return err
	}
	if err := manager.afterEvent("prepared"); err != nil {
		return err
	}
	defer func() {
		if returnErr != nil && !errors.Is(returnErr, ErrSimulatedCrash) {
			if recoveryErr := manager.Recover(ctx); recoveryErr != nil {
				returnErr = fmt.Errorf("%w; automatic recovery failed: %v", returnErr, recoveryErr)
			}
		}
	}()
	if err := manager.Plugin.EnsureActive(ctx, journal.To); err != nil {
		return fmt.Errorf("activate target plugin: %w", err)
	}
	journal.Phase = "target_plugin_ready"
	if err := writeJournal(manager.StateHome, journal); err != nil {
		return err
	}
	if err := manager.afterEvent("target_plugin_ready"); err != nil {
		return err
	}
	if err := manager.Plugin.EnsureAbsent(ctx, journal.From); err != nil {
		return fmt.Errorf("deactivate previous plugin: %w", err)
	}
	journal.Phase = "old_plugin_removed"
	if err := writeJournal(manager.StateHome, journal); err != nil {
		return err
	}
	if err := manager.afterEvent("old_plugin_removed"); err != nil {
		return err
	}
	if err := state.WriteCurrent(manager.StateHome, after); err != nil {
		return err
	}
	journal.Phase = "current_committed"
	if err := writeJournal(manager.StateHome, journal); err != nil {
		return err
	}
	if err := manager.afterEvent("current_committed"); err != nil {
		return err
	}
	return clearJournal(manager.StateHome)
}

func rejectNewInstallation(root string, wasInstalled bool, cause error) error {
	if wasInstalled {
		return cause
	}
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("%w; remove rejected installation: %v", cause, err)
	}
	return cause
}

func (manager Manager) afterEvent(event string) error {
	if manager.AfterEvent == nil {
		return nil
	}
	return manager.AfterEvent(event)
}

func pluginRefFromManifest(manifest releasebundle.Manifest) PluginRef {
	return PluginRef{Name: manifest.Plugin.Name, Version: manifest.Plugin.Version, Marketplace: manifest.Plugin.Marketplace}
}

func (manager Manager) checkProjectCompatibility(target releasebundle.Manifest) error {
	projectRoot, err := manager.resolveProjectRoot()
	if err != nil {
		return err
	}
	profilePath := filepath.Join(projectRoot, ".sop", "profile.json")
	lockPath := filepath.Join(projectRoot, ".sop", "lock.json")
	profileExists, err := regularOrMissing(profilePath)
	if err != nil {
		return err
	}
	lockExists, err := regularOrMissing(lockPath)
	if err != nil {
		return err
	}
	if !profileExists && !lockExists {
		return nil
	}
	if profileExists != lockExists {
		return fmt.Errorf("project SOP state is incomplete; profile.json and lock.json must both exist before release rollback; run and confirm project rollback first")
	}
	profile, err := config.LoadProfile(profilePath)
	if err != nil {
		return err
	}
	if profile.SchemaVersion != target.Contract.ProfileSchemaVersion || profile.SOPVersion != target.Version {
		return fmt.Errorf("project is incompatible with release %s; run and confirm project rollback first", target.Version)
	}
	lock, err := readCompatibilityLock(lockPath)
	if err != nil {
		return err
	}
	profileData, err := json.Marshal(profile)
	if err != nil {
		return fmt.Errorf("encode profile for compatibility check: %w", err)
	}
	profileHash := fmt.Sprintf("%x", sha256.Sum256(profileData))
	if lock.SchemaVersion != supportedLockSchemaVersion ||
		lock.SOPVersion != target.Version ||
		lock.GeneratorVersion != target.Executables.Engine.Version ||
		lock.RulesVersion != target.Contract.RulesVersion ||
		lock.ProfileHash != profileHash {
		return fmt.Errorf("project lock is incompatible with release %s; run and confirm project rollback first", target.Version)
	}
	return nil
}

const supportedLockSchemaVersion = 1

type compatibilityLock struct {
	SchemaVersion    int             `json:"schema_version"`
	SOPVersion       string          `json:"sop_version"`
	GeneratorVersion string          `json:"generator_version"`
	RulesVersion     string          `json:"rules_version"`
	ProfileHash      string          `json:"profile_hash"`
	Outputs          json.RawMessage `json:"outputs"`
}

func readCompatibilityLock(path string) (compatibilityLock, error) {
	file, err := os.Open(path)
	if err != nil {
		return compatibilityLock{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var lock compatibilityLock
	if err := decoder.Decode(&lock); err != nil {
		return compatibilityLock{}, fmt.Errorf("parse .sop/lock.json: %w", err)
	}
	if err := ensureDescriptionEOF(decoder); err != nil {
		return compatibilityLock{}, fmt.Errorf("parse .sop/lock.json: %w", err)
	}
	profileHash, hashErr := hex.DecodeString(lock.ProfileHash)
	var outputs []json.RawMessage
	outputsErr := json.Unmarshal(lock.Outputs, &outputs)
	if lock.Outputs == nil || outputsErr != nil || outputs == nil || lock.GeneratorVersion == "" || lock.RulesVersion == "" || hashErr != nil || len(profileHash) != sha256.Size {
		return compatibilityLock{}, errors.New("parse .sop/lock.json: required compatibility fields are missing")
	}
	return lock, nil
}

func (manager Manager) resolveProjectRoot() (string, error) {
	if manager.ProjectRoot != "" {
		return filepath.Abs(manager.ProjectRoot)
	}
	start, err := os.Getwd()
	if err != nil {
		return "", err
	}
	current := start
	for {
		profileExists, profileErr := pathExists(filepath.Join(current, ".sop", "profile.json"))
		if profileErr != nil {
			return "", profileErr
		}
		lockExists, lockErr := pathExists(filepath.Join(current, ".sop", "lock.json"))
		if lockErr != nil {
			return "", lockErr
		}
		if profileExists || lockExists {
			return current, nil
		}
		gitExists, gitErr := pathExists(filepath.Join(current, ".git"))
		if gitErr != nil {
			return "", gitErr
		}
		if gitExists {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return start, nil
		}
		current = parent
	}
}

func regularOrMissing(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("project compatibility file must be regular: %s", path)
	}
	return true, nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (manager Manager) forwardEngine(args []string) error {
	current, err := state.ReadCurrent(manager.StateHome)
	if err != nil {
		return err
	}
	versionRoot := filepath.Join(manager.StateHome, "versions", current.Version)
	enginePath := filepath.Join(versionRoot, "bin", platform.ExecutableName("sopctl-engine"))
	info, err := os.Lstat(enginePath)
	if err != nil {
		return fmt.Errorf("inspect active engine: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("active engine must be a regular file")
	}
	command := exec.Command(enginePath, args...)
	command.Stdin = manager.Stdin
	command.Stdout = manager.stdout()
	command.Stderr = manager.stderr()
	command.Env = append(
		os.Environ(),
		"SOP_STATE_HOME="+manager.StateHome,
		"SOP_RELEASE_VERSION="+current.Version,
		"SOP_ASSET_ROOT="+filepath.Join(versionRoot, "assets"),
	)
	if err := command.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return ProcessExit{Code: exitError.ExitCode()}
		}
		return fmt.Errorf("run active engine: %w", err)
	}
	return nil
}

func (manager Manager) releaseCheck() error {
	current, err := state.ReadCurrent(manager.StateHome)
	if err != nil {
		return err
	}
	currentRoot := filepath.Join(manager.StateHome, "versions", current.Version)
	currentManifest, err := releasebundle.Inspect(currentRoot)
	if err != nil {
		return fmt.Errorf("current release %s is damaged: %w; reinstall this exact version before continuing", current.Version, err)
	}
	if currentManifest.Version != current.Version {
		return errors.New("current release directory identity does not match current.json")
	}
	if err := validateTargetRuntime(currentManifest); err != nil {
		return fmt.Errorf("current release runtime is incompatible: %w", err)
	}
	if err := inspectTargetExecutables(currentRoot, currentManifest); err != nil {
		return fmt.Errorf("current release runtime health check failed: %w", err)
	}
	bootstrapMissing, err := fixedBootstrapNeedsCreate(manager.StateHome, currentRoot, currentManifest)
	if err != nil {
		bootstrapPath := filepath.Join(manager.StateHome, "bin", platform.ExecutableName("sopctl"))
		return fmt.Errorf("fixed bootstrap health check failed: %w; the installer will not overwrite an unproven file: if %s is the damaged managed bootstrap, move it aside and rerun sop-install with this exact version", err, bootstrapPath)
	}
	if bootstrapMissing {
		return errors.New("fixed bootstrap is missing; rerun sop-install with this exact version")
	}
	pluginHealth, ok := manager.Plugin.(PluginHealthController)
	if !ok {
		return errors.New("read-only plugin health controller is required for release check")
	}
	if err := pluginHealth.CheckActive(context.Background(), pluginRefFromManifest(currentManifest)); err != nil {
		return fmt.Errorf("current plugin health check failed: %w; rerun sop-install with this exact version", err)
	}
	fmt.Fprintf(manager.stdout(), "current %s\nCURRENT_RELEASE verified %s\nCURRENT_RUNTIME manager+engine protocol %d\nPLUGIN_HEALTH active %s\n", current.Version, currentManifest.GitCommit, managerEngineProtocol, currentManifest.Plugin.Version)
	if err := manager.writeCurrentProjectHealth(currentRoot, currentManifest); err != nil {
		return err
	}
	versions, err := manager.availableVersions()
	if err != nil {
		return err
	}
	latest := current.Version
	if len(versions) > 0 {
		candidate := versions[len(versions)-1]
		if compareVersions(candidate, current.Version) > 0 {
			latest = candidate
			root, err := manager.bundleRoot(latest)
			if err != nil {
				return err
			}
			manifest, err := releasebundle.Inspect(root)
			if err != nil {
				return fmt.Errorf("verify available release %s: %w", latest, err)
			}
			if err := validateTargetRuntime(manifest); err != nil {
				return fmt.Errorf("available release %s: %w", latest, err)
			}
			if err := inspectTargetExecutables(root, manifest); err != nil {
				return fmt.Errorf("available release %s runtime health check: %w", latest, err)
			}
		}
	}
	fmt.Fprintf(manager.stdout(), "available %s\n", latest)
	return nil
}

func (manager Manager) writeCurrentProjectHealth(versionRoot string, manifest releasebundle.Manifest) (returnErr error) {
	projectRoot, err := manager.resolveProjectRoot()
	if err != nil {
		return fmt.Errorf("resolve project for release health check: %w", err)
	}
	profileExists, err := regularOrMissing(filepath.Join(projectRoot, ".sop", "profile.json"))
	if err != nil {
		return err
	}
	lockExists, err := regularOrMissing(filepath.Join(projectRoot, ".sop", "lock.json"))
	if err != nil {
		return err
	}
	if !profileExists && !lockExists {
		fmt.Fprintf(manager.stdout(), "PROJECT_SOP none %s\n", projectRoot)
		return nil
	}
	if profileExists != lockExists {
		return errors.New("current project SOP state is incomplete; profile.json and lock.json must both exist; recover or render the project before release changes")
	}
	healthManager := manager
	healthManager.ProjectRoot = projectRoot
	if err := healthManager.checkProjectCompatibility(manifest); err != nil {
		return fmt.Errorf("current project is not compatible with release %s: %w; restore matching project state or choose its release", manifest.Version, err)
	}
	projectLock, err := manager.acquireProjectOperationLock(projectRoot)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := projectLock.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("release project health lock: %w", closeErr)
		}
	}()
	if err := healthManager.checkProjectCompatibility(manifest); err != nil {
		return fmt.Errorf("current project changed during release health check: %w", err)
	}
	if err := healthManager.checkProjectWithTargetEngine(versionRoot, manifest); err != nil {
		return fmt.Errorf("current project managed outputs are unhealthy: %w; repair or restore the project before release changes", err)
	}
	profile, err := config.LoadProfile(filepath.Join(projectRoot, ".sop", "profile.json"))
	if err != nil {
		return err
	}
	fmt.Fprintf(manager.stdout(), "PROJECT_SOP %s PROFILE_SCHEMA %d COMPATIBLE verified %s\n", profile.SOPVersion, profile.SchemaVersion, projectRoot)
	return nil
}

func (manager Manager) releaseDiff(args []string) error {
	flags := flag.NewFlagSet("release diff", flag.ContinueOnError)
	flags.SetOutput(manager.stderr())
	target := flags.String("to", "", "target release version")
	projectRoot := flags.String("project-root", manager.ProjectRoot, "project root for target-version read-only preview")
	profile := flags.String("profile", "", "target-compatible candidate profile for read-only preview")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return errors.New("release diff: --to <version> is required")
	}
	previewManager := manager
	previewManager.ProjectRoot = *projectRoot
	_, _, err := previewManager.writeDiff(*target, *profile)
	var incompatible incompatibleOutputTargetsError
	if errors.As(err, &incompatible) {
		return nil
	}
	return err
}

func (manager Manager) writeDiff(target, profilePath string) (releasebundle.Manifest, state.Current, error) {
	if !state.ValidVersion(target) {
		return releasebundle.Manifest{}, state.Current{}, errors.New("target version must be strict semver")
	}
	bundleRoot, err := manager.bundleRoot(target)
	if err != nil {
		return releasebundle.Manifest{}, state.Current{}, err
	}
	return manager.writeManifestDiff(target, bundleRoot, profilePath, true)
}

func (manager Manager) writeManifestDiff(target, bundleRoot, profilePath string, previewProject bool) (releasebundle.Manifest, state.Current, error) {
	current, err := state.ReadCurrent(manager.StateHome)
	if err != nil {
		return releasebundle.Manifest{}, state.Current{}, err
	}
	manifest, err := releasebundle.Inspect(bundleRoot)
	if err != nil {
		return releasebundle.Manifest{}, state.Current{}, fmt.Errorf("verify release %s: %w", target, err)
	}
	if manifest.Version != target {
		return releasebundle.Manifest{}, state.Current{}, fmt.Errorf("release directory %s contains version %s", target, manifest.Version)
	}
	if err := validateTargetRuntime(manifest); err != nil {
		return releasebundle.Manifest{}, state.Current{}, err
	}
	if err := ensureFixedBootstrapMatchesTarget(manager.StateHome, bundleRoot, manifest); err != nil {
		return releasebundle.Manifest{}, state.Current{}, err
	}
	currentRoot := filepath.Join(manager.StateHome, "versions", current.Version)
	currentManifest, err := releasebundle.Inspect(currentRoot)
	if err != nil {
		return releasebundle.Manifest{}, state.Current{}, fmt.Errorf("verify current release: %w", err)
	}
	differences, err := releasebundle.Compare(currentRoot, bundleRoot)
	if err != nil {
		return releasebundle.Manifest{}, state.Current{}, err
	}
	fmt.Fprintf(
		manager.stdout(),
		"CURRENT %s\nTARGET %s\nPLUGIN %s %s\nRULES %s\nPROFILE_SCHEMA %d\n",
		current.Version,
		manifest.Version,
		manifest.Plugin.Name,
		manifest.Plugin.Version,
		manifest.Contract.RulesVersion,
		manifest.Contract.ProfileSchemaVersion,
	)
	fmt.Fprintf(manager.stdout(), "PLUGIN_CHANGE %s %s -> %s\n", manifest.Plugin.Name, currentManifest.Plugin.Version, manifest.Plugin.Version)
	fmt.Fprintf(manager.stdout(), "RELEASE_NOTES\n  %s\n", indentReleaseText(manifest.ReleaseNotes))
	fmt.Fprintf(manager.stdout(), "UPGRADE_IMPACT\n  %s\n", indentReleaseText(manifest.UpgradeImpact))
	manager.writeFileDifferences(differences)
	if previewProject {
		moves, err := releasebundle.OutputTargetMoves(filepath.Join(manager.StateHome, "versions", current.Version), bundleRoot)
		if err != nil {
			return releasebundle.Manifest{}, state.Current{}, err
		}
		if len(moves) > 0 {
			for _, move := range moves {
				fmt.Fprintf(manager.stdout(), "INCOMPATIBLE output %s target %s -> %s\n", move.ID, move.CurrentTarget, move.TargetTarget)
			}
			return manifest, current, incompatibleOutputTargetsError{moves: moves}
		}
		if err := manager.writeProjectDiff(bundleRoot, manifest, profilePath); err != nil {
			return releasebundle.Manifest{}, state.Current{}, err
		}
	}
	return manifest, current, nil
}

func indentReleaseText(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "\n", "\n  ")
}

func (manager Manager) writeFileDifferences(differences []releasebundle.FileDifference) {
	fmt.Fprintln(manager.stdout(), "FILES")
	for _, difference := range differences {
		switch difference.Status {
		case "ADD":
			fmt.Fprintf(manager.stdout(), "ADD %s %s\n", difference.Path, difference.TargetSHA256)
		case "DELETE":
			fmt.Fprintf(manager.stdout(), "DELETE %s %s\n", difference.Path, difference.CurrentSHA256)
		default:
			fmt.Fprintf(manager.stdout(), "UPDATE %s %s -> %s\n", difference.Path, difference.CurrentSHA256, difference.TargetSHA256)
		}
	}
	if len(differences) == 0 {
		fmt.Fprintln(manager.stdout(), "NO_FILE_CHANGES")
	}
	for _, scope := range []struct {
		name   string
		prefix string
	}{
		{name: "plugin-skills", prefix: "marketplace/plugins/sop-better/skills/"},
		{name: "rules", prefix: "marketplace/plugins/sop-better/rules/"},
		{name: "master", prefix: "assets/master/"},
		{name: "schemas", prefix: "assets/schemas/"},
		{name: "manifest", prefix: "assets/manifest.json"},
	} {
		status := "unchanged"
		for _, difference := range differences {
			if strings.HasPrefix(difference.Path, scope.prefix) {
				status = "changed"
				break
			}
		}
		fmt.Fprintf(manager.stdout(), "SCOPE %s %s\n", scope.name, status)
	}
}

func (manager Manager) writeProjectDiff(bundleRoot string, target releasebundle.Manifest, profilePath string) (returnErr error) {
	projectRoot, err := manager.resolveProjectRoot()
	if err != nil {
		fmt.Fprintf(manager.stdout(), "PROJECT_DIFF unavailable: %v; release switch does not modify project files\n", err)
		return nil
	}
	if profilePath == "" {
		profilePath = filepath.Join(projectRoot, ".sop", "profile.json")
	}
	absoluteProfile, err := filepath.Abs(profilePath)
	if err != nil {
		fmt.Fprintf(manager.stdout(), "PROJECT_DIFF unavailable: resolve candidate profile: %v; release switch does not modify project files\n", err)
		return nil
	}
	info, err := os.Lstat(absoluteProfile)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(manager.stdout(), "PROJECT_DIFF unavailable: no candidate profile at %s; release switch does not modify project files. Prepare a %s profile and rerun with --project-root %s --profile PATH.\n", absoluteProfile, target.Version, projectRoot)
		return nil
	}
	if err != nil {
		fmt.Fprintf(manager.stdout(), "PROJECT_DIFF unavailable: inspect candidate profile: %v; release switch does not modify project files\n", err)
		return nil
	}
	if !info.Mode().IsRegular() {
		fmt.Fprintf(manager.stdout(), "PROJECT_DIFF unavailable: candidate profile must be a regular file: %s; release switch does not modify project files\n", absoluteProfile)
		return nil
	}
	profile, err := config.LoadProfile(absoluteProfile)
	if err != nil {
		fmt.Fprintf(manager.stdout(), "PROJECT_DIFF unavailable: %v; release switch does not modify project files\n", err)
		return nil
	}
	if profile.SchemaVersion != target.Contract.ProfileSchemaVersion || profile.SOPVersion != target.Version {
		fmt.Fprintf(manager.stdout(), "PROJECT_DIFF unavailable: candidate profile is schema %d / SOP %s, target requires schema %d / SOP %s; release switch does not modify project files. Prepare a target-compatible profile and rerun with --profile PATH.\n", profile.SchemaVersion, profile.SOPVersion, target.Contract.ProfileSchemaVersion, target.Version)
		return nil
	}

	previewRoot, err := os.MkdirTemp("", ".sop-release-preview-")
	if err != nil {
		return fmt.Errorf("create target-engine preview workspace: %w", err)
	}
	defer os.RemoveAll(previewRoot)
	installedRoot := filepath.Join(previewRoot, "versions", target.Version)
	installed, err := releasebundle.Install(bundleRoot, installedRoot)
	if err != nil {
		return fmt.Errorf("stage verified target engine for project diff: %w", err)
	}
	if !reflect.DeepEqual(installed, target) {
		return errors.New("release source changed while preparing target-engine project diff")
	}
	if err := inspectTargetExecutables(installedRoot, installed); err != nil {
		return fmt.Errorf("target-engine project diff preflight: %w", err)
	}
	projectID, err := projectid.Identifier(projectRoot)
	if err != nil {
		return err
	}
	projectLock, err := manager.acquireProjectOperationLock(projectRoot)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := projectLock.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("release project preview lock: %w", closeErr)
		}
	}()
	enginePath := filepath.Join(installedRoot, filepath.FromSlash(installed.Executables.Engine.Path))
	command := exec.Command(enginePath, "diff", "--project-root", projectRoot, "--profile", absoluteProfile)
	var output bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &output
	command.Stderr = &stderr
	command.Env = environmentWithOverrides(os.Environ(), map[string]string{
		"SOP_STATE_HOME":        manager.StateHome,
		"SOP_RELEASE_VERSION":   target.Version,
		"SOP_ASSET_ROOT":        filepath.Join(installedRoot, "assets"),
		"SOP_PROJECT_LOCK_HELD": projectID,
	})
	if err := command.Run(); err != nil {
		fmt.Fprintf(manager.stdout(), "PROJECT_DIFF failed using verified target engine %s: %s\n", target.Version, strings.TrimSpace(stderr.String()))
		return fmt.Errorf("target-engine project diff: %w", err)
	}
	fmt.Fprintf(manager.stdout(), "PROJECT_DIFF verified target engine %s\n", target.Version)
	if _, err := manager.stdout().Write(output.Bytes()); err != nil {
		return err
	}
	if output.Len() > 0 && output.Bytes()[output.Len()-1] != '\n' {
		fmt.Fprintln(manager.stdout())
	}
	return nil
}

func environmentWithOverrides(base []string, overrides map[string]string) []string {
	environment := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, found := strings.Cut(entry, "=")
		if _, overridden := overrides[key]; found && overridden {
			continue
		}
		environment = append(environment, entry)
	}
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		environment = append(environment, key+"="+overrides[key])
	}
	return environment
}

func (manager Manager) stdout() io.Writer {
	if manager.Stdout == nil {
		return io.Discard
	}
	return manager.Stdout
}

func (manager Manager) stderr() io.Writer {
	if manager.Stderr == nil {
		return io.Discard
	}
	return manager.Stderr
}

func (manager Manager) writeRestartNotice(version string) {
	fmt.Fprintf(
		manager.stdout(),
		"Release %s is active. Do not run project commands in this Codex session; close it and start a new Codex session first.\n",
		version,
	)
}

func (manager Manager) validateBootstrapHandoff() error {
	if BuildVersion == "0.1.0-dev" {
		return nil
	}
	current, err := state.ReadCurrent(manager.StateHome)
	if err != nil {
		return err
	}
	if current.Version != BuildVersion {
		return fmt.Errorf("bootstrap/manager version mismatch: current is %s but the launched manager is %s; retry the command", current.Version, BuildVersion)
	}
	return nil
}

func validateTargetRuntime(manifest releasebundle.Manifest) error {
	if manifest.Platform == nil || manifest.Executables == nil {
		return errors.New("release is not installable: versioned manager and engine metadata are required")
	}
	if manifest.Platform.OS != runtime.GOOS || manifest.Platform.Arch != runtime.GOARCH {
		return fmt.Errorf("release platform %s/%s does not match this machine %s/%s", manifest.Platform.OS, manifest.Platform.Arch, runtime.GOOS, runtime.GOARCH)
	}
	return nil
}

func ensureFixedBootstrapMatchesTarget(stateHome, targetRoot string, manifest releasebundle.Manifest) error {
	if manifest.Executables == nil || manifest.Executables.Bootstrap.Protocol != 1 {
		return errors.New("target release uses an unsupported fixed bootstrap protocol")
	}
	missing, err := fixedBootstrapNeedsCreate(stateHome, targetRoot, manifest)
	if err != nil {
		return fmt.Errorf("target release bootstrap is incompatible with the installed fixed bootstrap: %w", err)
	}
	if missing {
		return errors.New("installed fixed bootstrap is missing; rerun sop-install with the current exact version before changing releases")
	}
	return nil
}

func inspectTargetExecutables(root string, manifest releasebundle.Manifest) error {
	for component, executable := range map[string]releasebundle.Executable{
		"manager": manifest.Executables.Manager,
		"engine":  manifest.Executables.Engine,
	} {
		path := filepath.Join(root, filepath.FromSlash(executable.Path))
		command := exec.Command(path, "__describe", "--json")
		command.Env = append(
			os.Environ(),
			"SOP_RELEASE_VERSION="+manifest.Version,
			"SOP_ASSET_ROOT="+filepath.Join(root, "assets"),
		)
		output, err := command.CombinedOutput()
		if err != nil {
			return fmt.Errorf("preflight %s executable: %w: %s", component, err, strings.TrimSpace(string(output)))
		}
		decoder := json.NewDecoder(bytes.NewReader(output))
		decoder.DisallowUnknownFields()
		var description runtimeDescription
		if err := decoder.Decode(&description); err != nil {
			return fmt.Errorf("parse %s executable description: %w", component, err)
		}
		if err := ensureDescriptionEOF(decoder); err != nil {
			return fmt.Errorf("parse %s executable description: %w", component, err)
		}
		if description.Component != component || description.Version != manifest.Version || description.Protocol != managerEngineProtocol {
			return fmt.Errorf("%s executable handshake mismatch: component=%s version=%s protocol=%d", component, description.Component, description.Version, description.Protocol)
		}
	}
	return nil
}

func ensureDescriptionEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("multiple JSON values are not allowed")
}

func (manager Manager) availableVersions() ([]string, error) {
	sourceRoot, err := manager.releaseSourceRoot()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		return nil, fmt.Errorf("read release source: %w", err)
	}
	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && state.ValidVersion(entry.Name()) {
			versions = append(versions, entry.Name())
		}
	}
	sort.Slice(versions, func(i, j int) bool { return compareVersions(versions[i], versions[j]) < 0 })
	return versions, nil
}

func (manager Manager) bundleRoot(version string) (string, error) {
	sourceRoot, err := manager.releaseSourceRoot()
	if err != nil {
		return "", err
	}
	root := filepath.Join(sourceRoot, version)
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("find release %s: %w", version, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("release %s is not a directory", version)
	}
	return root, nil
}

func compareVersions(left, right string) int {
	leftWithoutBuild := strings.SplitN(left, "+", 2)[0]
	rightWithoutBuild := strings.SplitN(right, "+", 2)[0]
	leftVersion := strings.SplitN(leftWithoutBuild, "-", 2)
	rightVersion := strings.SplitN(rightWithoutBuild, "-", 2)
	leftParts := strings.Split(leftVersion[0], ".")
	rightParts := strings.Split(rightVersion[0], ".")
	for index := 0; index < 3; index++ {
		if comparison := compareNumericStrings(leftParts[index], rightParts[index]); comparison != 0 {
			return comparison
		}
	}
	leftPrerelease := ""
	if len(leftVersion) == 2 {
		leftPrerelease = leftVersion[1]
	}
	rightPrerelease := ""
	if len(rightVersion) == 2 {
		rightPrerelease = rightVersion[1]
	}
	if leftPrerelease == rightPrerelease {
		return 0
	}
	if leftPrerelease == "" {
		return 1
	}
	if rightPrerelease == "" {
		return -1
	}
	leftIdentifiers := strings.Split(leftPrerelease, ".")
	rightIdentifiers := strings.Split(rightPrerelease, ".")
	for index := 0; index < len(leftIdentifiers) && index < len(rightIdentifiers); index++ {
		leftIdentifier := leftIdentifiers[index]
		rightIdentifier := rightIdentifiers[index]
		if leftIdentifier == rightIdentifier {
			continue
		}
		leftNumeric := numericIdentifier(leftIdentifier)
		rightNumeric := numericIdentifier(rightIdentifier)
		switch {
		case leftNumeric && rightNumeric:
			return compareNumericStrings(leftIdentifier, rightIdentifier)
		case leftNumeric:
			return -1
		case rightNumeric:
			return 1
		default:
			return strings.Compare(leftIdentifier, rightIdentifier)
		}
	}
	if len(leftIdentifiers) < len(rightIdentifiers) {
		return -1
	}
	return 1
}

func numericIdentifier(identifier string) bool {
	for _, character := range identifier {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func compareNumericStrings(left, right string) int {
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return strings.Compare(left, right)
}
