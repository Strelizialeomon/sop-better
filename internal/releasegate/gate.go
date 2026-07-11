package releasegate

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Strelizialeomon/sop-better/internal/releasebundle"
)

type Options struct {
	SourceRoot    string
	PluginRoot    string
	OutputRoot    string
	Version       string
	GitTag        string
	GitCommit     string
	ReleaseNotes  string
	UpgradeImpact string
	TargetOS      string
	TargetArch    string
}

func Run(options Options) error {
	sourceRoot, err := canonicalDirectory(options.SourceRoot)
	if err != nil {
		return err
	}
	outputRoot, err := canonicalFuturePath(options.OutputRoot)
	if err != nil {
		return fmt.Errorf("resolve release output: %w", err)
	}
	if within(outputRoot, sourceRoot) {
		return errors.New("release gate output must be outside the source checkout so the Git identity remains clean")
	}
	if err := verifyGitIdentity(sourceRoot, options.GitTag, options.GitCommit); err != nil {
		return err
	}
	pluginRoot := options.PluginRoot
	if !filepath.IsAbs(pluginRoot) {
		pluginRoot = filepath.Join(sourceRoot, pluginRoot)
	}
	pluginRoot, err = canonicalDirectory(pluginRoot)
	if err != nil {
		return fmt.Errorf("resolve plugin root: %w", err)
	}
	if !within(pluginRoot, sourceRoot) {
		return errors.New("formal release plugin root must be inside the verified source checkout")
	}
	if err := runQualitySuite(sourceRoot); err != nil {
		return err
	}
	if err := runOfficialPluginValidator(sourceRoot, filepath.Join(pluginRoot, "plugins", "sop-better")); err != nil {
		return err
	}
	if err := verifyGitIdentity(sourceRoot, options.GitTag, options.GitCommit); err != nil {
		return fmt.Errorf("source changed while running release quality gates: %w", err)
	}

	binaries, err := os.MkdirTemp("", ".sop-release-binaries-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(binaries)
	suffix := ""
	if options.TargetOS == "windows" {
		suffix = ".exe"
	}
	bootstrap := filepath.Join(binaries, "sopctl"+suffix)
	installer := filepath.Join(binaries, "sop-install"+suffix)
	manager := filepath.Join(binaries, "sopctl-manager"+suffix)
	engine := filepath.Join(binaries, "sopctl-engine"+suffix)
	builds := []struct {
		output  string
		pkg     string
		ldflags string
	}{
		{output: bootstrap, pkg: "./cmd/sopctl-bootstrap", ldflags: "-buildid="},
		{output: installer, pkg: "./cmd/sop-install", ldflags: "-buildid= -X github.com/Strelizialeomon/sop-better/internal/releasemanager.BuildVersion=" + options.Version},
		{output: manager, pkg: "./cmd/sopctl-manager", ldflags: "-buildid= -X github.com/Strelizialeomon/sop-better/internal/releasemanager.BuildVersion=" + options.Version},
		{output: engine, pkg: "./cmd/sopctl-engine", ldflags: "-buildid= -X main.buildVersion=" + options.Version},
	}
	for _, build := range builds {
		if err := runCommand(sourceRoot, targetEnvironment(options.TargetOS, options.TargetArch), "go", "build", "-trimpath", "-buildvcs=false", "-ldflags", build.ldflags, "-o", build.output, build.pkg); err != nil {
			return fmt.Errorf("build %s from release commit: %w", filepath.Base(build.output), err)
		}
	}
	if err := verifyGitIdentity(sourceRoot, options.GitTag, options.GitCommit); err != nil {
		return fmt.Errorf("source changed while building release binaries: %w", err)
	}
	if err := releasebundle.Build(releasebundle.Options{
		SourceRoot: sourceRoot, PluginRoot: pluginRoot, OutputRoot: outputRoot,
		Version: options.Version, GitTag: options.GitTag, GitCommit: options.GitCommit,
		ReleaseNotes: options.ReleaseNotes, UpgradeImpact: options.UpgradeImpact,
		BootstrapBinary: bootstrap, InstallerBinary: installer, ManagerBinary: manager, EngineBinary: engine,
		BinaryVersion: options.Version, TargetOS: options.TargetOS, TargetArch: options.TargetArch,
	}); err != nil {
		return err
	}
	if err := runOfficialPluginValidator(sourceRoot, filepath.Join(outputRoot, "marketplace", "plugins", "sop-better")); err != nil {
		_ = os.RemoveAll(outputRoot)
		return fmt.Errorf("validate generated release plugin: %w", err)
	}
	if err := verifyGitIdentity(sourceRoot, options.GitTag, options.GitCommit); err != nil {
		_ = os.RemoveAll(outputRoot)
		return fmt.Errorf("source changed while assembling release bundle: %w", err)
	}
	return nil
}

func verifyGitIdentity(sourceRoot, tag, commit string) error {
	top, err := commandOutput(sourceRoot, nil, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("release source is not a Git checkout: %w", err)
	}
	canonicalTop, err := canonicalDirectory(strings.TrimSpace(top))
	if err != nil {
		return err
	}
	if canonicalTop != sourceRoot {
		return errors.New("release source must be the Git checkout root")
	}
	head, err := commandOutput(sourceRoot, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(head), commit) {
		return fmt.Errorf("release commit %s does not match source HEAD %s", commit, strings.TrimSpace(head))
	}
	peeledTag, err := commandOutput(sourceRoot, nil, "git", "rev-parse", "refs/tags/"+tag+"^{commit}")
	if err != nil {
		return fmt.Errorf("release tag %s is missing or invalid: %w", tag, err)
	}
	if !strings.EqualFold(strings.TrimSpace(peeledTag), commit) {
		return fmt.Errorf("release tag %s does not point to commit %s", tag, commit)
	}
	status, err := commandOutput(sourceRoot, nil, "git", "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("release source is dirty:\n%s", strings.TrimSpace(status))
	}
	return nil
}

func runQualitySuite(sourceRoot string) error {
	hostEnvironment := hostGoEnvironment()
	for _, command := range [][]string{
		{"git", "diff-tree", "--check", "--root", "-r", "HEAD"},
		{"go", "vet", "./..."},
		{"go", "test", "./...", "-count=1"},
	} {
		if err := runCommand(sourceRoot, hostEnvironment, command[0], command[1:]...); err != nil {
			return fmt.Errorf("release quality gate %s: %w", strings.Join(command, " "), err)
		}
	}
	return nil
}

func runOfficialPluginValidator(sourceRoot, pluginPath string) error {
	validator := strings.TrimSpace(os.Getenv("SOP_PLUGIN_VALIDATOR"))
	if validator == "" {
		return errors.New("SOP_PLUGIN_VALIDATOR must point to the official plugin-creator validate_plugin.py for a formal release")
	}
	validator, err := filepath.Abs(validator)
	if err != nil {
		return fmt.Errorf("resolve SOP_PLUGIN_VALIDATOR: %w", err)
	}
	info, err := os.Lstat(validator)
	if err != nil {
		return fmt.Errorf("inspect SOP_PLUGIN_VALIDATOR: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("SOP_PLUGIN_VALIDATOR must be a regular file")
	}
	if err := runCommand(sourceRoot, hostGoEnvironment(), "uv", "run", "--with", "pyyaml", "python", validator, pluginPath); err != nil {
		return fmt.Errorf("official plugin validator: %w", err)
	}
	return nil
}

func runCommand(directory string, environment []string, name string, args ...string) error {
	_, err := commandOutput(directory, environment, name, args...)
	return err
}

func commandOutput(directory string, environment []string, name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	command.Dir = directory
	if environment != nil {
		command.Env = environment
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, detail)
	}
	return stdout.String(), nil
}

func hostGoEnvironment() []string {
	return overrideEnvironment(os.Environ(), map[string]string{
		"GOOS": runtime.GOOS, "GOARCH": runtime.GOARCH,
	})
}

func targetEnvironment(goos, goarch string) []string {
	return overrideEnvironment(os.Environ(), map[string]string{
		"CGO_ENABLED": "0", "GOOS": goos, "GOARCH": goarch,
	})
}

func overrideEnvironment(base []string, overrides map[string]string) []string {
	result := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, found := strings.Cut(entry, "=")
		if found {
			if _, exists := overrides[key]; exists {
				continue
			}
		}
		result = append(result, entry)
	}
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}
	return result
}

func canonicalDirectory(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("release source must be a directory")
	}
	return filepath.Clean(resolved), nil
}

func canonicalFuturePath(path string) (string, error) {
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
	resolvedParent, err := canonicalFuturePath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedParent, filepath.Base(absolute)), nil
}

func within(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && (relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))))
}
