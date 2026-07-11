package releasemanager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/Strelizialeomon/sop-better/internal/platform"
	"github.com/Strelizialeomon/sop-better/internal/releasebundle"
	"github.com/Strelizialeomon/sop-better/internal/state"
)

const initialInstallJournalFormat = 1

type InitialInstaller struct {
	StateHome     string
	BundleRoot    string
	ReleaseSource string
	Plugin        PluginController
	Confirmer     Confirmer
	Stdout        io.Writer
	Stderr        io.Writer
	AfterEvent    func(string) error
}

type initialInstallJournal struct {
	Format            int       `json:"format"`
	Phase             string    `json:"phase"`
	Version           string    `json:"version"`
	GitCommit         string    `json:"git_commit"`
	ReleaseSHA256     string    `json:"release_sha256"`
	Plugin            PluginRef `json:"plugin"`
	BootstrapSHA256   string    `json:"bootstrap_sha256"`
	ReleaseSourceRoot string    `json:"release_source_root"`
	VersionCreated    bool      `json:"version_created"`
	BootstrapCreated  bool      `json:"bootstrap_created"`
	SourceCreated     bool      `json:"source_created"`
}

func (installer InitialInstaller) Install(ctx context.Context) (returnErr error) {
	if strings.TrimSpace(installer.StateHome) == "" {
		return errors.New("state home is required")
	}
	if strings.TrimSpace(installer.BundleRoot) == "" {
		return errors.New("bundle root is required")
	}
	if installer.Plugin == nil {
		return errors.New("plugin controller is required for initial install")
	}
	manifest, err := releasebundle.Inspect(installer.BundleRoot)
	if err != nil {
		return fmt.Errorf("verify install bundle: %w", err)
	}
	if err := validateTargetRuntime(manifest); err != nil {
		return err
	}
	releaseSource := installer.ReleaseSource
	if releaseSource == "" {
		bundleRoot, resolveErr := filepath.Abs(installer.BundleRoot)
		if resolveErr != nil {
			return fmt.Errorf("resolve bundle root for release source: %w", resolveErr)
		}
		releaseSource = filepath.Dir(bundleRoot)
	}
	releaseSource, err = normalizeLocalReleaseSource(releaseSource)
	if err != nil {
		return err
	}
	if err := installer.writePreview(manifest, releaseSource); err != nil {
		return err
	}
	if installer.Confirmer == nil {
		return errors.New("interactive confirmation is required")
	}
	confirmed, err := installer.Confirmer.Confirm(ctx, manifest.Version)
	if err != nil {
		return err
	}
	if !confirmed {
		return errors.New("initial install cancelled; no installation state was changed")
	}

	lock, err := state.AcquireLock(installer.StateHome)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := lock.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("release initial-install lock: %w", closeErr)
		}
	}()
	if err := installer.recover(ctx); err != nil {
		return fmt.Errorf("recover pending initial install: %w", err)
	}
	if current, exists, err := readCurrentIfExists(installer.StateHome); err != nil {
		return err
	} else if exists {
		if current.Version == manifest.Version {
			installedRoot := filepath.Join(installer.StateHome, "versions", current.Version)
			if _, err := fixedBootstrapNeedsCreate(installer.StateHome, installer.BundleRoot, manifest); err != nil {
				bootstrapPath := filepath.Join(installer.StateHome, "bin", platform.ExecutableName("sopctl"))
				return fmt.Errorf("preflight fixed bootstrap reconciliation: %w; the installer will not overwrite an unproven file: if %s is the damaged managed bootstrap, move it aside and rerun this exact-version installer", err, bootstrapPath)
			}
			if _, err := releaseSourceNeedsCreate(installer.StateHome, releaseSource); err != nil {
				return err
			}
			installed, repairErr := releasebundle.RepairInstalled(installer.BundleRoot, installedRoot)
			if repairErr != nil {
				return fmt.Errorf("repair existing current release from verified exact-version bundle: %w", repairErr)
			}
			installedIdentity, installedIdentityErr := fileSHA256(filepath.Join(installedRoot, "release.json"))
			if installedIdentityErr != nil {
				return installedIdentityErr
			}
			sourceIdentity, sourceIdentityErr := fileSHA256(filepath.Join(installer.BundleRoot, "release.json"))
			if sourceIdentityErr != nil {
				return sourceIdentityErr
			}
			if installed.GitCommit != manifest.GitCommit || installedIdentity != sourceIdentity {
				return errors.New("current version has a different release identity")
			}
			if err := inspectTargetExecutables(installedRoot, installed); err != nil {
				return fmt.Errorf("verify existing current runtime: %w", err)
			}
			if err := installer.Plugin.EnsureActive(ctx, pluginRefFromManifest(installed)); err != nil {
				return fmt.Errorf("reconcile installed plugin: %w", err)
			}
			if _, err := installFixedBootstrap(installer.StateHome, installedRoot, installed); err != nil {
				return fmt.Errorf("reconcile fixed bootstrap: %w", err)
			}
			if _, err := installReleaseSource(installer.StateHome, releaseSource); err != nil {
				return err
			}
			fmt.Fprintf(installer.stdout(), "Release %s is already installed and its runtime, plugin, bootstrap, and release source are healthy.\n", current.Version)
			installer.writeUsageNotice()
			return nil
		}
		return fmt.Errorf("release %s is already installed; use sopctl release upgrade", current.Version)
	}

	installedRoot := filepath.Join(installer.StateHome, "versions", manifest.Version)
	_, statErr := os.Lstat(installedRoot)
	versionCreated := errors.Is(statErr, os.ErrNotExist)
	if statErr != nil && !versionCreated {
		return statErr
	}
	installed, err := releasebundle.Install(installer.BundleRoot, installedRoot)
	if err != nil {
		return fmt.Errorf("install verified release: %w", err)
	}
	if !reflect.DeepEqual(installed, manifest) {
		return rejectNewInstallation(installedRoot, !versionCreated, errors.New("release source changed after confirmation; no installation was committed"))
	}
	if err := inspectTargetExecutables(installedRoot, installed); err != nil {
		return rejectNewInstallation(installedRoot, !versionCreated, err)
	}
	releaseIdentity, err := fileSHA256(filepath.Join(installedRoot, "release.json"))
	if err != nil {
		return rejectNewInstallation(installedRoot, !versionCreated, err)
	}
	journal := initialInstallJournal{
		Phase: "prepared", Version: installed.Version, GitCommit: installed.GitCommit,
		ReleaseSHA256: releaseIdentity, Plugin: pluginRefFromManifest(installed),
		BootstrapSHA256:   installed.Executables.Bootstrap.SHA256,
		ReleaseSourceRoot: releaseSource,
		VersionCreated:    versionCreated,
	}
	if err := writeInitialInstallJournal(installer.StateHome, journal); err != nil {
		return rejectNewInstallation(installedRoot, !versionCreated, err)
	}
	if err := installer.afterEvent("prepared"); err != nil {
		return err
	}
	defer func() {
		if returnErr != nil && !errors.Is(returnErr, ErrSimulatedCrash) {
			if recoveryErr := installer.recover(ctx); recoveryErr != nil {
				returnErr = fmt.Errorf("%w; automatic initial-install recovery failed: %v", returnErr, recoveryErr)
			}
		}
	}()

	if err := installer.Plugin.EnsureActive(ctx, journal.Plugin); err != nil {
		return fmt.Errorf("activate target plugin: %w", err)
	}
	journal.Phase = "target_plugin_ready"
	if err := writeInitialInstallJournal(installer.StateHome, journal); err != nil {
		return err
	}
	if err := installer.afterEvent("target_plugin_ready"); err != nil {
		return err
	}

	bootstrapCreated, err := fixedBootstrapNeedsCreate(installer.StateHome, installedRoot, installed)
	if err != nil {
		return err
	}
	journal.BootstrapCreated = bootstrapCreated
	journal.Phase = "bootstrap_installing"
	if err := writeInitialInstallJournal(installer.StateHome, journal); err != nil {
		return err
	}
	created, err := installFixedBootstrap(installer.StateHome, installedRoot, installed)
	if err != nil {
		return err
	}
	if created != bootstrapCreated {
		return errors.New("fixed bootstrap state changed during initial install")
	}
	journal.Phase = "bootstrap_ready"
	if err := writeInitialInstallJournal(installer.StateHome, journal); err != nil {
		return err
	}
	if err := installer.afterEvent("bootstrap_ready"); err != nil {
		return err
	}

	sourceCreated, err := releaseSourceNeedsCreate(installer.StateHome, releaseSource)
	if err != nil {
		return err
	}
	journal.SourceCreated = sourceCreated
	journal.Phase = "source_installing"
	if err := writeInitialInstallJournal(installer.StateHome, journal); err != nil {
		return err
	}
	createdSource, err := installReleaseSource(installer.StateHome, releaseSource)
	if err != nil {
		return err
	}
	if createdSource != sourceCreated {
		return errors.New("release source configuration changed during initial install")
	}
	journal.Phase = "source_ready"
	if err := writeInitialInstallJournal(installer.StateHome, journal); err != nil {
		return err
	}
	if err := installer.afterEvent("source_ready"); err != nil {
		return err
	}

	if err := state.WriteCurrent(installer.StateHome, state.Current{Format: state.CurrentFormat, Version: installed.Version}); err != nil {
		return err
	}
	journal.Phase = "current_committed"
	if err := writeInitialInstallJournal(installer.StateHome, journal); err != nil {
		return err
	}
	if err := installer.afterEvent("current_committed"); err != nil {
		return err
	}
	if err := clearInitialInstallJournal(installer.StateHome); err != nil {
		return err
	}
	fmt.Fprintf(installer.stdout(), "Release %s installed.\n", installed.Version)
	installer.writeUsageNotice()
	return nil
}

func (installer InitialInstaller) writeUsageNotice() {
	bootstrapPath := filepath.Join(installer.StateHome, "bin", platform.ExecutableName("sopctl"))
	fmt.Fprintf(
		installer.stdout(),
		"Start a new Codex session, then run: %s (or persist SOP_STATE_HOME and add %s to PATH).\n",
		formatStateHomeCommand(runtime.GOOS, installer.StateHome, bootstrapPath),
		filepath.Join(installer.StateHome, "bin"),
	)
}

func formatStateHomeCommand(goos, stateHome, bootstrapPath string) string {
	if goos == "windows" {
		return fmt.Sprintf("$env:SOP_STATE_HOME = %s; & %s release check", quotePowerShell(stateHome), quotePowerShell(bootstrapPath))
	}
	return fmt.Sprintf("SOP_STATE_HOME=%s %s release check", quotePOSIXShell(stateHome), quotePOSIXShell(bootstrapPath))
}

func quotePOSIXShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func quotePowerShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func (installer InitialInstaller) recover(ctx context.Context) error {
	journal, exists, err := readInitialInstallJournal(installer.StateHome)
	if err != nil || !exists {
		return err
	}
	if installer.Plugin == nil {
		return errors.New("plugin controller is required for initial-install recovery")
	}
	installedRoot := filepath.Join(installer.StateHome, "versions", journal.Version)
	manifest, err := releasebundle.Inspect(installedRoot)
	if err != nil {
		return fmt.Errorf("verify initial-install release pin: %w", err)
	}
	releaseIdentity, err := fileSHA256(filepath.Join(installedRoot, "release.json"))
	if err != nil {
		return err
	}
	if manifest.GitCommit != journal.GitCommit || releaseIdentity != journal.ReleaseSHA256 || pluginRefFromManifest(manifest) != journal.Plugin || manifest.Executables.Bootstrap.SHA256 != journal.BootstrapSHA256 {
		return errors.New("initial-install release pin differs from the transaction journal")
	}
	current, currentExists, err := readCurrentIfExists(installer.StateHome)
	if err != nil {
		return err
	}
	if currentExists {
		if current.Version != journal.Version || current.Previous != "" {
			return errors.New("current.json conflicts with the pending initial-install journal")
		}
		if _, err := installFixedBootstrap(installer.StateHome, installedRoot, manifest); err != nil {
			return err
		}
		if _, err := installReleaseSource(installer.StateHome, journal.ReleaseSourceRoot); err != nil {
			return err
		}
		if err := installer.Plugin.EnsureActive(ctx, journal.Plugin); err != nil {
			return fmt.Errorf("restore installed plugin: %w", err)
		}
		return clearInitialInstallJournal(installer.StateHome)
	}

	if err := installer.Plugin.EnsureAbsent(ctx, journal.Plugin); err != nil {
		return fmt.Errorf("remove target plugin during initial-install recovery: %w", err)
	}
	if journal.BootstrapCreated {
		bootstrapPath := filepath.Join(installer.StateHome, "bin", platform.ExecutableName("sopctl"))
		digest, digestErr := fileSHA256(bootstrapPath)
		if digestErr != nil && !errors.Is(digestErr, os.ErrNotExist) {
			return digestErr
		}
		if digestErr == nil {
			if digest != journal.BootstrapSHA256 {
				return errors.New("created fixed bootstrap changed during initial-install recovery")
			}
			if err := os.Remove(bootstrapPath); err != nil {
				return err
			}
		}
	}
	if journal.SourceCreated && (journal.Phase == "source_installing" || journal.Phase == "source_ready" || journal.Phase == "current_committed") {
		if err := removeCreatedReleaseSource(installer.StateHome, journal.ReleaseSourceRoot); err != nil {
			return err
		}
	}
	if journal.VersionCreated {
		if err := os.RemoveAll(installedRoot); err != nil {
			return err
		}
	}
	return clearInitialInstallJournal(installer.StateHome)
}

func (installer InitialInstaller) writePreview(manifest releasebundle.Manifest, releaseSource string) error {
	fmt.Fprintf(installer.stdout(), "INSTALL %s\nPLATFORM %s/%s\nRELEASE_SOURCE local %s\nRELEASE_NOTES\n  %s\nUPGRADE_IMPACT\n  %s\nFILES\n", manifest.Version, manifest.Platform.OS, manifest.Platform.Arch, releaseSource, indentReleaseText(manifest.ReleaseNotes), indentReleaseText(manifest.UpgradeImpact))
	for _, file := range manifest.Files {
		fmt.Fprintf(installer.stdout(), "%s %s\n", file.SHA256, file.Path)
	}
	for _, path := range []string{"release.json", "SHA256SUMS"} {
		digest, err := fileSHA256(filepath.Join(installer.BundleRoot, path))
		if err != nil {
			return fmt.Errorf("checksum preview file %s: %w", path, err)
		}
		fmt.Fprintf(installer.stdout(), "%s %s\n", digest, path)
	}
	return nil
}

func (installer InitialInstaller) afterEvent(event string) error {
	if installer.AfterEvent == nil {
		return nil
	}
	return installer.AfterEvent(event)
}

func (installer InitialInstaller) stdout() io.Writer {
	if installer.Stdout == nil {
		return io.Discard
	}
	return installer.Stdout
}

func installFixedBootstrap(stateHome, installedRoot string, manifest releasebundle.Manifest) (bool, error) {
	sourcePath := filepath.Join(installedRoot, filepath.FromSlash(manifest.Executables.Bootstrap.Path))
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return false, fmt.Errorf("read fixed bootstrap: %w", err)
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != manifest.Executables.Bootstrap.SHA256 {
		return false, errors.New("fixed bootstrap checksum differs from verified release metadata")
	}
	destination := filepath.Join(stateHome, "bin", platform.ExecutableName("sopctl"))
	info, err := os.Lstat(destination)
	if err == nil {
		if !info.Mode().IsRegular() {
			return false, errors.New("existing fixed bootstrap must be a regular file")
		}
		existing, digestErr := fileSHA256(destination)
		if digestErr != nil {
			return false, digestErr
		}
		if existing != manifest.Executables.Bootstrap.SHA256 {
			return false, errors.New("existing fixed bootstrap differs; refusing to overwrite it")
		}
		return false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := platform.AtomicWrite(destination, data, 0o755); err != nil {
		return false, fmt.Errorf("install fixed bootstrap: %w", err)
	}
	return true, nil
}

func fixedBootstrapNeedsCreate(stateHome, installedRoot string, manifest releasebundle.Manifest) (bool, error) {
	sourcePath := filepath.Join(installedRoot, filepath.FromSlash(manifest.Executables.Bootstrap.Path))
	sourceDigest, err := fileSHA256(sourcePath)
	if err != nil {
		return false, fmt.Errorf("checksum fixed bootstrap: %w", err)
	}
	if sourceDigest != manifest.Executables.Bootstrap.SHA256 {
		return false, errors.New("fixed bootstrap checksum differs from verified release metadata")
	}
	destination := filepath.Join(stateHome, "bin", platform.ExecutableName("sopctl"))
	info, err := os.Lstat(destination)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, errors.New("existing fixed bootstrap must be a regular file")
	}
	digest, err := fileSHA256(destination)
	if err != nil {
		return false, err
	}
	if digest != manifest.Executables.Bootstrap.SHA256 {
		return false, errors.New("existing fixed bootstrap differs; refusing to overwrite it")
	}
	return false, nil
}

func initialInstallJournalPath(stateHome string) string {
	return filepath.Join(stateHome, "transactions", "install.json")
}

func writeInitialInstallJournal(stateHome string, journal initialInstallJournal) error {
	journal.Format = initialInstallJournalFormat
	if err := validateInitialInstallJournal(journal); err != nil {
		return err
	}
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	return platform.AtomicWrite(initialInstallJournalPath(stateHome), append(data, '\n'), 0o600)
}

func readInitialInstallJournal(stateHome string) (initialInstallJournal, bool, error) {
	file, err := os.Open(initialInstallJournalPath(stateHome))
	if errors.Is(err, os.ErrNotExist) {
		return initialInstallJournal{}, false, nil
	}
	if err != nil {
		return initialInstallJournal{}, false, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var journal initialInstallJournal
	if err := decoder.Decode(&journal); err != nil {
		return initialInstallJournal{}, false, fmt.Errorf("parse initial-install journal: %w", err)
	}
	if err := ensureDescriptionEOF(decoder); err != nil {
		return initialInstallJournal{}, false, fmt.Errorf("parse initial-install journal: %w", err)
	}
	if err := validateInitialInstallJournal(journal); err != nil {
		return initialInstallJournal{}, false, err
	}
	return journal, true, nil
}

func validateInitialInstallJournal(journal initialInstallJournal) error {
	if journal.Format != initialInstallJournalFormat {
		return errors.New("initial-install journal format is unsupported")
	}
	switch journal.Phase {
	case "prepared", "target_plugin_ready", "bootstrap_installing", "bootstrap_ready", "source_installing", "source_ready", "current_committed":
	default:
		return errors.New("initial-install journal phase is invalid")
	}
	if !state.ValidVersion(journal.Version) || !journalCommitPattern.MatchString(journal.GitCommit) {
		return errors.New("initial-install journal release pin is invalid")
	}
	if len(journal.ReleaseSHA256) != sha256.Size*2 || len(journal.BootstrapSHA256) != sha256.Size*2 {
		return errors.New("initial-install journal checksum pin is invalid")
	}
	if _, err := hex.DecodeString(journal.ReleaseSHA256); err != nil {
		return errors.New("initial-install journal release checksum is invalid")
	}
	if _, err := hex.DecodeString(journal.BootstrapSHA256); err != nil {
		return errors.New("initial-install journal bootstrap checksum is invalid")
	}
	if err := validatePluginRef(journal.Plugin, journal.Version); err != nil {
		return fmt.Errorf("initial-install journal plugin: %w", err)
	}
	if journal.BootstrapCreated && journal.Phase != "bootstrap_installing" && journal.Phase != "bootstrap_ready" && journal.Phase != "current_committed" {
		if journal.Phase != "source_installing" && journal.Phase != "source_ready" {
			return errors.New("initial-install journal bootstrap creation flag is inconsistent")
		}
	}
	if journal.ReleaseSourceRoot == "" || !filepath.IsAbs(journal.ReleaseSourceRoot) {
		return errors.New("initial-install journal release source pin is invalid")
	}
	if journal.SourceCreated && journal.Phase != "source_installing" && journal.Phase != "source_ready" && journal.Phase != "current_committed" {
		return errors.New("initial-install journal source creation flag is inconsistent")
	}
	return nil
}

func clearInitialInstallJournal(stateHome string) error {
	err := os.Remove(initialInstallJournalPath(stateHome))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func readCurrentIfExists(stateHome string) (state.Current, bool, error) {
	_, err := os.Lstat(filepath.Join(stateHome, "current.json"))
	if errors.Is(err, os.ErrNotExist) {
		return state.Current{}, false, nil
	}
	if err != nil {
		return state.Current{}, false, err
	}
	current, err := state.ReadCurrent(stateHome)
	return current, err == nil, err
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
