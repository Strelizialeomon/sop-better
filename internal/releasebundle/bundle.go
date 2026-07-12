package releasebundle

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/Strelizialeomon/sop-better/internal/config"
)

const (
	releaseFormat = 1
	pluginName    = "sop-better"
)

var (
	semverPattern           = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)
	commitPattern           = regexp.MustCompile(`^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)
	nonMarketplaceCharacter = regexp.MustCompile(`[^A-Za-z0-9_-]+`)
	commonHomePathPattern   = regexp.MustCompile(`(?i)(?:(?:^|[^A-Z0-9])[A-Z]:[\\/]|\\\\[^\\\s"']+\\[^\\\s"']+(?:\\|$)|(?:^|[^A-Z0-9:/])/(?:Users|home)/[^/\s"']+/)`)
)

type Options struct {
	SourceRoot      string
	PluginRoot      string
	OutputRoot      string
	Version         string
	GitTag          string
	GitCommit       string
	ReleaseNotes    string
	UpgradeImpact   string
	BootstrapBinary string
	InstallerBinary string
	ManagerBinary   string
	EngineBinary    string
	BinaryVersion   string
	TargetOS        string
	TargetArch      string
}

type Manifest struct {
	Format        int            `json:"format"`
	Version       string         `json:"version"`
	GitTag        string         `json:"git_tag"`
	GitCommit     string         `json:"git_commit"`
	ReleaseNotes  string         `json:"release_notes"`
	UpgradeImpact string         `json:"upgrade_impact"`
	Plugin        PluginMetadata `json:"plugin"`
	Contract      Contract       `json:"contract"`
	Standard      Standard       `json:"standard"`
	Platform      *Platform      `json:"platform,omitempty"`
	Executables   *Executables   `json:"executables,omitempty"`
	Files         []FileDigest   `json:"files"`
}

type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type Executables struct {
	Bootstrap BootstrapExecutable `json:"bootstrap"`
	Installer Executable          `json:"installer"`
	Manager   Executable          `json:"manager"`
	Engine    Executable          `json:"engine"`
}

type BootstrapExecutable struct {
	Path     string `json:"path"`
	Protocol int    `json:"protocol"`
	SHA256   string `json:"sha256"`
}

type Executable struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

type PluginMetadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Marketplace string `json:"marketplace"`
}

type Contract struct {
	ManifestSchemaVersion int    `json:"manifest_schema_version"`
	ProfileSchemaVersion  int    `json:"profile_schema_version"`
	RulesVersion          string `json:"rules_version"`
}

type Standard struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type FileDigest struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type FileDifference struct {
	Status        string
	Path          string
	CurrentSHA256 string
	TargetSHA256  string
}

type OutputTargetMove struct {
	ID            string
	CurrentTarget string
	TargetTarget  string
}

func OutputTargetMoves(currentRoot, targetRoot string) ([]OutputTargetMove, error) {
	if _, err := Inspect(currentRoot); err != nil {
		return nil, fmt.Errorf("verify current release: %w", err)
	}
	if _, err := Inspect(targetRoot); err != nil {
		return nil, fmt.Errorf("verify target release: %w", err)
	}
	current, err := config.LoadManifest(filepath.Join(currentRoot, "assets", "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("read current generation contract: %w", err)
	}
	target, err := config.LoadManifest(filepath.Join(targetRoot, "assets", "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("read target generation contract: %w", err)
	}
	currentTargets := make(map[string]string, len(current.Outputs))
	for _, output := range current.Outputs {
		currentTargets[output.ID] = output.Target
	}
	moves := make([]OutputTargetMove, 0)
	for _, output := range target.Outputs {
		if currentTarget, exists := currentTargets[output.ID]; exists && currentTarget != output.Target {
			moves = append(moves, OutputTargetMove{ID: output.ID, CurrentTarget: currentTarget, TargetTarget: output.Target})
		}
	}
	sort.Slice(moves, func(i, j int) bool { return moves[i].ID < moves[j].ID })
	return moves, nil
}

func Compare(currentRoot, targetRoot string) ([]FileDifference, error) {
	current, err := Inspect(currentRoot)
	if err != nil {
		return nil, fmt.Errorf("verify current release: %w", err)
	}
	target, err := Inspect(targetRoot)
	if err != nil {
		return nil, fmt.Errorf("verify target release: %w", err)
	}
	currentFiles := make(map[string]string, len(current.Files)+1)
	targetFiles := make(map[string]string, len(target.Files)+1)
	for _, file := range current.Files {
		currentFiles[file.Path] = file.SHA256
	}
	for _, file := range target.Files {
		targetFiles[file.Path] = file.SHA256
	}
	currentRelease, err := digestFile(filepath.Join(currentRoot, "release.json"))
	if err != nil {
		return nil, err
	}
	targetRelease, err := digestFile(filepath.Join(targetRoot, "release.json"))
	if err != nil {
		return nil, err
	}
	currentFiles["release.json"] = currentRelease
	targetFiles["release.json"] = targetRelease
	paths := make(map[string]struct{}, len(currentFiles)+len(targetFiles))
	for path := range currentFiles {
		paths[path] = struct{}{}
	}
	for path := range targetFiles {
		paths[path] = struct{}{}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	differences := make([]FileDifference, 0)
	for _, path := range ordered {
		currentDigest, inCurrent := currentFiles[path]
		targetDigest, inTarget := targetFiles[path]
		switch {
		case !inCurrent:
			differences = append(differences, FileDifference{Status: "ADD", Path: path, TargetSHA256: targetDigest})
		case !inTarget:
			differences = append(differences, FileDifference{Status: "DELETE", Path: path, CurrentSHA256: currentDigest})
		case currentDigest != targetDigest:
			differences = append(differences, FileDifference{Status: "UPDATE", Path: path, CurrentSHA256: currentDigest, TargetSHA256: targetDigest})
		}
	}
	return differences, nil
}

type sourceManifest struct {
	SchemaVersion        int    `json:"schema_version"`
	SOPVersion           string `json:"sop_version"`
	ProfileSchemaVersion int    `json:"profile_schema_version"`
	RulesVersion         string `json:"rules_version"`
	Standard             struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"standard"`
}

func Build(options Options) error {
	if err := validateOptions(options); err != nil {
		return err
	}

	sourceRoot, err := filepath.Abs(options.SourceRoot)
	if err != nil {
		return fmt.Errorf("resolve source root: %w", err)
	}
	pluginRoot, err := filepath.Abs(options.PluginRoot)
	if err != nil {
		return fmt.Errorf("resolve plugin root: %w", err)
	}
	outputRoot, err := filepath.Abs(options.OutputRoot)
	if err != nil {
		return fmt.Errorf("resolve output root: %w", err)
	}
	if err := rejectOutputOverlap(outputRoot, sourceRoot); err != nil {
		return err
	}
	if _, err := os.Stat(outputRoot); err == nil {
		return fmt.Errorf("output already exists: %s", outputRoot)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect output: %w", err)
	}

	manifest, err := loadSourceManifest(filepath.Join(sourceRoot, "manifest.json"))
	if err != nil {
		return err
	}
	if manifest.SOPVersion != options.Version {
		return fmt.Errorf("manifest.sop_version %q does not match release version %q", manifest.SOPVersion, options.Version)
	}
	if manifest.Standard.Path != "STANDARD.md" {
		return fmt.Errorf("manifest.standard.path must be %q", "STANDARD.md")
	}
	standardPath := filepath.Join(sourceRoot, "STANDARD.md")
	standardDigest, err := digestFile(standardPath)
	if err != nil {
		return fmt.Errorf("checksum STANDARD.md: %w", err)
	}
	if !strings.EqualFold(manifest.Standard.SHA256, standardDigest) {
		return errors.New("manifest.standard.sha256 does not match STANDARD.md")
	}

	if err := os.MkdirAll(filepath.Dir(outputRoot), 0o755); err != nil {
		return fmt.Errorf("create output parent: %w", err)
	}
	stage, err := os.MkdirTemp(filepath.Dir(outputRoot), ".sop-release-stage-")
	if err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	defer os.RemoveAll(stage)

	if err := populate(stage, sourceRoot, pluginRoot, options, manifest, standardDigest); err != nil {
		return err
	}
	if err := rejectAbsolutePathLeaks(stage, sourceRoot, pluginRoot); err != nil {
		return err
	}
	if err := Verify(stage); err != nil {
		return fmt.Errorf("verify staged release: %w", err)
	}
	if err := os.Rename(stage, outputRoot); err != nil {
		return fmt.Errorf("publish release bundle: %w", err)
	}
	return nil
}

func rejectOutputOverlap(outputRoot, sourceRoot string) error {
	outputParent, err := canonicalPath(filepath.Dir(outputRoot))
	if err != nil {
		return fmt.Errorf("resolve output parent: %w", err)
	}
	for _, snapshot := range []string{
		filepath.Join(sourceRoot, "master"),
		filepath.Join(sourceRoot, "schemas"),
		filepath.Join(sourceRoot, "skills", "sop-init"),
		filepath.Join(sourceRoot, "skills", "sop-audit"),
		filepath.Join(sourceRoot, "skills", "sop-run"),
	} {
		resolvedSnapshot, err := canonicalPath(snapshot)
		if err != nil {
			return fmt.Errorf("resolve source snapshot: %w", err)
		}
		if pathWithin(outputParent, resolvedSnapshot) {
			return fmt.Errorf("output overlaps source snapshot: %s", snapshot)
		}
	}
	return nil
}

func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	parent := filepath.Dir(absolute)
	if parent == absolute {
		return filepath.Clean(absolute), nil
	}
	resolvedParent, err := canonicalPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedParent, filepath.Base(absolute)), nil
}

func pathWithin(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func rejectAbsolutePathLeaks(root string, knownRoots ...string) error {
	candidates := make([][]byte, 0, len(knownRoots)+1)
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		knownRoots = append(knownRoots, home)
	}
	for _, knownRoot := range knownRoots {
		absolute, err := filepath.Abs(knownRoot)
		if err != nil {
			return fmt.Errorf("resolve path leak candidate: %w", err)
		}
		resolved, err := canonicalPath(absolute)
		if err != nil {
			return fmt.Errorf("resolve path leak candidate symlinks: %w", err)
		}
		variants := []string{absolute, filepath.ToSlash(absolute), resolved, filepath.ToSlash(resolved)}
		if info, lstatErr := os.Lstat(absolute); lstatErr == nil && info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(absolute)
			if readErr != nil {
				return fmt.Errorf("read path leak candidate symlink: %w", readErr)
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(absolute), target)
			}
			target = filepath.Clean(target)
			variants = append(variants, target, filepath.ToSlash(target))
		} else if lstatErr != nil && !errors.Is(lstatErr, os.ErrNotExist) {
			return fmt.Errorf("inspect path leak candidate: %w", lstatErr)
		}
		for _, candidate := range variants {
			if candidate != "" && candidate != string(filepath.Separator) {
				candidates = append(candidates, []byte(candidate))
			}
		}
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		releasePath := filepath.ToSlash(relative)
		isExecutablePayload := strings.HasPrefix(releasePath, "bin/") || strings.HasPrefix(releasePath, "bootstrap/")
		leaks := !isExecutablePayload && commonHomePathPattern.Find(data) != nil
		if !leaks {
			for _, candidate := range candidates {
				if bytes.Contains(data, candidate) {
					leaks = true
					break
				}
			}
		}
		if leaks {
			return fmt.Errorf("artifact contains an author absolute path: %s", releasePath)
		}
		return nil
	})
}

func Verify(bundleRoot string) error {
	root, err := filepath.Abs(bundleRoot)
	if err != nil {
		return fmt.Errorf("resolve bundle root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve bundle root symlinks: %w", err)
	}
	if err := validateBundleTree(root); err != nil {
		return err
	}

	var release Manifest
	if err := readJSON(filepath.Join(root, "release.json"), &release); err != nil {
		return fmt.Errorf("read release.json: %w", err)
	}
	if err := validateReleaseManifest(release); err != nil {
		return err
	}
	if release.Executables != nil {
		bootstrap := release.Executables.Bootstrap
		bootstrapDigest, err := digestFile(filepath.Join(root, filepath.FromSlash(bootstrap.Path)))
		if err != nil {
			return fmt.Errorf("checksum bootstrap executable: %w", err)
		}
		if bootstrapDigest != bootstrap.SHA256 {
			return errors.New("bootstrap executable checksum does not match release.json")
		}
		for name, executable := range map[string]Executable{"installer": release.Executables.Installer, "manager": release.Executables.Manager, "engine": release.Executables.Engine} {
			if !safeRelativePath(executable.Path) {
				return fmt.Errorf("%s executable path must be release-relative", name)
			}
			digest, err := digestFile(filepath.Join(root, filepath.FromSlash(executable.Path)))
			if err != nil {
				return fmt.Errorf("checksum %s executable: %w", name, err)
			}
			if digest != executable.SHA256 {
				return fmt.Errorf("%s executable checksum does not match release.json", name)
			}
		}
	}

	assetRoot := filepath.Join(root, "assets")
	contract, err := config.LoadManifest(filepath.Join(assetRoot, "manifest.json"))
	if err != nil {
		return fmt.Errorf("read assets/manifest.json: %w", err)
	}
	if err := contract.ValidateAssets(assetRoot); err != nil {
		return fmt.Errorf("validate bundled generation contract: %w", err)
	}
	if contract.SOPVersion != release.Version {
		return errors.New("assets/manifest.json sop_version does not match release.json")
	}
	if contract.Standard.Path != "STANDARD.md" {
		return errors.New("manifest.standard.path must be \"STANDARD.md\"")
	}
	if contract.SchemaVersion != release.Contract.ManifestSchemaVersion ||
		contract.ProfileSchemaVersion != release.Contract.ProfileSchemaVersion ||
		contract.RulesVersion != release.Contract.RulesVersion {
		return errors.New("assets/manifest.json contract versions do not match release.json")
	}

	var plugin map[string]any
	pluginManifestPath := filepath.Join(root, "marketplace", "plugins", pluginName, ".codex-plugin", "plugin.json")
	if err := readJSON(pluginManifestPath, &plugin); err != nil {
		return fmt.Errorf("read plugin manifest: %w", err)
	}
	if plugin["name"] != release.Plugin.Name || plugin["version"] != release.Plugin.Version {
		return errors.New("plugin manifest name/version does not match release.json")
	}

	var marketplace map[string]any
	marketplacePath := filepath.Join(root, "marketplace", ".agents", "plugins", "marketplace.json")
	if err := readJSON(marketplacePath, &marketplace); err != nil {
		return fmt.Errorf("read marketplace manifest: %w", err)
	}
	if marketplace["name"] != release.Plugin.Marketplace {
		return errors.New("marketplace name does not match release.json")
	}
	if err := validateMarketplacePluginEntry(marketplace); err != nil {
		return err
	}

	standardDigest, err := digestFile(filepath.Join(root, filepath.FromSlash(release.Standard.Path)))
	if err != nil {
		return fmt.Errorf("checksum bundled STANDARD.md: %w", err)
	}
	if !strings.EqualFold(standardDigest, release.Standard.SHA256) ||
		!strings.EqualFold(standardDigest, contract.Standard.SHA256) {
		return errors.New("STANDARD.md checksum does not match release metadata")
	}

	if err := verifySnapshotCopies(root); err != nil {
		return err
	}
	if err := verifyReleaseFiles(root, release.Files); err != nil {
		return err
	}
	if err := verifyChecksumFile(root); err != nil {
		return err
	}
	return nil
}

func Inspect(bundleRoot string) (Manifest, error) {
	if err := Verify(bundleRoot); err != nil {
		return Manifest{}, err
	}
	root, err := filepath.Abs(bundleRoot)
	if err != nil {
		return Manifest{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := readJSON(filepath.Join(root, "release.json"), &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func Install(sourceRoot, destinationRoot string) (Manifest, error) {
	manifest, err := Inspect(sourceRoot)
	if err != nil {
		return Manifest{}, err
	}
	if existing, statErr := os.Lstat(destinationRoot); statErr == nil {
		if existing.Mode()&os.ModeSymlink != 0 {
			return Manifest{}, errors.New("installed version path must not be a symlink")
		}
		if !existing.IsDir() {
			return Manifest{}, errors.New("installed version path is not a directory")
		}
		installed, inspectErr := Inspect(destinationRoot)
		if inspectErr != nil {
			return Manifest{}, fmt.Errorf("verify installed version: %w", inspectErr)
		}
		sourceIdentity, sourceDigestErr := digestFile(filepath.Join(sourceRoot, "release.json"))
		if sourceDigestErr != nil {
			return Manifest{}, fmt.Errorf("checksum source release identity: %w", sourceDigestErr)
		}
		installedIdentity, installedDigestErr := digestFile(filepath.Join(destinationRoot, "release.json"))
		if installedDigestErr != nil {
			return Manifest{}, fmt.Errorf("checksum installed release identity: %w", installedDigestErr)
		}
		if installed.Version != manifest.Version || installed.GitCommit != manifest.GitCommit || installedIdentity != sourceIdentity {
			return Manifest{}, errors.New("installed version differs from release source")
		}
		return installed, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return Manifest{}, statErr
	}
	if err := os.MkdirAll(filepath.Dir(destinationRoot), 0o755); err != nil {
		return Manifest{}, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(destinationRoot), ".install-stage-")
	if err != nil {
		return Manifest{}, err
	}
	defer os.RemoveAll(stage)
	if _, err := copyTree(sourceRoot, stage); err != nil {
		return Manifest{}, err
	}
	if manifest.Executables != nil {
		for _, path := range []string{manifest.Executables.Bootstrap.Path, manifest.Executables.Installer.Path, manifest.Executables.Manager.Path, manifest.Executables.Engine.Path} {
			if err := os.Chmod(filepath.Join(stage, filepath.FromSlash(path)), 0o755); err != nil {
				return Manifest{}, err
			}
		}
	}
	staged, err := Inspect(stage)
	if err != nil {
		return Manifest{}, err
	}
	if !reflect.DeepEqual(staged, manifest) {
		return Manifest{}, errors.New("release source changed while it was being installed")
	}
	if err := os.Rename(stage, destinationRoot); err != nil {
		return Manifest{}, err
	}
	return staged, nil
}

// RepairInstalled replaces a damaged installed bundle with the exact verified
// source bundle. It stages and verifies the replacement before moving the old
// directory, restores the old directory on ordinary errors, and uses stable
// stage/backup names so a rerun can finish or roll back an interrupted repair.
func RepairInstalled(sourceRoot, destinationRoot string) (Manifest, error) {
	manifest, err := Inspect(sourceRoot)
	if err != nil {
		return Manifest{}, err
	}
	parent := filepath.Dir(destinationRoot)
	base := filepath.Base(destinationRoot)
	stage := filepath.Join(parent, "."+base+".repair-stage")
	backup := filepath.Join(parent, "."+base+".repair-backup")
	if err := recoverInstalledRepair(sourceRoot, destinationRoot, stage, backup, manifest); err != nil {
		return Manifest{}, err
	}
	if installed, inspectErr := Inspect(destinationRoot); inspectErr == nil {
		if sameReleaseIdentity(sourceRoot, destinationRoot, manifest, installed) {
			return installed, nil
		}
		return Manifest{}, errors.New("installed version differs from release source")
	}
	if info, statErr := os.Lstat(destinationRoot); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return Install(sourceRoot, destinationRoot)
		}
		return Manifest{}, statErr
	} else if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return Manifest{}, errors.New("damaged installed version path must be a real directory")
	}
	if err := os.RemoveAll(stage); err != nil {
		return Manifest{}, err
	}
	staged, err := Install(sourceRoot, stage)
	if err != nil {
		return Manifest{}, fmt.Errorf("stage verified repair: %w", err)
	}
	if err := os.Rename(destinationRoot, backup); err != nil {
		return Manifest{}, fmt.Errorf("preserve damaged installed version: %w", err)
	}
	if err := os.Rename(stage, destinationRoot); err != nil {
		restoreErr := os.Rename(backup, destinationRoot)
		if restoreErr != nil {
			return Manifest{}, fmt.Errorf("commit installed repair: %w; restore previous directory: %v", err, restoreErr)
		}
		return Manifest{}, fmt.Errorf("commit installed repair: %w", err)
	}
	installed, err := Inspect(destinationRoot)
	if err != nil || !reflect.DeepEqual(installed, staged) {
		_ = os.RemoveAll(destinationRoot)
		restoreErr := os.Rename(backup, destinationRoot)
		if err == nil {
			err = errors.New("repaired release differs from staged release")
		}
		if restoreErr != nil {
			return Manifest{}, fmt.Errorf("verify installed repair: %w; restore previous directory: %v", err, restoreErr)
		}
		return Manifest{}, fmt.Errorf("verify installed repair: %w", err)
	}
	if err := os.RemoveAll(backup); err != nil {
		return Manifest{}, fmt.Errorf("remove repaired release backup: %w", err)
	}
	return installed, nil
}

func recoverInstalledRepair(sourceRoot, destinationRoot, stage, backup string, source Manifest) error {
	destinationExists, err := pathExistsForRepair(destinationRoot)
	if err != nil {
		return err
	}
	stageExists, err := pathExistsForRepair(stage)
	if err != nil {
		return err
	}
	backupExists, err := pathExistsForRepair(backup)
	if err != nil {
		return err
	}
	if destinationExists && backupExists {
		if installed, err := Inspect(destinationRoot); err == nil && sameReleaseIdentity(sourceRoot, destinationRoot, source, installed) {
			if err := os.RemoveAll(backup); err != nil {
				return err
			}
			return os.RemoveAll(stage)
		}
		if err := os.RemoveAll(destinationRoot); err != nil {
			return err
		}
		if err := os.Rename(backup, destinationRoot); err != nil {
			return fmt.Errorf("restore interrupted installed repair: %w", err)
		}
		return os.RemoveAll(stage)
	}
	if !destinationExists && backupExists {
		if stageExists {
			if staged, err := Inspect(stage); err == nil && sameReleaseIdentity(sourceRoot, stage, source, staged) {
				if err := os.Rename(stage, destinationRoot); err != nil {
					return err
				}
				return os.RemoveAll(backup)
			}
		}
		if err := os.Rename(backup, destinationRoot); err != nil {
			return fmt.Errorf("restore interrupted installed repair: %w", err)
		}
		return os.RemoveAll(stage)
	}
	if stageExists {
		return os.RemoveAll(stage)
	}
	return nil
}

func pathExistsForRepair(path string) (bool, error) {
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func sameReleaseIdentity(sourceRoot, installedRoot string, source, installed Manifest) bool {
	if source.Version != installed.Version || source.GitCommit != installed.GitCommit {
		return false
	}
	sourceDigest, sourceErr := digestFile(filepath.Join(sourceRoot, "release.json"))
	installedDigest, installedErr := digestFile(filepath.Join(installedRoot, "release.json"))
	return sourceErr == nil && installedErr == nil && sourceDigest == installedDigest
}

func validateBundleTree(root string) error {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("inspect bundle root: %w", err)
	}
	if !rootInfo.IsDir() {
		return errors.New("bundle root must be a directory")
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("bundle contains symlink: %s", path)
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect bundle file %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("bundle contains non-regular file: %s", path)
		}
		return nil
	})
}

func validateOptions(options Options) error {
	if semverPattern.MatchString(options.Version) == false {
		return fmt.Errorf("version %q must be strict semver", options.Version)
	}
	if options.GitTag != "v"+options.Version {
		return fmt.Errorf("git tag %q does not match version %q", options.GitTag, options.Version)
	}
	if !commitPattern.MatchString(options.GitCommit) {
		return errors.New("git commit must be a full 40- or 64-character hexadecimal SHA")
	}
	if err := validateHumanText("release notes", options.ReleaseNotes); err != nil {
		return err
	}
	if err := validateHumanText("upgrade impact", options.UpgradeImpact); err != nil {
		return err
	}
	if options.BootstrapBinary == "" || options.InstallerBinary == "" || options.ManagerBinary == "" || options.EngineBinary == "" || options.BinaryVersion == "" || options.TargetOS == "" || options.TargetArch == "" {
		return errors.New("bootstrap, installer, manager, engine, binary version, target OS, and target arch must be provided together")
	}
	if options.BinaryVersion != options.Version {
		return errors.New("binary version does not match release version")
	}
	for name, value := range map[string]string{
		"source":      options.SourceRoot,
		"plugin root": options.PluginRoot,
		"output":      options.OutputRoot,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s path is required", name)
		}
	}
	return nil
}

func validateReleaseManifest(release Manifest) error {
	if release.Format != releaseFormat {
		return fmt.Errorf("release.json format must be %d", releaseFormat)
	}
	if !semverPattern.MatchString(release.Version) {
		return errors.New("release.json version must be strict semver")
	}
	if release.GitTag != "v"+release.Version {
		return errors.New("release.json git_tag does not match version")
	}
	if !commitPattern.MatchString(release.GitCommit) {
		return errors.New("release.json git_commit must be a full SHA")
	}
	if err := validateHumanText("release.json release_notes", release.ReleaseNotes); err != nil {
		return err
	}
	if err := validateHumanText("release.json upgrade_impact", release.UpgradeImpact); err != nil {
		return err
	}
	if release.Plugin.Name != pluginName || release.Plugin.Version != release.Version {
		return errors.New("release.json plugin name/version is inconsistent")
	}
	if release.Plugin.Marketplace != marketplaceName(release.Version) {
		return errors.New("release.json marketplace name is inconsistent")
	}
	if release.Contract.ManifestSchemaVersion != 1 || release.Contract.ProfileSchemaVersion != 1 || strings.TrimSpace(release.Contract.RulesVersion) == "" {
		return errors.New("release.json contract versions are incomplete")
	}
	if release.Standard.Path != "assets/STANDARD.md" || len(release.Standard.SHA256) != sha256.Size*2 {
		return errors.New("release.json STANDARD.md metadata is invalid")
	}
	if release.Platform == nil || release.Executables == nil {
		return errors.New("release.json platform and versioned executables are required")
	}
	if release.Platform.OS == "" || release.Platform.Arch == "" {
		return errors.New("release.json platform is incomplete")
	}
	suffix := ""
	if release.Platform.OS == "windows" {
		suffix = ".exe"
	}
	expectedPaths := map[string]string{
		"installer": filepath.ToSlash(filepath.Join("bin", "sop-install"+suffix)),
		"manager":   filepath.ToSlash(filepath.Join("bin", "sopctl-manager"+suffix)),
		"engine":    filepath.ToSlash(filepath.Join("bin", "sopctl-engine"+suffix)),
	}
	expectedBootstrapPath := filepath.ToSlash(filepath.Join("bootstrap", "sopctl"+suffix))
	if release.Executables.Bootstrap.Path != expectedBootstrapPath ||
		release.Executables.Bootstrap.Protocol != 1 ||
		len(release.Executables.Bootstrap.SHA256) != sha256.Size*2 {
		return errors.New("release.json bootstrap executable metadata is inconsistent")
	}
	for name, executable := range map[string]Executable{"installer": release.Executables.Installer, "manager": release.Executables.Manager, "engine": release.Executables.Engine} {
		if executable.Version != release.Version || executable.Path == "" || len(executable.SHA256) != sha256.Size*2 {
			return fmt.Errorf("release.json %s executable metadata is inconsistent", name)
		}
		if executable.Path != expectedPaths[name] {
			return fmt.Errorf("release.json %s executable path must be %s", name, expectedPaths[name])
		}
	}
	return nil
}

func validateHumanText(label, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", label)
	}
	if len(value) > 16*1024 {
		return fmt.Errorf("%s is too large", label)
	}
	for _, character := range value {
		if (character < 0x20 && character != '\n' && character != '\t') || character == 0x7f {
			return fmt.Errorf("%s contains terminal control characters", label)
		}
	}
	return nil
}

func populate(stage, sourceRoot, pluginRoot string, options Options, contract sourceManifest, standardDigest string) error {
	assetRoot := filepath.Join(stage, "assets")
	pluginOutput := filepath.Join(stage, "marketplace", "plugins", pluginName)

	for _, pair := range [][2]string{
		{filepath.Join(sourceRoot, "STANDARD.md"), filepath.Join(assetRoot, "STANDARD.md")},
		{filepath.Join(sourceRoot, "manifest.json"), filepath.Join(assetRoot, "manifest.json")},
		{filepath.Join(sourceRoot, "STANDARD.md"), filepath.Join(pluginOutput, "rules", "STANDARD.md")},
		{filepath.Join(sourceRoot, "manifest.json"), filepath.Join(pluginOutput, "rules", "manifest.json")},
	} {
		if err := copyFile(pair[0], pair[1]); err != nil {
			return err
		}
	}
	for _, pair := range [][2]string{
		{filepath.Join(sourceRoot, "master"), filepath.Join(assetRoot, "master")},
		{filepath.Join(sourceRoot, "schemas"), filepath.Join(assetRoot, "schemas")},
		{filepath.Join(assetRoot, "master"), filepath.Join(pluginOutput, "rules", "master")},
		{filepath.Join(assetRoot, "schemas"), filepath.Join(pluginOutput, "rules", "schemas")},
		{filepath.Join(sourceRoot, "skills", "sop-init"), filepath.Join(pluginOutput, "skills", "sop-init")},
		{filepath.Join(sourceRoot, "skills", "sop-audit"), filepath.Join(pluginOutput, "skills", "sop-audit")},
		{filepath.Join(sourceRoot, "skills", "sop-run"), filepath.Join(pluginOutput, "skills", "sop-run")},
	} {
		count, err := copyTree(pair[0], pair[1])
		if err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("required snapshot is empty: %s", pair[0])
		}
	}

	if err := writeVersionedPlugin(pluginRoot, pluginOutput, options.Version); err != nil {
		return err
	}
	marketplace := marketplaceName(options.Version)
	if err := writeVersionedMarketplace(pluginRoot, filepath.Join(stage, "marketplace"), marketplace, options.Version); err != nil {
		return err
	}
	var platformMetadata *Platform
	var executables *Executables
	if options.ManagerBinary != "" {
		suffix := ""
		if options.TargetOS == "windows" {
			suffix = ".exe"
		}
		managerPath := filepath.ToSlash(filepath.Join("bin", "sopctl-manager"+suffix))
		enginePath := filepath.ToSlash(filepath.Join("bin", "sopctl-engine"+suffix))
		installerPath := filepath.ToSlash(filepath.Join("bin", "sop-install"+suffix))
		bootstrapPath := filepath.ToSlash(filepath.Join("bootstrap", "sopctl"+suffix))
		if err := copyExecutable(options.BootstrapBinary, filepath.Join(stage, filepath.FromSlash(bootstrapPath))); err != nil {
			return err
		}
		if err := copyExecutable(options.ManagerBinary, filepath.Join(stage, filepath.FromSlash(managerPath))); err != nil {
			return err
		}
		if err := copyExecutable(options.EngineBinary, filepath.Join(stage, filepath.FromSlash(enginePath))); err != nil {
			return err
		}
		if err := copyExecutable(options.InstallerBinary, filepath.Join(stage, filepath.FromSlash(installerPath))); err != nil {
			return err
		}
		managerDigest, err := digestFile(filepath.Join(stage, filepath.FromSlash(managerPath)))
		if err != nil {
			return err
		}
		engineDigest, err := digestFile(filepath.Join(stage, filepath.FromSlash(enginePath)))
		if err != nil {
			return err
		}
		bootstrapDigest, err := digestFile(filepath.Join(stage, filepath.FromSlash(bootstrapPath)))
		if err != nil {
			return err
		}
		installerDigest, err := digestFile(filepath.Join(stage, filepath.FromSlash(installerPath)))
		if err != nil {
			return err
		}
		platformMetadata = &Platform{OS: options.TargetOS, Arch: options.TargetArch}
		executables = &Executables{
			Bootstrap: BootstrapExecutable{Path: bootstrapPath, Protocol: 1, SHA256: bootstrapDigest},
			Installer: Executable{Path: installerPath, Version: options.BinaryVersion, SHA256: installerDigest},
			Manager:   Executable{Path: managerPath, Version: options.BinaryVersion, SHA256: managerDigest},
			Engine:    Executable{Path: enginePath, Version: options.BinaryVersion, SHA256: engineDigest},
		}
	}

	payload, err := collectDigests(stage, map[string]bool{"release.json": true, "SHA256SUMS": true})
	if err != nil {
		return err
	}
	release := Manifest{
		Format:        releaseFormat,
		Version:       options.Version,
		GitTag:        options.GitTag,
		GitCommit:     strings.ToLower(options.GitCommit),
		ReleaseNotes:  strings.TrimSpace(options.ReleaseNotes),
		UpgradeImpact: strings.TrimSpace(options.UpgradeImpact),
		Plugin: PluginMetadata{
			Name:        pluginName,
			Version:     options.Version,
			Marketplace: marketplace,
		},
		Contract: Contract{
			ManifestSchemaVersion: contract.SchemaVersion,
			ProfileSchemaVersion:  contract.ProfileSchemaVersion,
			RulesVersion:          contract.RulesVersion,
		},
		Standard:    Standard{Path: "assets/STANDARD.md", SHA256: standardDigest},
		Platform:    platformMetadata,
		Executables: executables,
		Files:       payload,
	}
	if err := writeJSON(filepath.Join(stage, "release.json"), release); err != nil {
		return err
	}
	allFiles, err := collectDigests(stage, map[string]bool{"SHA256SUMS": true})
	if err != nil {
		return err
	}
	return writeChecksumFile(filepath.Join(stage, "SHA256SUMS"), allFiles)
}

func loadSourceManifest(path string) (sourceManifest, error) {
	if err := requireRegularFile(path); err != nil {
		return sourceManifest{}, fmt.Errorf("read manifest.json: %w", err)
	}
	manifest, err := config.LoadManifest(path)
	if err != nil {
		return sourceManifest{}, err
	}
	if err := manifest.ValidateAssets(filepath.Dir(path)); err != nil {
		return sourceManifest{}, fmt.Errorf("validate generation contract: %w", err)
	}
	contract := sourceManifest{
		SchemaVersion:        manifest.SchemaVersion,
		SOPVersion:           manifest.SOPVersion,
		ProfileSchemaVersion: manifest.ProfileSchemaVersion,
		RulesVersion:         manifest.RulesVersion,
	}
	contract.Standard.Path = manifest.Standard.Path
	contract.Standard.SHA256 = manifest.Standard.SHA256
	return contract, nil
}

func writeVersionedPlugin(scaffoldRoot, outputRoot, version string) error {
	path := filepath.Join(scaffoldRoot, "plugins", pluginName, ".codex-plugin", "plugin.json")
	var manifest map[string]any
	if err := readJSON(path, &manifest); err != nil {
		return fmt.Errorf("read plugin scaffold: %w", err)
	}
	if manifest["name"] != pluginName {
		return fmt.Errorf("plugin scaffold name must be %q", pluginName)
	}
	manifest["version"] = version
	return writeJSON(filepath.Join(outputRoot, ".codex-plugin", "plugin.json"), manifest)
}

func writeVersionedMarketplace(scaffoldRoot, outputRoot, name, version string) error {
	path := filepath.Join(scaffoldRoot, ".agents", "plugins", "marketplace.json")
	var marketplace map[string]any
	if err := readJSON(path, &marketplace); err != nil {
		return fmt.Errorf("read marketplace scaffold: %w", err)
	}
	if err := validateMarketplacePluginEntry(marketplace); err != nil {
		return err
	}
	marketplace["name"] = name
	marketplace["interface"] = map[string]any{"displayName": "SOP Better stable v" + version}
	return writeJSON(filepath.Join(outputRoot, ".agents", "plugins", "marketplace.json"), marketplace)
}

func validateMarketplacePluginEntry(marketplace map[string]any) error {
	plugins, ok := marketplace["plugins"].([]any)
	if !ok || len(plugins) != 1 {
		return errors.New("marketplace plugin entry must contain only sop-better")
	}
	entry, ok := plugins[0].(map[string]any)
	if !ok || entry["name"] != pluginName {
		return errors.New("marketplace plugin entry name must be sop-better")
	}
	source, ok := entry["source"].(map[string]any)
	if !ok || source["source"] != "local" || source["path"] != "./plugins/sop-better" {
		return errors.New("marketplace plugin entry source must be ./plugins/sop-better")
	}
	policy, ok := entry["policy"].(map[string]any)
	if !ok || policy["installation"] != "AVAILABLE" || policy["authentication"] != "ON_INSTALL" {
		return errors.New("marketplace plugin entry policy must be AVAILABLE/ON_INSTALL")
	}
	category, ok := entry["category"].(string)
	if !ok || strings.TrimSpace(category) == "" {
		return errors.New("marketplace plugin entry category is required")
	}
	return nil
}

func marketplaceName(version string) string {
	normalized := nonMarketplaceCharacter.ReplaceAllString(version, "-")
	if strings.ContainsAny(version, "+-") {
		normalized += "-x" + hex.EncodeToString([]byte(version))
	}
	return "sop-better-stable-v" + normalized
}

func verifySnapshotCopies(root string) error {
	pairs := [][2]string{
		{"assets/STANDARD.md", "marketplace/plugins/sop-better/rules/STANDARD.md"},
		{"assets/manifest.json", "marketplace/plugins/sop-better/rules/manifest.json"},
	}
	for _, pair := range pairs {
		left, err := digestFile(filepath.Join(root, filepath.FromSlash(pair[0])))
		if err != nil {
			return fmt.Errorf("checksum %s: %w", pair[0], err)
		}
		right, err := digestFile(filepath.Join(root, filepath.FromSlash(pair[1])))
		if err != nil {
			return fmt.Errorf("checksum %s: %w", pair[1], err)
		}
		if left != right {
			return fmt.Errorf("snapshot copies differ: %s and %s", pair[0], pair[1])
		}
	}
	for _, pair := range [][2]string{
		{"assets/master", "marketplace/plugins/sop-better/rules/master"},
		{"assets/schemas", "marketplace/plugins/sop-better/rules/schemas"},
	} {
		left, err := collectDigests(filepath.Join(root, filepath.FromSlash(pair[0])), nil)
		if err != nil {
			return fmt.Errorf("checksum snapshot tree %s: %w", pair[0], err)
		}
		right, err := collectDigests(filepath.Join(root, filepath.FromSlash(pair[1])), nil)
		if err != nil {
			return fmt.Errorf("checksum snapshot tree %s: %w", pair[1], err)
		}
		if len(left) == 0 || len(left) != len(right) {
			return fmt.Errorf("snapshot trees differ: %s and %s", pair[0], pair[1])
		}
		for index := range left {
			if left[index] != right[index] {
				return fmt.Errorf("snapshot trees differ: %s and %s", pair[0], pair[1])
			}
		}
	}
	for _, path := range []string{
		"marketplace/plugins/sop-better/skills/sop-init/SKILL.md",
		"marketplace/plugins/sop-better/skills/sop-audit/SKILL.md",
		"marketplace/plugins/sop-better/skills/sop-run/SKILL.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			return fmt.Errorf("required snapshot %s: %w", path, err)
		}
	}
	return nil
}

func verifyReleaseFiles(root string, declared []FileDigest) error {
	actual, err := collectDigests(root, map[string]bool{"release.json": true, "SHA256SUMS": true})
	if err != nil {
		return err
	}
	if len(actual) != len(declared) {
		return errors.New("release.json file list does not match bundle contents")
	}
	for index := range actual {
		if actual[index] != declared[index] {
			return fmt.Errorf("release.json checksum mismatch for %s", actual[index].Path)
		}
	}
	return nil
}

func verifyChecksumFile(root string) error {
	data, err := os.ReadFile(filepath.Join(root, "SHA256SUMS"))
	if err != nil {
		return fmt.Errorf("read SHA256SUMS: %w", err)
	}
	declared := make([]FileDigest, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		digest, path, ok := strings.Cut(line, "  ")
		if !ok || len(digest) != sha256.Size*2 || !safeRelativePath(path) {
			return fmt.Errorf("invalid SHA256SUMS line: %q", line)
		}
		declared = append(declared, FileDigest{Path: path, SHA256: strings.ToLower(digest)})
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read SHA256SUMS: %w", err)
	}
	actual, err := collectDigests(root, map[string]bool{"SHA256SUMS": true})
	if err != nil {
		return err
	}
	if len(actual) != len(declared) {
		return errors.New("SHA256SUMS file list does not match bundle contents")
	}
	for index := range actual {
		if actual[index] != declared[index] {
			return fmt.Errorf("SHA256SUMS mismatch for %s", actual[index].Path)
		}
	}
	return nil
}

func collectDigests(root string, exclude map[string]bool) ([]FileDigest, error) {
	files := make([]FileDigest, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("bundle contains symlink: %s", path)
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect bundle file %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("bundle contains non-regular file: %s", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if exclude[relative] {
			return nil
		}
		digest, err := digestFile(path)
		if err != nil {
			return err
		}
		files = append(files, FileDigest{Path: relative, SHA256: digest})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func writeChecksumFile(path string, files []FileDigest) error {
	var builder strings.Builder
	for _, file := range files {
		fmt.Fprintf(&builder, "%s  %s\n", file.SHA256, file.Path)
	}
	return os.WriteFile(path, []byte(builder.String()), 0o644)
}

func copyTree(sourceRoot, destinationRoot string) (int, error) {
	info, err := os.Stat(sourceRoot)
	if err != nil {
		return 0, fmt.Errorf("inspect required snapshot %s: %w", sourceRoot, err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("required snapshot is not a directory: %s", sourceRoot)
	}
	count := 0
	err = filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(destinationRoot, relative)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("snapshot contains symlink: %s", path)
		}
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("snapshot contains non-regular file: %s", path)
		}
		if err := copyFile(path, destination); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}

func copyFile(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", source, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source is not a regular file: %s", source)
	}
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", destination, err)
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return fmt.Errorf("copy %s: %w", source, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", destination, closeErr)
	}
	return nil
}

func copyExecutable(source, destination string) error {
	if err := copyFile(source, destination); err != nil {
		return err
	}
	if err := os.Chmod(destination, 0o755); err != nil {
		return fmt.Errorf("mark executable %s: %w", destination, err)
	}
	return nil
}

func digestFile(path string) (string, error) {
	if err := requireRegularFile(path); err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func readJSON(path string, destination any) error {
	if err := requireRegularFile(path); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return err
	}
	return nil
}

func requireRegularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("file must not be a symlink: %s", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("non-regular file: %s", path)
	}
	return nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	return nil
}

func safeRelativePath(path string) bool {
	if path == "" || strings.Contains(path, "\\") || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	return clean == path && clean != "." && !strings.HasPrefix(clean, "../")
}
