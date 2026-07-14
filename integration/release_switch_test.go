package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Strelizialeomon/sop-better/internal/bootstrap"
	"github.com/Strelizialeomon/sop-better/internal/codexplugin"
	"github.com/Strelizialeomon/sop-better/internal/config"
	"github.com/Strelizialeomon/sop-better/internal/projectid"
	"github.com/Strelizialeomon/sop-better/internal/releasemanager"
	"github.com/Strelizialeomon/sop-better/internal/state"
)

func TestBootstrapDerivesManagerAndForwardsArgsAndExitCode(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	managerName := "sopctl-manager" + executableSuffix()
	managerPath := filepath.Join(stateHome, "versions", "1.2.3", "bin", managerName)
	buildTestManager(t, managerPath, "1.2.3")
	writeFile(t, filepath.Join(stateHome, "current.json"), []byte("{\n  \"format\": 1,\n  \"version\": \"1.2.3\",\n  \"previous\": \"1.2.2\"\n}\n"))
	bootstrap := buildGoCommand(t, repoRoot, "./cmd/sopctl-bootstrap")

	command := exec.Command(bootstrap, "alpha", "two words")
	command.Env = append(os.Environ(), "SOP_STATE_HOME="+stateHome)
	output, err := command.CombinedOutput()
	exitError, ok := err.(*exec.ExitError)
	if !ok || exitError.ExitCode() != 23 {
		t.Fatalf("bootstrap exit = %v, want manager exit 23; output:\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) != "alpha|two words" {
		t.Fatalf("bootstrap did not forward manager arguments exactly:\n%s", output)
	}
}

func TestBootstrapRejectsDamagedCurrentBeforeLaunchingAnything(t *testing.T) {
	stateHome := t.TempDir()
	writeFile(t, filepath.Join(stateHome, "current.json"), []byte("{\"format\":1,\"version\":\"../../escape\",\"previous\":\"\"}\n"))

	exitCode, err := bootstrap.Run(context.Background(), stateHome, []string{"check"}, bootstrap.Streams{
		Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), "strict semver") {
		t.Fatalf("bootstrap error = %v, want strict current.json rejection", err)
	}
	if exitCode != 1 {
		t.Fatalf("bootstrap exit = %d, want 1", exitCode)
	}
	if _, statErr := os.Stat(filepath.Join(stateHome, "escape")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("bootstrap followed a damaged version path: %v", statErr)
	}
}

func TestBootstrapBuildIsByteStableAcrossReleaseBuilds(t *testing.T) {
	repoRoot := repositoryRoot(t)
	first := filepath.Join(t.TempDir(), "sopctl"+executableSuffix())
	second := filepath.Join(t.TempDir(), "sopctl"+executableSuffix())
	for _, output := range []string{first, second} {
		command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-buildid=", "-o", output, "./cmd/sopctl-bootstrap")
		command.Dir = repoRoot
		if result, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build fixed bootstrap: %v\n%s", err, result)
		}
	}
	firstBytes, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("unchanged bootstrap source produced different release binaries")
	}
}

func TestManagerCheckAndDiffAreReadOnly(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	writeFile(t, filepath.Join(stateHome, "current.json"), []byte("{\"format\":1,\"version\":\"1.0.0\",\"previous\":\"\"}\n"))
	releaseSource := t.TempDir()
	targetBundle := filepath.Join(releaseSource, "2.0.0")
	buildSwitchBundle(t, repoRoot, targetBundle, "2.0.0")
	before := snapshotTree(t, stateHome)

	plugins := newFakePluginController(pluginRef("1.0.0"))
	var checkOutput bytes.Buffer
	checkManager := releasemanager.Manager{
		StateHome: stateHome, ReleaseSource: releaseSource, ProjectRoot: t.TempDir(),
		Plugin: plugins, Stdout: &checkOutput, Stderr: &checkOutput,
	}
	if err := checkManager.Run([]string{"release", "check"}); err != nil {
		t.Fatalf("release check: %v\n%s", err, checkOutput.String())
	}
	if !strings.Contains(checkOutput.String(), "current 1.0.0") || !strings.Contains(checkOutput.String(), "available 2.0.0") {
		t.Fatalf("release check output is incomplete:\n%s", checkOutput.String())
	}

	manager := buildGoCommand(t, repoRoot, "./cmd/sopctl-manager")
	diff := exec.Command(manager, "release", "diff", "--to", "2.0.0")
	diff.Env = append(os.Environ(), "SOP_STATE_HOME="+stateHome, "SOP_RELEASE_SOURCE="+releaseSource)
	diffOutput, err := diff.CombinedOutput()
	if err != nil {
		t.Fatalf("release diff: %v\n%s", err, diffOutput)
	}
	for _, expected := range []string{"CURRENT 1.0.0", "TARGET 2.0.0", "PLUGIN sop-better 2.0.0", "PLUGIN_CHANGE sop-better 1.0.0 -> 2.0.0", "RELEASE_NOTES", "Test release notes for 2.0.0", "UPGRADE_IMPACT", "Test upgrade impact for 2.0.0", "PROFILE_SCHEMA 1"} {
		if !strings.Contains(string(diffOutput), expected) {
			t.Fatalf("release diff is missing %q:\n%s", expected, diffOutput)
		}
	}
	for _, expected := range []string{
		"UPDATE assets/manifest.json",
		"UPDATE marketplace/plugins/sop-better/rules/manifest.json",
		"SCOPE plugin-skills",
		"SCOPE rules",
		"SCOPE master",
		"SCOPE schemas",
		"PROJECT_DIFF unavailable",
		"release switch does not modify project files",
	} {
		if !strings.Contains(string(diffOutput), expected) {
			t.Fatalf("release diff is missing real difference %q:\n%s", expected, diffOutput)
		}
	}
	after := snapshotTree(t, stateHome)
	if len(before) != len(after) {
		t.Fatalf("check/diff changed state: before=%v after=%v", before, after)
	}
	for path, content := range before {
		if after[path] != content {
			t.Fatalf("check/diff changed state file %s", path)
		}
	}
}

func TestReleaseDiffRunsVerifiedTargetEngineAgainstCandidateProfileWithoutWriting(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	writeFile(t, filepath.Join(stateHome, "current.json"), []byte("{\"format\":1,\"version\":\"1.0.0\",\"previous\":\"\"}\n"))
	releaseSource := t.TempDir()
	buildSwitchBundleWithRealEngine(t, repoRoot, filepath.Join(releaseSource, "2.0.0"), "2.0.0")

	projectRoot := t.TempDir()
	candidateProfile := filepath.Join(t.TempDir(), "profile.json")
	writeFile(t, candidateProfile, []byte(`{
  "schema_version": 1,
  "sop_version": "2.0.0",
  "project": {"name": "release-preview", "default_branch": "main", "sop_initialized_on": "2026-07-10"},
  "ends": [{"name": "backend", "path": "backend"}],
  "humans": [{"id": "owner", "roles": ["developer"]}],
  "parallel_agents": false,
  "risk": "reversible",
  "house_style": []
}
`))
	stateBefore := snapshotTree(t, stateHome)
	projectBefore := snapshotTree(t, projectRoot)

	manager := buildGoCommand(t, repoRoot, "./cmd/sopctl-manager")
	command := exec.Command(
		manager, "release", "diff", "--to", "2.0.0",
		"--project-root", projectRoot,
		"--profile", candidateProfile,
	)
	command.Env = append(os.Environ(), "SOP_STATE_HOME="+stateHome, "SOP_RELEASE_SOURCE="+releaseSource)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("release diff with target engine: %v\n%s", err, output)
	}
	for _, expected := range []string{"PROJECT_DIFF verified target engine 2.0.0", "CREATE .sop/profile.json", "CREATE AGENTS.md", "CREATE .sop/lock.json"} {
		if !strings.Contains(string(output), expected) {
			t.Fatalf("target-engine project diff is missing %q:\n%s", expected, output)
		}
	}
	if got := snapshotTree(t, stateHome); !sameSnapshot(withoutOperationLock(stateBefore), withoutOperationLock(got)) {
		t.Fatalf("release diff changed installation state: before=%v after=%v", stateBefore, got)
	}
	if got := snapshotTree(t, projectRoot); !sameSnapshot(projectBefore, got) {
		t.Fatalf("release diff changed project: before=%v after=%v", projectBefore, got)
	}
}

func TestReleaseProjectPreviewSharesTheRealProjectOperationLock(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	releaseSource := t.TempDir()
	buildSwitchBundleWithRealEngine(t, repoRoot, filepath.Join(releaseSource, "2.0.0"), "2.0.0")
	projectRoot := t.TempDir()
	candidateProfile := filepath.Join(t.TempDir(), "profile.json")
	writeFile(t, candidateProfile, []byte(`{
  "schema_version": 1,
  "sop_version": "2.0.0",
  "project": {"name": "locked-preview", "default_branch": "main", "sop_initialized_on": "2026-07-10"},
  "ends": [{"name": "backend", "path": "backend"}],
  "humans": [{"id": "owner", "roles": ["developer"]}],
  "parallel_agents": false,
  "risk": "reversible",
  "house_style": []
}
`))
	projectID, err := projectid.Identifier(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	stateBefore := snapshotTree(t, stateHome)
	held, err := state.AcquireFileLock(filepath.Join(stateHome, "projects", projectID, "operation.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	projectBefore := snapshotTree(t, projectRoot)
	manager := releasemanager.Manager{
		StateHome: stateHome, ReleaseSource: releaseSource, ProjectRoot: projectRoot,
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}

	err = manager.Run([]string{"release", "diff", "--to", "2.0.0", "--profile", candidateProfile})
	if err == nil || !strings.Contains(err.Error(), "project operation is already running") {
		t.Fatalf("concurrent project preview error = %v, want shared-lock rejection", err)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	if got := withoutOperationLock(snapshotTree(t, stateHome)); !sameSnapshot(stateBefore, got) {
		t.Fatalf("blocked project preview changed release state: before=%v after=%v", stateBefore, got)
	}
	if got := snapshotTree(t, projectRoot); !sameSnapshot(projectBefore, got) {
		t.Fatalf("blocked project preview changed project: before=%v after=%v", projectBefore, got)
	}
}

func TestReleaseDiffMarksMovedPublishedOutputIncompatibleAndUpgradeRefuses(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	writeFile(t, filepath.Join(stateHome, "current.json"), []byte("{\"format\":1,\"version\":\"1.0.0\",\"previous\":\"\"}\n"))
	releaseSource := t.TempDir()
	targetBundle := filepath.Join(releaseSource, "2.0.0")
	buildSwitchBundleWithOutputTarget(t, repoRoot, targetBundle, "2.0.0", "MOVED.md")
	before := snapshotTree(t, stateHome)

	var diffOutput bytes.Buffer
	diffManager := releasemanager.Manager{
		StateHome: stateHome, ReleaseSource: releaseSource,
		Stdout: &diffOutput, Stderr: &bytes.Buffer{},
	}
	if err := diffManager.Run([]string{"release", "diff", "--to", "2.0.0"}); err != nil {
		t.Fatalf("incompatible release diff should remain inspectable: %v\n%s", err, diffOutput.String())
	}
	if !strings.Contains(diffOutput.String(), "INCOMPATIBLE output root target AGENTS.md -> MOVED.md") {
		t.Fatalf("release diff did not mark moved output target:\n%s", diffOutput.String())
	}

	plugins := &fakePluginController{active: map[string]releasemanager.PluginRef{pluginRef("1.0.0").Selector(): pluginRef("1.0.0")}}
	upgradeManager := releasemanager.Manager{
		StateHome: stateHome, ReleaseSource: releaseSource,
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		Plugin: plugins, Confirmer: staticConfirmer{input: "2.0.0"},
	}
	err := upgradeManager.Run([]string{"release", "upgrade", "--to", "2.0.0"})
	if err == nil || !strings.Contains(err.Error(), "published output ID changed target") {
		t.Fatalf("upgrade error = %v, want moved-output compatibility rejection", err)
	}
	if len(plugins.calls) != 0 {
		t.Fatalf("incompatible upgrade touched plugins: %v", plugins.calls)
	}
	if got := snapshotTree(t, stateHome); !sameSnapshot(before, got) {
		t.Fatalf("incompatible upgrade changed installation: before=%v after=%v", before, got)
	}
}

func TestInitialInstallerCommitsCurrentLastAndInstallsFixedBootstrap(t *testing.T) {
	repoRoot := repositoryRoot(t)
	releaseChannel := t.TempDir()
	bundleRoot := filepath.Join(releaseChannel, "1.0.0")
	buildSwitchBundle(t, repoRoot, bundleRoot, "1.0.0")
	stateHome := t.TempDir()
	plugins := &fakePluginController{active: make(map[string]releasemanager.PluginRef)}
	var events []string
	installer := releasemanager.InitialInstaller{
		StateHome: stateHome, BundleRoot: bundleRoot,
		Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		AfterEvent: func(event string) error {
			events = append(events, event)
			_, err := os.Stat(filepath.Join(stateHome, "current.json"))
			if event == "current_committed" {
				if err != nil {
					t.Fatalf("current.json was not present at commit event: %v", err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("current.json existed before final commit at %s: %v", event, err)
			}
			return nil
		},
	}
	if err := installer.Install(context.Background()); err != nil {
		t.Fatalf("initial install: %v", err)
	}
	if got := strings.Join(events, ","); got != "prepared,target_plugin_ready,bootstrap_ready,source_ready,current_committed" {
		t.Fatalf("install events = %s", got)
	}
	assertCurrent(t, stateHome, "1.0.0", "")
	plugins.assertOnly(t, pluginRef("1.0.0"))
	assertSameFile(
		t,
		filepath.Join(bundleRoot, "bootstrap", "sopctl"+executableSuffix()),
		filepath.Join(stateHome, "bin", "sopctl"+executableSuffix()),
	)
	if _, err := os.Stat(filepath.Join(stateHome, "versions", "1.0.0", "release.json")); err != nil {
		t.Fatalf("verified version is not installed: %v", err)
	}
	buildSwitchBundle(t, repoRoot, filepath.Join(releaseChannel, "2.0.0"), "2.0.0")
	var checkOutput bytes.Buffer
	manager := releasemanager.Manager{StateHome: stateHome, Plugin: plugins, ProjectRoot: t.TempDir(), Stdout: &checkOutput, Stderr: &bytes.Buffer{}}
	if err := manager.Run([]string{"release", "check"}); err != nil {
		t.Fatalf("new-session release check did not use persisted source: %v", err)
	}
	if !strings.Contains(checkOutput.String(), "available 2.0.0") {
		t.Fatalf("persisted release source did not expose next version:\n%s", checkOutput.String())
	}
}

func TestSameVersionInstallerReconcilesHealthAndPrintsCustomStateUsage(t *testing.T) {
	repoRoot := repositoryRoot(t)
	releaseChannel := t.TempDir()
	bundleRoot := filepath.Join(releaseChannel, "1.0.0")
	buildSwitchBundle(t, repoRoot, bundleRoot, "1.0.0")
	stateHome := filepath.Join(t.TempDir(), "custom state")
	plugins := &fakePluginController{active: make(map[string]releasemanager.PluginRef)}
	var output bytes.Buffer
	installer := releasemanager.InitialInstaller{
		StateHome: stateHome, BundleRoot: bundleRoot,
		Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
		Stdout: &output, Stderr: &bytes.Buffer{},
	}
	if err := installer.Install(context.Background()); err != nil {
		t.Fatalf("initial install: %v", err)
	}
	bootstrapPath := filepath.Join(stateHome, "bin", "sopctl"+executableSuffix())
	stateHomeAssignment := map[bool]string{false: "SOP_STATE_HOME='", true: "$env:SOP_STATE_HOME = '"}[runtime.GOOS == "windows"]
	for _, expected := range []string{stateHomeAssignment + stateHome + "'", "'" + bootstrapPath + "'"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("custom state usage is missing %q:\n%s", expected, output.String())
		}
	}

	if err := os.Remove(bootstrapPath); err != nil {
		t.Fatal(err)
	}
	managerPath := filepath.Join(stateHome, "versions", "1.0.0", "bin", "sopctl-manager"+executableSuffix())
	if err := os.WriteFile(managerPath, []byte("damaged runtime\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	delete(plugins.active, pluginRef("1.0.0").Selector())
	manager := releasemanager.Manager{
		StateHome: stateHome, ReleaseSource: releaseChannel, ProjectRoot: t.TempDir(),
		Plugin: plugins, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}
	if err := manager.Run([]string{"release", "check"}); err == nil || !strings.Contains(err.Error(), "current release 1.0.0 is damaged") {
		t.Fatalf("damaged install health error = %v", err)
	}

	output.Reset()
	if err := installer.Install(context.Background()); err != nil {
		t.Fatalf("same-version reconciliation: %v", err)
	}
	if !strings.Contains(output.String(), "already installed and its runtime, plugin, bootstrap, and release source are healthy") {
		t.Fatalf("reconciliation result is unclear:\n%s", output.String())
	}
	if err := manager.Run([]string{"release", "check"}); err != nil {
		t.Fatalf("release health after same-version reconciliation: %v", err)
	}
	plugins.assertOnly(t, pluginRef("1.0.0"))
	if _, err := os.Stat(bootstrapPath); err != nil {
		t.Fatalf("same-version reconciliation did not restore bootstrap: %v", err)
	}

	if err := os.WriteFile(bootstrapPath, []byte("tampered fixed bootstrap\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := manager.Run([]string{"release", "check"}); err == nil || !strings.Contains(err.Error(), "move it aside") || !strings.Contains(err.Error(), bootstrapPath) {
		t.Fatalf("tampered bootstrap health guidance = %v", err)
	}
	if err := installer.Install(context.Background()); err == nil || !strings.Contains(err.Error(), "will not overwrite an unproven file") || !strings.Contains(err.Error(), "move it aside") {
		t.Fatalf("tampered bootstrap installer guidance = %v", err)
	}
	if err := os.Rename(bootstrapPath, bootstrapPath+".damaged"); err != nil {
		t.Fatal(err)
	}
	if err := installer.Install(context.Background()); err != nil {
		t.Fatalf("reconcile after moving damaged bootstrap aside: %v", err)
	}
	if err := manager.Run([]string{"release", "check"}); err != nil {
		t.Fatalf("release health after safe bootstrap recovery: %v", err)
	}
}

func TestInitialInstallerRecoversInterruptedPreCommitInstall(t *testing.T) {
	repoRoot := repositoryRoot(t)
	bundleRoot := filepath.Join(t.TempDir(), "bundle")
	buildSwitchBundle(t, repoRoot, bundleRoot, "1.0.0")
	stateHome := t.TempDir()
	plugins := &fakePluginController{active: make(map[string]releasemanager.PluginRef)}
	crashing := releasemanager.InitialInstaller{
		StateHome: stateHome, BundleRoot: bundleRoot,
		Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		AfterEvent: func(event string) error {
			if event == "source_ready" {
				return releasemanager.ErrSimulatedCrash
			}
			return nil
		},
	}
	if err := crashing.Install(context.Background()); !errors.Is(err, releasemanager.ErrSimulatedCrash) {
		t.Fatalf("crashing install error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateHome, "current.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pre-commit crash wrote current.json: %v", err)
	}

	recovered := releasemanager.InitialInstaller{
		StateHome: stateHome, BundleRoot: bundleRoot,
		Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}
	if err := recovered.Install(context.Background()); err != nil {
		t.Fatalf("recover and retry initial install: %v", err)
	}
	assertCurrent(t, stateHome, "1.0.0", "")
	plugins.assertOnly(t, pluginRef("1.0.0"))
}

func TestInitialInstallerFinishesRecoveryWhenCurrentWasCommitted(t *testing.T) {
	repoRoot := repositoryRoot(t)
	bundleRoot := filepath.Join(t.TempDir(), "bundle")
	buildSwitchBundle(t, repoRoot, bundleRoot, "1.0.0")
	stateHome := t.TempDir()
	plugins := &fakePluginController{active: make(map[string]releasemanager.PluginRef)}
	crashing := releasemanager.InitialInstaller{
		StateHome: stateHome, BundleRoot: bundleRoot,
		Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		AfterEvent: func(event string) error {
			if event == "current_committed" {
				return releasemanager.ErrSimulatedCrash
			}
			return nil
		},
	}
	if err := crashing.Install(context.Background()); !errors.Is(err, releasemanager.ErrSimulatedCrash) {
		t.Fatalf("post-commit crash error = %v", err)
	}
	assertCurrent(t, stateHome, "1.0.0", "")
	if _, err := os.Stat(filepath.Join(stateHome, "transactions", "install.json")); err != nil {
		t.Fatalf("post-commit crash did not leave recovery journal: %v", err)
	}

	recovered := releasemanager.InitialInstaller{
		StateHome: stateHome, BundleRoot: bundleRoot,
		Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}
	if err := recovered.Install(context.Background()); err != nil {
		t.Fatalf("finish committed install recovery: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateHome, "transactions", "install.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("committed install journal remains: %v", err)
	}
	assertCurrent(t, stateHome, "1.0.0", "")
	plugins.assertOnly(t, pluginRef("1.0.0"))
}

func TestInitialInstallerCompensatesPluginFailureAndNeverOverwritesBootstrap(t *testing.T) {
	repoRoot := repositoryRoot(t)
	bundleRoot := filepath.Join(t.TempDir(), "bundle")
	buildSwitchBundle(t, repoRoot, bundleRoot, "1.0.0")

	t.Run("plugin failure", func(t *testing.T) {
		stateHome := t.TempDir()
		plugins := &fakePluginController{active: make(map[string]releasemanager.PluginRef), failAt: 1}
		installer := releasemanager.InitialInstaller{
			StateHome: stateHome, BundleRoot: bundleRoot,
			Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
			Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		}
		if err := installer.Install(context.Background()); err == nil || !strings.Contains(err.Error(), "activate target plugin") {
			t.Fatalf("plugin failure error = %v", err)
		}
		for _, path := range []string{"current.json", "release-source.json", filepath.Join("versions", "1.0.0"), filepath.Join("bin", "sopctl"+executableSuffix())} {
			if _, err := os.Stat(filepath.Join(stateHome, path)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed install left %s: %v", path, err)
			}
		}
	})

	t.Run("different fixed bootstrap", func(t *testing.T) {
		stateHome := t.TempDir()
		bootstrapPath := filepath.Join(stateHome, "bin", "sopctl"+executableSuffix())
		writeFile(t, bootstrapPath, []byte("existing fixed bootstrap\n"))
		before, err := os.ReadFile(bootstrapPath)
		if err != nil {
			t.Fatal(err)
		}
		plugins := &fakePluginController{active: make(map[string]releasemanager.PluginRef)}
		installer := releasemanager.InitialInstaller{
			StateHome: stateHome, BundleRoot: bundleRoot,
			Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
			Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		}
		if err := installer.Install(context.Background()); err == nil || !strings.Contains(err.Error(), "fixed bootstrap") {
			t.Fatalf("different bootstrap error = %v", err)
		}
		after, err := os.ReadFile(bootstrapPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("initial installer overwrote a different fixed bootstrap")
		}
		if len(plugins.active) != 0 {
			t.Fatalf("failed bootstrap install left plugin active: %v", plugins.active)
		}
	})

	t.Run("different persisted release source", func(t *testing.T) {
		stateHome := t.TempDir()
		existingSource := t.TempDir()
		requestedSource := t.TempDir()
		existingConfig := fmt.Sprintf("{\n  \"format\": 1,\n  \"type\": \"local\",\n  \"root\": %q\n}\n", existingSource)
		writeFile(t, filepath.Join(stateHome, "release-source.json"), []byte(existingConfig))
		plugins := &fakePluginController{active: make(map[string]releasemanager.PluginRef)}
		installer := releasemanager.InitialInstaller{
			StateHome: stateHome, BundleRoot: bundleRoot, ReleaseSource: requestedSource,
			Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
			Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		}
		if err := installer.Install(context.Background()); err == nil || !strings.Contains(err.Error(), "refusing to replace") {
			t.Fatalf("different release source error = %v", err)
		}
		assertFileContent(t, filepath.Join(stateHome, "release-source.json"), existingConfig)
		if len(plugins.active) != 0 {
			t.Fatalf("release source conflict left plugin active: %v", plugins.active)
		}
		if _, err := os.Stat(filepath.Join(stateHome, "versions", "1.0.0")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("release source conflict left target version: %v", err)
		}
	})
}

func TestManagerCheckRejectsADamagedCandidate(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	writeFile(t, filepath.Join(stateHome, "current.json"), []byte("{\"format\":1,\"version\":\"1.0.0\",\"previous\":\"\"}\n"))
	releaseSource := t.TempDir()
	source := createReleaseSource(t, "2.0.0")
	bundle := filepath.Join(releaseSource, "2.0.0")
	command := releaseBuildCommand(t, repoRoot, source, bundle, "2.0.0")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build candidate: %v\n%s", err, output)
	}
	if err := os.Remove(filepath.Join(bundle, "bin", "sopctl-manager"+executableSuffix())); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	manager := releasemanager.Manager{
		StateHome: stateHome, ReleaseSource: releaseSource, ProjectRoot: t.TempDir(),
		Plugin: newFakePluginController(pluginRef("1.0.0")), Stdout: &output, Stderr: &output,
	}
	err := manager.Run([]string{"release", "check"})
	if err == nil || !strings.Contains(err.Error(), "verify available release") {
		t.Fatalf("release check error = %v, want damaged candidate rejection\n%s", err, output.String())
	}
}

func TestManagerCheckVerifiesRuntimePluginAndCurrentProjectWithoutWrites(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	currentRoot := filepath.Join(stateHome, "versions", "1.0.0")
	buildSwitchBundleWithRealEngine(t, repoRoot, currentRoot, "1.0.0")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	writeCompatibleProjectState(t, projectRoot, "1.0.0", "1.0.0", "rules-test-1")
	if err := os.Remove(filepath.Join(projectRoot, ".sop", "lock.json")); err != nil {
		t.Fatal(err)
	}
	render := exec.Command(filepath.Join(currentRoot, "bin", "sopctl-engine"+executableSuffix()), "render", "--project-root", projectRoot)
	render.Env = environmentForTargetEngine(stateHome, currentRoot, "1.0.0")
	if output, err := render.CombinedOutput(); err != nil {
		t.Fatalf("render genuine current project: %v\n%s", err, output)
	}
	stateBefore := snapshotTree(t, stateHome)
	projectBefore := snapshotTree(t, projectRoot)
	plugins := newFakePluginController(pluginRef("1.0.0"))
	var output bytes.Buffer
	manager := releasemanager.Manager{
		StateHome: stateHome, ReleaseSource: t.TempDir(), ProjectRoot: projectRoot,
		Plugin: plugins, Stdout: &output, Stderr: &output,
	}
	if err := manager.Run([]string{"release", "check"}); err != nil {
		t.Fatalf("release check: %v\n%s", err, output.String())
	}
	for _, expected := range []string{
		"CURRENT_RELEASE verified",
		"CURRENT_RUNTIME manager+engine protocol 1",
		"PLUGIN_HEALTH active 1.0.0",
		"PROJECT_SOP 1.0.0 PROFILE_SCHEMA 1 COMPATIBLE verified",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("release check missing %q:\n%s", expected, output.String())
		}
	}
	if got := snapshotTree(t, projectRoot); !sameSnapshot(projectBefore, got) {
		t.Fatalf("release health check changed project: before=%v after=%v", projectBefore, got)
	}
	if got := snapshotTree(t, stateHome); !sameSnapshot(withoutOperationLock(stateBefore), withoutOperationLock(got)) {
		t.Fatalf("release health check changed installation: before=%v after=%v", stateBefore, got)
	}

	plugins.healthErr = errors.New("inactive test plugin")
	if err := manager.Run([]string{"release", "check"}); err == nil || !strings.Contains(err.Error(), "current plugin health check failed") {
		t.Fatalf("inactive plugin health error = %v", err)
	}
	if len(plugins.calls) != 0 {
		t.Fatalf("read-only plugin health check mutated plugin state: %v", plugins.calls)
	}
}

func TestManagerForwardsProjectCommandsToSameVersionEngine(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	writeFile(t, filepath.Join(stateHome, "current.json"), []byte("{\"format\":1,\"version\":\"1.2.3\",\"previous\":\"1.2.2\"}\n"))
	enginePath := filepath.Join(stateHome, "versions", "1.2.3", "bin", "sopctl-engine"+executableSuffix())
	buildTestEngine(t, enginePath, "1.2.3")
	manager := buildGoCommand(t, repoRoot, "./cmd/sopctl-manager")

	command := exec.Command(manager, "check", "--project-root", "project with spaces")
	command.Env = append(os.Environ(), "SOP_STATE_HOME="+stateHome)
	output, err := command.CombinedOutput()
	exitError, ok := err.(*exec.ExitError)
	if !ok || exitError.ExitCode() != 17 {
		t.Fatalf("manager exit = %v, want engine exit 17; output:\n%s", err, output)
	}
	want := "version=1.2.3\nassets=" + filepath.Join(stateHome, "versions", "1.2.3", "assets") + "\nargs=check|--project-root|project with spaces"
	if strings.TrimSpace(string(output)) != want {
		t.Fatalf("manager did not forward to the same-version engine:\n%s", output)
	}
}

func TestVersionedManagerAndEngineDescribeTheirRuntimeContract(t *testing.T) {
	repoRoot := repositoryRoot(t)
	version := "1.2.3"
	manager := buildVersionedGoCommand(t, repoRoot, "./cmd/sopctl-manager", "github.com/Strelizialeomon/sop-better/internal/releasemanager.BuildVersion="+version)
	engine := buildVersionedGoCommand(t, repoRoot, "./cmd/sopctl-engine", "main.buildVersion="+version)

	for component, binary := range map[string]string{"manager": manager, "engine": engine} {
		command := exec.Command(binary, "__describe", "--json")
		command.Env = append(os.Environ(), "SOP_RELEASE_VERSION="+version, "SOP_ASSET_ROOT="+t.TempDir())
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("%s __describe: %v\n%s", component, err, output)
		}
		var description struct {
			Component string `json:"component"`
			Version   string `json:"version"`
			Protocol  int    `json:"protocol"`
		}
		if err := json.Unmarshal(output, &description); err != nil {
			t.Fatalf("parse %s __describe: %v\n%s", component, err, output)
		}
		if description.Component != component || description.Version != version || description.Protocol != 1 {
			t.Fatalf("%s description = %+v", component, description)
		}
	}
}

func TestVersionedEngineRejectsAMismatchedManagerVersion(t *testing.T) {
	repoRoot := repositoryRoot(t)
	engine := buildVersionedGoCommand(t, repoRoot, "./cmd/sopctl-engine", "main.buildVersion=1.2.3")
	command := exec.Command(engine, "check", "--project-root", t.TempDir())
	command.Env = append(os.Environ(), "SOP_RELEASE_VERSION=2.0.0", "SOP_ASSET_ROOT="+repoRoot)
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "version mismatch") {
		t.Fatalf("mismatched engine error = %v, want version mismatch\n%s", err, output)
	}
}

func TestVersionedEngineRendersAndChecksUsingOnlyItsManagerAssetRoot(t *testing.T) {
	repoRoot := repositoryRoot(t)
	engine := buildVersionedGoCommand(t, repoRoot, "./cmd/sopctl-engine", "main.buildVersion=0.1.0")
	assetRoot := createReleaseSource(t, "0.1.0")
	projectRoot := t.TempDir()
	profile, err := os.ReadFile(filepath.Join(repoRoot, "testdata", "fixtures", "solo-single-end", "profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(projectRoot, ".sop", "profile.json"), profile)
	environment := append(
		os.Environ(),
		"SOP_RELEASE_VERSION=0.1.0",
		"SOP_ASSET_ROOT="+assetRoot,
		"SOP_STATE_HOME="+t.TempDir(),
	)
	for _, args := range [][]string{
		{"render", "--project-root", projectRoot},
		{"check", "--project-root", projectRoot},
	} {
		command := exec.Command(engine, args...)
		command.Env = environment
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("engine %v: %v\n%s", args, err, output)
		}
	}

	command := exec.Command(engine, "check", "--project-root", projectRoot, "--asset-root", t.TempDir())
	command.Env = environment
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "flag provided but not defined") {
		t.Fatalf("engine accepted a user-selected asset root: %v\n%s", err, output)
	}
}

func TestVersionedManagerRejectsAStaleBootstrapHandoff(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "2.0.0", Previous: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	manager := buildVersionedGoCommand(t, repoRoot, "./cmd/sopctl-manager", "github.com/Strelizialeomon/sop-better/internal/releasemanager.BuildVersion=1.2.3")
	command := exec.Command(manager, "release", "check")
	command.Env = append(os.Environ(), "SOP_STATE_HOME="+stateHome, "SOP_RELEASE_SOURCE="+t.TempDir())
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "version mismatch") || !strings.Contains(string(output), "retry") {
		t.Fatalf("stale manager error = %v, want retryable version mismatch\n%s", err, output)
	}
}

func TestManagerUpgradesAndRollsBackWithoutChangingProject(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	releaseSource := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	buildSwitchBundle(t, repoRoot, filepath.Join(releaseSource, "2.0.0"), "2.0.0")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	fixedBootstrapPath := filepath.Join(stateHome, "bin", "sopctl"+executableSuffix())
	fixedBootstrapBefore, err := os.ReadFile(fixedBootstrapPath)
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	writeFile(t, filepath.Join(projectRoot, "README.md"), []byte("project must not change\n"))
	projectBefore := snapshotTree(t, projectRoot)
	plugins := newFakePluginController(pluginRef("1.0.0"))
	var output bytes.Buffer
	manager := releasemanager.Manager{
		StateHome:     stateHome,
		ReleaseSource: releaseSource,
		ProjectRoot:   projectRoot,
		Plugin:        plugins,
		Confirmer:     staticConfirmer{input: "2.0.0"},
		Stdout:        &output,
		Stderr:        &output,
	}

	if err := manager.Run([]string{"release", "upgrade", "--to", "2.0.0"}); err != nil {
		t.Fatalf("upgrade: %v\n%s", err, output.String())
	}
	assertCurrent(t, stateHome, "2.0.0", "1.0.0")
	plugins.assertOnly(t, pluginRef("2.0.0"))
	if _, err := os.Stat(filepath.Join(stateHome, "transactions", "release.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal remains after upgrade: %v", err)
	}
	if got := snapshotTree(t, projectRoot); !sameSnapshot(projectBefore, got) {
		t.Fatalf("release upgrade changed the project: before=%v after=%v", projectBefore, got)
	}
	fixedBootstrapAfter, err := os.ReadFile(fixedBootstrapPath)
	if err != nil || !bytes.Equal(fixedBootstrapBefore, fixedBootstrapAfter) {
		t.Fatalf("upgrade changed fixed bootstrap: err=%v", err)
	}
	if strings.Count(output.String(), "TARGET 2.0.0") < 1 {
		t.Fatalf("upgrade did not show the target diff:\n%s", output.String())
	}
	if !strings.Contains(output.String(), "Do not run project commands in this Codex session") {
		t.Fatalf("upgrade did not require a fresh Codex session:\n%s", output.String())
	}

	output.Reset()
	manager.Confirmer = staticConfirmer{input: "1.0.0"}
	if err := manager.Run([]string{"release", "rollback"}); err != nil {
		t.Fatalf("rollback: %v\n%s", err, output.String())
	}
	assertCurrent(t, stateHome, "1.0.0", "2.0.0")
	plugins.assertOnly(t, pluginRef("1.0.0"))
	if !strings.Contains(output.String(), "Do not run project commands in this Codex session") {
		t.Fatalf("rollback did not require a fresh Codex session:\n%s", output.String())
	}
	if got := snapshotTree(t, projectRoot); !sameSnapshot(projectBefore, got) {
		t.Fatalf("release rollback changed the project: before=%v after=%v", projectBefore, got)
	}
	fixedBootstrapAfter, err = os.ReadFile(fixedBootstrapPath)
	if err != nil || !bytes.Equal(fixedBootstrapBefore, fixedBootstrapAfter) {
		t.Fatalf("rollback changed fixed bootstrap: err=%v", err)
	}
}

func TestReleaseSwitchRejectsTargetWithDifferentFixedBootstrapBeforeChanges(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	releaseSource := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	buildSwitchBundleWithBootstrapSalt(t, repoRoot, filepath.Join(releaseSource, "2.0.0"), "2.0.0", "different-bootstrap")
	buildSwitchBundleWithBootstrapSalt(t, repoRoot, filepath.Join(stateHome, "versions", "0.9.0"), "0.9.0", "different-bootstrap")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "1.0.0", Previous: "0.9.0"}); err != nil {
		t.Fatal(err)
	}
	plugins := newFakePluginController(pluginRef("1.0.0"))
	manager := releasemanager.Manager{
		StateHome: stateHome, ReleaseSource: releaseSource, ProjectRoot: t.TempDir(),
		Plugin: plugins, Confirmer: staticConfirmer{input: "2.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}
	before := snapshotTree(t, stateHome)
	for _, args := range [][]string{
		{"release", "diff", "--to", "2.0.0"},
		{"release", "upgrade", "--to", "2.0.0"},
		{"release", "rollback"},
	} {
		err := manager.Run(args)
		if err == nil || !strings.Contains(err.Error(), "target release bootstrap is incompatible") {
			t.Fatalf("%v error = %v, want fixed-bootstrap identity rejection", args, err)
		}
	}
	assertCurrent(t, stateHome, "1.0.0", "0.9.0")
	plugins.assertOnly(t, pluginRef("1.0.0"))
	if len(plugins.calls) != 0 {
		t.Fatalf("bootstrap-incompatible targets touched plugins: %v", plugins.calls)
	}
	if got := snapshotTree(t, stateHome); !sameSnapshot(withoutOperationLock(before), withoutOperationLock(got)) {
		t.Fatalf("bootstrap-incompatible targets changed release state: before=%v after=%v", before, got)
	}
}

func TestManagerCLIRejectsPipedUpgradeConfirmationBeforeWritingState(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	releaseSource := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	buildSwitchBundle(t, repoRoot, filepath.Join(releaseSource, "2.0.0"), "2.0.0")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, stateHome)
	manager := buildGoCommand(t, repoRoot, "./cmd/sopctl-manager")
	command := exec.Command(manager, "release", "upgrade", "--to", "2.0.0")
	command.Env = append(os.Environ(), "SOP_STATE_HOME="+stateHome, "SOP_RELEASE_SOURCE="+releaseSource)
	command.Stdin = strings.NewReader("2.0.0\n")
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "TTY") {
		t.Fatalf("piped upgrade error = %v, want explicit TTY rejection\n%s", err, output)
	}
	if got := snapshotTree(t, stateHome); !sameSnapshot(before, got) {
		t.Fatalf("non-TTY upgrade changed state: before=%v after=%v", before, got)
	}
}

func TestFileLockExcludesConcurrentOwnerAndCanBeReused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.lock")
	first, err := state.AcquireFileLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.AcquireFileLock(path); !errors.Is(err, state.ErrLockBusy) {
		t.Fatalf("second owner error = %v, want ErrLockBusy", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := state.AcquireFileLock(path)
	if err != nil {
		t.Fatalf("lock could not be reused after release: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFileLockIgnoresAStalePIDFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.lock")
	writeFile(t, path, []byte("999999999\n"))

	lock, err := state.AcquireFileLock(path)
	if err != nil {
		t.Fatalf("stale lock file blocked a new process: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPluginFailuresRestorePreviousPluginAndCurrent(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run("plugin-call-"+string(rune('0'+failAt)), func(t *testing.T) {
			repoRoot := repositoryRoot(t)
			stateHome := t.TempDir()
			releaseSource := t.TempDir()
			buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
			buildSwitchBundle(t, repoRoot, filepath.Join(releaseSource, "2.0.0"), "2.0.0")
			if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "1.0.0"}); err != nil {
				t.Fatal(err)
			}
			projectRoot := t.TempDir()
			writeFile(t, filepath.Join(projectRoot, "README.md"), []byte("unchanged\n"))
			projectBefore := snapshotTree(t, projectRoot)
			plugins := newFakePluginController(pluginRef("1.0.0"))
			plugins.failAt = failAt
			manager := releasemanager.Manager{
				StateHome: stateHome, ReleaseSource: releaseSource, ProjectRoot: projectRoot,
				Plugin: plugins, Confirmer: staticConfirmer{input: "2.0.0"},
				Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
			}

			err := manager.Run([]string{"release", "upgrade", "--to", "2.0.0"})
			if err == nil || !strings.Contains(err.Error(), "injected plugin failure") {
				t.Fatalf("upgrade error = %v, want injected plugin failure", err)
			}
			assertCurrent(t, stateHome, "1.0.0", "")
			plugins.assertOnly(t, pluginRef("1.0.0"))
			if _, err := os.Stat(filepath.Join(stateHome, "transactions", "release.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("journal remains after successful compensation: %v", err)
			}
			if got := snapshotTree(t, projectRoot); !sameSnapshot(projectBefore, got) {
				t.Fatalf("failed upgrade changed project: before=%v after=%v", projectBefore, got)
			}
		})
	}
}

func TestCancelledUpgradeLeavesStateAndPluginsUntouched(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	releaseSource := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	buildSwitchBundle(t, repoRoot, filepath.Join(releaseSource, "2.0.0"), "2.0.0")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	plugins := newFakePluginController(pluginRef("1.0.0"))
	before := snapshotTree(t, stateHome)
	manager := releasemanager.Manager{
		StateHome: stateHome, ReleaseSource: releaseSource,
		Plugin: plugins, Confirmer: staticConfirmer{input: "no"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}

	err := manager.Run([]string{"release", "upgrade", "--to", "2.0.0"})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("cancelled upgrade error = %v", err)
	}
	after := snapshotTree(t, stateHome)
	if !sameSnapshot(before, after) {
		t.Fatalf("cancelled upgrade changed state: before=%v after=%v", before, after)
	}
	if len(plugins.calls) != 0 {
		t.Fatalf("cancelled upgrade called plugin controller: %v", plugins.calls)
	}
}

func TestReleaseRollbackListsHealthyInstalledFallbackAndSupportsExplicitTarget(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	for _, version := range []string{"1.0.0", "1.5.0", "2.0.0"} {
		buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", version), version)
	}
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "2.0.0", Previous: "1.5.0"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(stateHome, "versions", "1.5.0", "bin", "sopctl-manager"+executableSuffix())); err != nil {
		t.Fatal(err)
	}
	plugins := newFakePluginController(pluginRef("2.0.0"))
	manager := releasemanager.Manager{
		StateHome: stateHome, ProjectRoot: t.TempDir(),
		Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}

	err := manager.Run([]string{"release", "rollback"})
	if err == nil || !strings.Contains(err.Error(), "1.0.0") || !strings.Contains(err.Error(), "release rollback --to") {
		t.Fatalf("damaged default rollback error = %v, want healthy fallback guidance", err)
	}
	assertCurrent(t, stateHome, "2.0.0", "1.5.0")
	if len(plugins.calls) != 0 {
		t.Fatalf("fallback discovery touched plugins: %v", plugins.calls)
	}

	if err := manager.Run([]string{"release", "rollback", "--to", "1.0.0"}); err != nil {
		t.Fatalf("explicit installed rollback target: %v", err)
	}
	assertCurrent(t, stateHome, "1.0.0", "2.0.0")
	plugins.assertOnly(t, pluginRef("1.0.0"))
}

func TestReleaseRollbackExplicitTargetMustBeInstalledHealthyAndOlder(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	for _, version := range []string{"1.0.0", "2.0.0", "3.0.0"} {
		buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", version), version)
	}
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "2.0.0", Previous: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	plugins := newFakePluginController(pluginRef("2.0.0"))
	manager := releasemanager.Manager{
		StateHome: stateHome, ProjectRoot: t.TempDir(),
		Plugin: plugins, Confirmer: staticConfirmer{input: "3.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}
	for _, target := range []string{"2.0.0", "3.0.0", "0.9.0"} {
		err := manager.Run([]string{"release", "rollback", "--to", target})
		if err == nil {
			t.Fatalf("rollback accepted invalid explicit target %s", target)
		}
	}
	assertCurrent(t, stateHome, "2.0.0", "1.0.0")
	if len(plugins.calls) != 0 {
		t.Fatalf("invalid explicit rollback touched plugins: %v", plugins.calls)
	}
}

func TestReleaseRollbackRejectsAnIncompatibleProjectBeforePluginChanges(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "2.0.0"), "2.0.0")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "2.0.0", Previous: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	writeFile(t, filepath.Join(projectRoot, ".sop", "profile.json"), []byte("{\"schema_version\":1,\"sop_version\":\"2.0.0\"}\n"))
	projectBefore := snapshotTree(t, projectRoot)
	stateBefore := snapshotTree(t, stateHome)
	plugins := newFakePluginController(pluginRef("2.0.0"))
	manager := releasemanager.Manager{
		StateHome: stateHome, ProjectRoot: projectRoot,
		Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}

	err := manager.Run([]string{"release", "rollback"})
	if err == nil || !strings.Contains(err.Error(), "project rollback") {
		t.Fatalf("release rollback error = %v, want project rollback guidance", err)
	}
	assertCurrent(t, stateHome, "2.0.0", "1.0.0")
	plugins.assertOnly(t, pluginRef("2.0.0"))
	if len(plugins.calls) != 0 {
		t.Fatalf("incompatible rollback touched plugins: %v", plugins.calls)
	}
	if got := snapshotTree(t, projectRoot); !sameSnapshot(projectBefore, got) {
		t.Fatalf("incompatible rollback changed project: before=%v after=%v", projectBefore, got)
	}
	if got := snapshotTree(t, stateHome); !sameSnapshot(stateBefore, got) {
		t.Fatalf("incompatible rollback changed release state: before=%v after=%v", stateBefore, got)
	}
}

func TestReleaseRollbackAllowsAProjectAlreadyRestoredToTheTargetVersion(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	buildSwitchBundleWithRealEngine(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "2.0.0"), "2.0.0")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "2.0.0", Previous: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	writeCompatibleProjectState(t, projectRoot, "1.0.0", "1.0.0", "rules-test-1")
	if err := os.Remove(filepath.Join(projectRoot, ".sop", "lock.json")); err != nil {
		t.Fatal(err)
	}
	targetEngine := filepath.Join(stateHome, "versions", "1.0.0", "bin", "sopctl-engine"+executableSuffix())
	render := exec.Command(targetEngine, "render", "--project-root", projectRoot)
	render.Env = environmentForTargetEngine(stateHome, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	if output, err := render.CombinedOutput(); err != nil {
		t.Fatalf("render genuine rollback-target project state: %v\n%s", err, output)
	}
	projectBefore := snapshotTree(t, projectRoot)
	plugins := newFakePluginController(pluginRef("2.0.0"))
	manager := releasemanager.Manager{
		StateHome: stateHome, ProjectRoot: projectRoot,
		Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}

	if err := manager.Run([]string{"release", "rollback"}); err != nil {
		t.Fatalf("release rollback after project rollback: %v", err)
	}
	assertCurrent(t, stateHome, "1.0.0", "2.0.0")
	plugins.assertOnly(t, pluginRef("1.0.0"))
	if got := snapshotTree(t, projectRoot); !sameSnapshot(projectBefore, got) {
		t.Fatalf("release rollback changed restored project: before=%v after=%v", projectBefore, got)
	}
}

func TestReleaseRollbackTargetEngineCheckRejectsDamagedManagedOutput(t *testing.T) {
	for _, test := range []struct {
		name   string
		damage func(*testing.T, string)
	}{
		{name: "tampered", damage: func(t *testing.T, projectRoot string) {
			writeFile(t, filepath.Join(projectRoot, "AGENTS.md"), []byte("locally tampered\n"))
		}},
		{name: "missing", damage: func(t *testing.T, projectRoot string) {
			if err := os.Remove(filepath.Join(projectRoot, "AGENTS.md")); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repoRoot := repositoryRoot(t)
			stateHome := t.TempDir()
			buildSwitchBundleWithRealEngine(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
			buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "2.0.0"), "2.0.0")
			if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "2.0.0", Previous: "1.0.0"}); err != nil {
				t.Fatal(err)
			}
			projectRoot := t.TempDir()
			writeCompatibleProjectState(t, projectRoot, "1.0.0", "1.0.0", "rules-test-1")
			if err := os.Remove(filepath.Join(projectRoot, ".sop", "lock.json")); err != nil {
				t.Fatal(err)
			}
			targetRoot := filepath.Join(stateHome, "versions", "1.0.0")
			render := exec.Command(filepath.Join(targetRoot, "bin", "sopctl-engine"+executableSuffix()), "render", "--project-root", projectRoot)
			render.Env = environmentForTargetEngine(stateHome, targetRoot, "1.0.0")
			if output, err := render.CombinedOutput(); err != nil {
				t.Fatalf("render target project state: %v\n%s", err, output)
			}
			test.damage(t, projectRoot)
			projectBefore := snapshotTree(t, projectRoot)
			stateBefore := snapshotTree(t, stateHome)
			plugins := newFakePluginController(pluginRef("2.0.0"))
			manager := releasemanager.Manager{
				StateHome: stateHome, ProjectRoot: projectRoot,
				Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
				Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
			}

			err := manager.Run([]string{"release", "rollback"})
			if err == nil || !strings.Contains(err.Error(), "not valid under rollback target") {
				t.Fatalf("damaged output rollback error = %v, want target-engine check rejection", err)
			}
			assertCurrent(t, stateHome, "2.0.0", "1.0.0")
			if len(plugins.calls) != 0 {
				t.Fatalf("target check failure touched plugins: %v", plugins.calls)
			}
			if got := snapshotTree(t, projectRoot); !sameSnapshot(projectBefore, got) {
				t.Fatalf("target check failure changed project: before=%v after=%v", projectBefore, got)
			}
			if got := snapshotTree(t, stateHome); !sameSnapshot(withoutOperationLock(stateBefore), withoutOperationLock(got)) {
				t.Fatalf("target check failure changed release state: before=%v after=%v", stateBefore, got)
			}
		})
	}
}

func TestReleaseRollbackRejectsARulesMismatchedLock(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "2.0.0"), "2.0.0")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "2.0.0", Previous: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	writeCompatibleProjectState(t, projectRoot, "1.0.0", "1.0.0", "wrong-rules")
	projectBefore := snapshotTree(t, projectRoot)
	plugins := newFakePluginController(pluginRef("2.0.0"))
	manager := releasemanager.Manager{
		StateHome: stateHome, ProjectRoot: projectRoot,
		Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}

	err := manager.Run([]string{"release", "rollback"})
	if err == nil || !strings.Contains(err.Error(), "project lock") {
		t.Fatalf("rules-mismatched rollback error = %v, want lock rejection", err)
	}
	assertCurrent(t, stateHome, "2.0.0", "1.0.0")
	plugins.assertOnly(t, pluginRef("2.0.0"))
	if len(plugins.calls) != 0 {
		t.Fatalf("rules-mismatched rollback touched plugins: %v", plugins.calls)
	}
	if got := snapshotTree(t, projectRoot); !sameSnapshot(projectBefore, got) {
		t.Fatalf("rules-mismatched rollback changed project: before=%v after=%v", projectBefore, got)
	}
}

func TestReleaseRollbackFindsProjectStateFromASubdirectory(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "2.0.0"), "2.0.0")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "2.0.0", Previous: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(projectRoot, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeCompatibleProjectState(t, projectRoot, "2.0.0", "2.0.0", "rules-test-1")
	subdirectory := filepath.Join(projectRoot, "backend", "nested")
	if err := os.MkdirAll(subdirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(subdirectory)
	plugins := newFakePluginController(pluginRef("2.0.0"))
	manager := releasemanager.Manager{
		StateHome: stateHome,
		Plugin:    plugins, Confirmer: staticConfirmer{input: "1.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}

	err := manager.Run([]string{"release", "rollback"})
	if err == nil || !strings.Contains(err.Error(), "project") {
		t.Fatalf("subdirectory rollback error = %v, want discovered project rejection", err)
	}
	assertCurrent(t, stateHome, "2.0.0", "1.0.0")
	if len(plugins.calls) != 0 {
		t.Fatalf("subdirectory rollback touched plugins: %v", plugins.calls)
	}
}

func TestReleaseRollbackSharesTheProjectOperationLockWithRender(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "2.0.0"), "2.0.0")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "2.0.0", Previous: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	writeCompatibleProjectState(t, projectRoot, "1.0.0", "1.0.0", "rules-test-1")
	projectID, err := projectid.Identifier(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	held, err := state.AcquireFileLock(filepath.Join(stateHome, "projects", projectID, "operation.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	plugins := newFakePluginController(pluginRef("2.0.0"))
	manager := releasemanager.Manager{
		StateHome: stateHome, ProjectRoot: projectRoot,
		Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}

	err = manager.Run([]string{"release", "rollback"})
	if err == nil || !strings.Contains(err.Error(), "project operation is already running") {
		t.Fatalf("concurrent render lock error = %v, want shared project lock rejection", err)
	}
	assertCurrent(t, stateHome, "2.0.0", "1.0.0")
	plugins.assertOnly(t, pluginRef("2.0.0"))
	if len(plugins.calls) != 0 {
		t.Fatalf("busy project rollback touched plugins: %v", plugins.calls)
	}
}

func TestReleaseUpgradeCannotDowngradeAroundTheProjectCompatibilityGate(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	releaseSource := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "2.0.0"), "2.0.0")
	buildSwitchBundle(t, repoRoot, filepath.Join(releaseSource, "1.0.0"), "1.0.0")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "2.0.0", Previous: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	writeFile(t, filepath.Join(projectRoot, ".sop", "profile.json"), []byte("{\"schema_version\":1,\"sop_version\":\"2.0.0\"}\n"))
	stateBefore := snapshotTree(t, stateHome)
	projectBefore := snapshotTree(t, projectRoot)
	plugins := newFakePluginController(pluginRef("2.0.0"))
	manager := releasemanager.Manager{
		StateHome: stateHome, ReleaseSource: releaseSource, ProjectRoot: projectRoot,
		Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}

	err := manager.Run([]string{"release", "upgrade", "--to", "1.0.0"})
	if err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("downgrade-through-upgrade error = %v, want newer-version rejection", err)
	}
	assertCurrent(t, stateHome, "2.0.0", "1.0.0")
	plugins.assertOnly(t, pluginRef("2.0.0"))
	if len(plugins.calls) != 0 {
		t.Fatalf("downgrade-through-upgrade touched plugins: %v", plugins.calls)
	}
	if got := snapshotTree(t, stateHome); !sameSnapshot(stateBefore, got) {
		t.Fatalf("downgrade-through-upgrade changed state: before=%v after=%v", stateBefore, got)
	}
	if got := snapshotTree(t, projectRoot); !sameSnapshot(projectBefore, got) {
		t.Fatalf("downgrade-through-upgrade changed project: before=%v after=%v", projectBefore, got)
	}
}

func TestReleaseSwitchRejectsAConcurrentCurrentChangeAfterConfirmation(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	releaseSource := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "3.0.0"), "3.0.0")
	buildSwitchBundle(t, repoRoot, filepath.Join(releaseSource, "2.0.0"), "2.0.0")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	plugins := newFakePluginController(pluginRef("1.0.0"))
	manager := releasemanager.Manager{
		StateHome: stateHome, ReleaseSource: releaseSource,
		Plugin: plugins,
		Confirmer: confirmerFunc(func(_ context.Context, expected string) (bool, error) {
			if expected != "2.0.0" {
				t.Fatalf("confirmation expected %s", expected)
			}
			if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "3.0.0", Previous: "1.0.0"}); err != nil {
				return false, err
			}
			plugins.active = map[string]releasemanager.PluginRef{pluginRef("3.0.0").Selector(): pluginRef("3.0.0")}
			return true, nil
		}),
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}

	err := manager.Run([]string{"release", "upgrade", "--to", "2.0.0"})
	if err == nil || !strings.Contains(err.Error(), "changed after diff") {
		t.Fatalf("concurrent switch error = %v, want stale-confirmation rejection", err)
	}
	assertCurrent(t, stateHome, "3.0.0", "1.0.0")
	plugins.assertOnly(t, pluginRef("3.0.0"))
	if len(plugins.calls) != 0 {
		t.Fatalf("stale confirmed switch touched plugins: %v", plugins.calls)
	}
	if _, err := os.Stat(filepath.Join(stateHome, "versions", "2.0.0")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale confirmed switch installed target: %v", err)
	}
}

func TestReleaseSwitchNeverExecutesABinaryTamperedAfterConfirmation(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	releaseSource := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	targetRoot := filepath.Join(releaseSource, "2.0.0")
	buildSwitchBundle(t, repoRoot, targetRoot, "2.0.0")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(t.TempDir(), "tampered-binary-ran")
	malicious := filepath.Join(t.TempDir(), "malicious"+executableSuffix())
	buildSentinelExecutable(t, malicious, sentinel)
	plugins := newFakePluginController(pluginRef("1.0.0"))
	manager := releasemanager.Manager{
		StateHome: stateHome, ReleaseSource: releaseSource,
		Plugin: plugins,
		Confirmer: confirmerFunc(func(_ context.Context, _ string) (bool, error) {
			data, err := os.ReadFile(malicious)
			if err != nil {
				return false, err
			}
			path := filepath.Join(targetRoot, "bin", "sopctl-manager"+executableSuffix())
			return true, os.WriteFile(path, data, 0o755)
		}),
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}

	err := manager.Run([]string{"release", "upgrade", "--to", "2.0.0"})
	if err == nil {
		t.Fatal("tampered target unexpectedly upgraded")
	}
	if _, statErr := os.Stat(sentinel); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("manager executed an unverified target binary: %v", statErr)
	}
	assertCurrent(t, stateHome, "1.0.0", "")
	plugins.assertOnly(t, pluginRef("1.0.0"))
	if len(plugins.calls) != 0 {
		t.Fatalf("tampered target touched plugins: %v", plugins.calls)
	}
}

func TestCrashRecoveryUsesCurrentInsteadOfJournalPhase(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	releaseSource := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	buildSwitchBundle(t, repoRoot, filepath.Join(releaseSource, "2.0.0"), "2.0.0")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	plugins := newFakePluginController(pluginRef("1.0.0"))
	manager := releasemanager.Manager{
		StateHome: stateHome, ReleaseSource: releaseSource,
		Plugin: plugins, Confirmer: staticConfirmer{input: "2.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		AfterEvent: func(event string) error {
			if event == "target_plugin_ready" {
				return releasemanager.ErrSimulatedCrash
			}
			return nil
		},
	}

	err := manager.Run([]string{"release", "upgrade", "--to", "2.0.0"})
	if !errors.Is(err, releasemanager.ErrSimulatedCrash) {
		t.Fatalf("upgrade error = %v, want simulated crash", err)
	}
	assertCurrent(t, stateHome, "1.0.0", "")
	if len(plugins.active) != 2 {
		t.Fatalf("simulated crash did not leave both plugin states for recovery: %v", plugins.active)
	}
	journalPath := filepath.Join(stateHome, "transactions", "release.json")
	mutateJSONFile(t, journalPath, func(journal map[string]any) {
		journal["phase"] = "current_committed"
	})

	manager.AfterEvent = nil
	if err := manager.Run([]string{"release", "check"}); err != nil {
		t.Fatalf("next manager command did not recover first: %v", err)
	}
	assertCurrent(t, stateHome, "1.0.0", "")
	plugins.assertOnly(t, pluginRef("1.0.0"))
	if _, err := os.Stat(journalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal remains after recovery: %v", err)
	}
}

func TestRecoveryRefusesATamperedReleasePinBeforeTouchingPlugins(t *testing.T) {
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	releaseSource := t.TempDir()
	buildSwitchBundle(t, repoRoot, filepath.Join(stateHome, "versions", "1.0.0"), "1.0.0")
	buildSwitchBundle(t, repoRoot, filepath.Join(releaseSource, "2.0.0"), "2.0.0")
	if err := state.WriteCurrent(stateHome, state.Current{Format: 1, Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	plugins := newFakePluginController(pluginRef("1.0.0"))
	manager := releasemanager.Manager{
		StateHome: stateHome, ReleaseSource: releaseSource,
		Plugin: plugins, Confirmer: staticConfirmer{input: "2.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		AfterEvent: func(event string) error {
			if event == "target_plugin_ready" {
				return releasemanager.ErrSimulatedCrash
			}
			return nil
		},
	}
	if err := manager.Run([]string{"release", "upgrade", "--to", "2.0.0"}); !errors.Is(err, releasemanager.ErrSimulatedCrash) {
		t.Fatalf("upgrade error = %v, want simulated crash", err)
	}
	journalPath := filepath.Join(stateHome, "transactions", "release.json")
	mutateJSONFile(t, journalPath, func(journal map[string]any) {
		journal["from_commit"] = strings.Repeat("b", 40)
	})
	callsBefore := len(plugins.calls)
	manager.AfterEvent = nil

	err := manager.Run([]string{"release", "check"})
	if err == nil || !strings.Contains(err.Error(), "release pin") {
		t.Fatalf("tampered recovery error = %v, want release pin rejection", err)
	}
	if len(plugins.calls) != callsBefore {
		t.Fatalf("tampered recovery touched plugins: before=%d after=%v", callsBefore, plugins.calls)
	}
	if _, err := os.Stat(journalPath); err != nil {
		t.Fatalf("tampered recovery removed its evidence: %v", err)
	}
	assertCurrent(t, stateHome, "1.0.0", "")
}

func TestRealCodexPluginInitialInstallUpgradeAndRollbackInTemporaryHome(t *testing.T) {
	if os.Getenv("SOP_RUN_REAL_CODEX_PLUGIN_TEST") != "1" {
		t.Skip("set SOP_RUN_REAL_CODEX_PLUGIN_TEST=1 to exercise the installed Codex CLI in a temporary CODEX_HOME")
	}
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skipf("codex CLI is unavailable: %v", err)
	}
	repoRoot := repositoryRoot(t)
	stateHome := t.TempDir()
	releaseSource := t.TempDir()
	initialBundle := filepath.Join(t.TempDir(), "1.0.0")
	buildSwitchBundle(t, repoRoot, initialBundle, "1.0.0")
	buildSwitchBundle(t, repoRoot, filepath.Join(releaseSource, "2.0.0"), "2.0.0")
	temporaryCodexHome := t.TempDir()
	plugins := codexplugin.Controller{
		StateHome: stateHome,
		Runner: codexplugin.CommandRunner{Env: []string{
			"CODEX_HOME=" + temporaryCodexHome,
		}},
	}
	initialInstaller := releasemanager.InitialInstaller{
		StateHome: stateHome, BundleRoot: initialBundle,
		Plugin: plugins, Confirmer: staticConfirmer{input: "1.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}
	if err := initialInstaller.Install(context.Background()); err != nil {
		t.Fatalf("real Codex initial install: %v", err)
	}
	assertCurrent(t, stateHome, "1.0.0", "")
	t.Cleanup(func() {
		_ = plugins.EnsureAbsent(context.Background(), pluginRef("1.0.0"))
		_ = plugins.EnsureAbsent(context.Background(), pluginRef("2.0.0"))
	})
	projectRoot := t.TempDir()
	writeFile(t, filepath.Join(projectRoot, "README.md"), []byte("unchanged\n"))
	projectBefore := snapshotTree(t, projectRoot)
	manager := releasemanager.Manager{
		StateHome: stateHome, ReleaseSource: releaseSource, ProjectRoot: projectRoot,
		Plugin: plugins, Confirmer: staticConfirmer{input: "2.0.0"},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}
	if err := manager.Run([]string{"release", "upgrade", "--to", "2.0.0"}); err != nil {
		t.Fatalf("real Codex upgrade: %v", err)
	}
	assertCurrent(t, stateHome, "2.0.0", "1.0.0")
	manager.Confirmer = staticConfirmer{input: "1.0.0"}
	if err := manager.Run([]string{"release", "rollback"}); err != nil {
		t.Fatalf("real Codex rollback: %v", err)
	}
	assertCurrent(t, stateHome, "1.0.0", "2.0.0")
	if got := snapshotTree(t, projectRoot); !sameSnapshot(projectBefore, got) {
		t.Fatalf("real Codex release switch changed project: before=%v after=%v", projectBefore, got)
	}
}

type staticConfirmer struct {
	input string
}

type confirmerFunc func(context.Context, string) (bool, error)

func (confirm confirmerFunc) Confirm(ctx context.Context, expected string) (bool, error) {
	return confirm(ctx, expected)
}

func (confirmer staticConfirmer) Confirm(_ context.Context, expected string) (bool, error) {
	return confirmer.input == expected, nil
}

type fakePluginController struct {
	active    map[string]releasemanager.PluginRef
	calls     []string
	healthErr error
	failAt    int
}

func newFakePluginController(active releasemanager.PluginRef) *fakePluginController {
	return &fakePluginController{active: map[string]releasemanager.PluginRef{active.Selector(): active}}
}

func (controller *fakePluginController) EnsureActive(_ context.Context, plugin releasemanager.PluginRef) error {
	if err := controller.record("active " + plugin.Selector()); err != nil {
		return err
	}
	controller.active[plugin.Selector()] = plugin
	return nil
}

func (controller *fakePluginController) EnsureAbsent(_ context.Context, plugin releasemanager.PluginRef) error {
	if err := controller.record("absent " + plugin.Selector()); err != nil {
		return err
	}
	delete(controller.active, plugin.Selector())
	return nil
}

func (controller *fakePluginController) CheckActive(_ context.Context, plugin releasemanager.PluginRef) error {
	if controller.healthErr != nil {
		return controller.healthErr
	}
	active, ok := controller.active[plugin.Selector()]
	if !ok || active != plugin {
		return fmt.Errorf("plugin %s is not the exact active plugin", plugin.Selector())
	}
	return nil
}

func (controller *fakePluginController) record(call string) error {
	controller.calls = append(controller.calls, call)
	if controller.failAt > 0 && len(controller.calls) == controller.failAt {
		return errors.New("injected plugin failure")
	}
	return nil
}

func (controller *fakePluginController) assertOnly(t *testing.T, want releasemanager.PluginRef) {
	t.Helper()
	if len(controller.active) != 1 {
		t.Fatalf("active plugins = %v, want only %s", controller.active, want.Selector())
	}
	if _, ok := controller.active[want.Selector()]; !ok {
		t.Fatalf("active plugins = %v, want %s", controller.active, want.Selector())
	}
}

func pluginRef(version string) releasemanager.PluginRef {
	return releasemanager.PluginRef{
		Name:        "sop-better",
		Version:     version,
		Marketplace: "sop-better-stable-v" + strings.ReplaceAll(version, ".", "-"),
	}
}

func assertCurrent(t *testing.T, stateHome, version, previous string) {
	t.Helper()
	current, err := state.ReadCurrent(stateHome)
	if err != nil {
		t.Fatal(err)
	}
	if current.Version != version || current.Previous != previous {
		t.Fatalf("current = %+v, want version=%s previous=%s", current, version, previous)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", path, data, want)
	}
}

func writeCompatibleProjectState(t *testing.T, projectRoot, version, generatorVersion, rulesVersion string) {
	t.Helper()
	profile := fmt.Sprintf(`{
  "schema_version": 1,
  "sop_version": %q,
  "project": {"name": "compatibility-test", "default_branch": "main", "sop_initialized_on": "2026-07-10"},
  "ends": [{"name": "backend", "path": "backend"}],
  "humans": [{"id": "owner", "roles": ["developer"]}],
  "parallel_agents": false,
  "risk": "reversible",
  "house_style": []
}
`, version)
	parsedProfile, err := config.ParseProfile([]byte(profile))
	if err != nil {
		t.Fatal(err)
	}
	canonicalProfile, err := json.Marshal(parsedProfile)
	if err != nil {
		t.Fatal(err)
	}
	profileHash := fmt.Sprintf("%x", sha256.Sum256(canonicalProfile))
	lock := fmt.Sprintf(`{
  "schema_version": 1,
  "sop_version": %q,
  "generator_version": %q,
  "rules_version": %q,
	"profile_hash": %q,
  "outputs": []
}
`, version, generatorVersion, rulesVersion, profileHash)
	writeFile(t, filepath.Join(projectRoot, ".sop", "profile.json"), []byte(profile))
	writeFile(t, filepath.Join(projectRoot, ".sop", "lock.json"), []byte(lock))
}

func sameSnapshot(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for path, content := range left {
		if right[path] != content {
			return false
		}
	}
	return true
}

func withoutOperationLock(snapshot map[string]string) map[string]string {
	filtered := make(map[string]string, len(snapshot))
	for path, content := range snapshot {
		if strings.HasSuffix(path, "/operation.lock") {
			continue
		}
		filtered[path] = content
	}
	return filtered
}

func buildSwitchBundle(t *testing.T, repoRoot, outputRoot, version string) {
	t.Helper()
	sourceRoot := createReleaseSource(t, version)
	binaries := t.TempDir()
	bootstrapBinary := filepath.Join(binaries, "sopctl"+executableSuffix())
	managerBinary := filepath.Join(binaries, "sopctl-manager"+executableSuffix())
	engineBinary := filepath.Join(binaries, "sopctl-engine"+executableSuffix())
	buildFixedBootstrap(t, repoRoot, bootstrapBinary)
	buildTestManager(t, managerBinary, version)
	buildTestEngine(t, engineBinary, version)
	command := exec.Command(
		"go", "run", "./cmd/sop-release", "assemble-unverified",
		"--source", sourceRoot,
		"--plugin-root", filepath.Join(repoRoot, "plugin"),
		"--output", outputRoot,
		"--version", version,
		"--tag", "v"+version,
		"--commit", strings.Repeat("a", 40),
		"--release-notes", "Test release notes for "+version,
		"--upgrade-impact", "Test upgrade impact for "+version,
		"--bootstrap-binary", bootstrapBinary,
		"--installer-binary", managerBinary,
		"--manager-binary", managerBinary,
		"--engine-binary", engineBinary,
		"--binary-version", version,
		"--target-os", runtime.GOOS,
		"--target-arch", runtime.GOARCH,
	)
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build switch release %s: %v\n%s", version, err, output)
	}
	installTestFixedBootstrapForVersionRoot(t, outputRoot)
}

func buildSwitchBundleWithRealEngine(t *testing.T, repoRoot, outputRoot, version string) {
	t.Helper()
	sourceRoot := createReleaseSource(t, version)
	binaries := t.TempDir()
	bootstrapBinary := filepath.Join(binaries, "sopctl"+executableSuffix())
	managerBinary := filepath.Join(binaries, "sopctl-manager"+executableSuffix())
	engineBinary := filepath.Join(binaries, "sopctl-engine"+executableSuffix())
	buildFixedBootstrap(t, repoRoot, bootstrapBinary)
	buildTestManager(t, managerBinary, version)
	engine := buildVersionedGoCommand(t, repoRoot, "./cmd/sopctl-engine", "main.buildVersion="+version)
	data, err := os.ReadFile(engine)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, engineBinary, data)
	command := exec.Command(
		"go", "run", "./cmd/sop-release", "assemble-unverified",
		"--source", sourceRoot,
		"--plugin-root", filepath.Join(repoRoot, "plugin"),
		"--output", outputRoot,
		"--version", version,
		"--tag", "v"+version,
		"--commit", strings.Repeat("a", 40),
		"--release-notes", "Test release notes for "+version,
		"--upgrade-impact", "Test upgrade impact for "+version,
		"--bootstrap-binary", bootstrapBinary,
		"--installer-binary", managerBinary,
		"--manager-binary", managerBinary,
		"--engine-binary", engineBinary,
		"--binary-version", version,
		"--target-os", runtime.GOOS,
		"--target-arch", runtime.GOARCH,
	)
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build real-engine switch release %s: %v\n%s", version, err, output)
	}
	installTestFixedBootstrapForVersionRoot(t, outputRoot)
}

func buildSwitchBundleWithOutputTarget(t *testing.T, repoRoot, outputRoot, version, target string) {
	t.Helper()
	sourceRoot := createReleaseSource(t, version)
	mutateJSONFile(t, filepath.Join(sourceRoot, "manifest.json"), func(manifest map[string]any) {
		outputs := manifest["outputs"].([]any)
		outputs[0].(map[string]any)["target"] = target
	})
	binaries := t.TempDir()
	bootstrapBinary := filepath.Join(binaries, "sopctl"+executableSuffix())
	managerBinary := filepath.Join(binaries, "sopctl-manager"+executableSuffix())
	engineBinary := filepath.Join(binaries, "sopctl-engine"+executableSuffix())
	buildFixedBootstrap(t, repoRoot, bootstrapBinary)
	buildTestManager(t, managerBinary, version)
	buildTestEngine(t, engineBinary, version)
	command := exec.Command(
		"go", "run", "./cmd/sop-release", "assemble-unverified",
		"--source", sourceRoot,
		"--plugin-root", filepath.Join(repoRoot, "plugin"),
		"--output", outputRoot,
		"--version", version,
		"--tag", "v"+version,
		"--commit", strings.Repeat("a", 40),
		"--release-notes", "Test release notes for "+version,
		"--upgrade-impact", "Test upgrade impact for "+version,
		"--bootstrap-binary", bootstrapBinary,
		"--installer-binary", managerBinary,
		"--manager-binary", managerBinary,
		"--engine-binary", engineBinary,
		"--binary-version", version,
		"--target-os", runtime.GOOS,
		"--target-arch", runtime.GOARCH,
	)
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build moved-output switch release %s: %v\n%s", version, err, output)
	}
	installTestFixedBootstrapForVersionRoot(t, outputRoot)
}

func buildSwitchBundleWithBootstrapSalt(t *testing.T, repoRoot, outputRoot, version, salt string) {
	t.Helper()
	sourceRoot := createReleaseSource(t, version)
	binaries := t.TempDir()
	bootstrapBinary := filepath.Join(binaries, "sopctl"+executableSuffix())
	managerBinary := filepath.Join(binaries, "sopctl-manager"+executableSuffix())
	engineBinary := filepath.Join(binaries, "sopctl-engine"+executableSuffix())
	buildFixedBootstrap(t, repoRoot, bootstrapBinary)
	bootstrap, err := os.OpenFile(bootstrapBinary, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bootstrap.WriteString(salt); err != nil {
		bootstrap.Close()
		t.Fatal(err)
	}
	if err := bootstrap.Close(); err != nil {
		t.Fatal(err)
	}
	buildTestManager(t, managerBinary, version)
	buildTestEngine(t, engineBinary, version)
	command := exec.Command(
		"go", "run", "./cmd/sop-release", "assemble-unverified",
		"--source", sourceRoot,
		"--plugin-root", filepath.Join(repoRoot, "plugin"),
		"--output", outputRoot,
		"--version", version,
		"--tag", "v"+version,
		"--commit", strings.Repeat("a", 40),
		"--release-notes", "Test release notes for "+version,
		"--upgrade-impact", "Test upgrade impact for "+version,
		"--bootstrap-binary", bootstrapBinary,
		"--installer-binary", managerBinary,
		"--manager-binary", managerBinary,
		"--engine-binary", engineBinary,
		"--binary-version", version,
		"--target-os", runtime.GOOS,
		"--target-arch", runtime.GOARCH,
	)
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build different-bootstrap release %s: %v\n%s", version, err, output)
	}
}

func installTestFixedBootstrapForVersionRoot(t *testing.T, bundleRoot string) {
	t.Helper()
	versionsRoot := filepath.Dir(bundleRoot)
	if filepath.Base(versionsRoot) != "versions" {
		return
	}
	stateHome := filepath.Dir(versionsRoot)
	source := filepath.Join(bundleRoot, "bootstrap", "sopctl"+executableSuffix())
	destination := filepath.Join(stateHome, "bin", "sopctl"+executableSuffix())
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if existing, err := os.ReadFile(destination); err == nil {
		if !bytes.Equal(existing, data) {
			t.Fatalf("test releases unexpectedly use different fixed bootstraps: %s", bundleRoot)
		}
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o755); err != nil {
		t.Fatal(err)
	}
}

func buildFixedBootstrap(t *testing.T, repoRoot, output string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-buildid=", "-o", output, "./cmd/sopctl-bootstrap")
	command.Dir = repoRoot
	if result, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build fixed bootstrap: %v\n%s", err, result)
	}
}

func buildSentinelExecutable(t *testing.T, output, sentinel string) {
	t.Helper()
	source := filepath.Join(t.TempDir(), "main.go")
	program := fmt.Sprintf(`package main
import (
  "fmt"
  "os"
)
func main() {
  if err := os.WriteFile(%q, []byte("executed"), 0o600); err != nil { panic(err) }
  fmt.Println("{\"component\":\"manager\",\"version\":\"2.0.0\",\"protocol\":1}")
}
`, sentinel)
	writeFile(t, source, []byte(program))
	command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-buildid=", "-o", output, source)
	if result, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build sentinel executable: %v\n%s", err, result)
	}
}

func buildTestManager(t *testing.T, output, version string) {
	t.Helper()
	source := filepath.Join(t.TempDir(), "main.go")
	writeFile(t, source, []byte(strings.Replace(`package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "__describe" && os.Args[2] == "--json" {
		fmt.Println("{\"component\":\"manager\",\"version\":\"VERSION\",\"protocol\":1}")
		return
	}
	fmt.Println(strings.Join(os.Args[1:], "|"))
	os.Exit(23)
}
`, "VERSION", version, 1)))
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-buildid=", "-o", output, source)
	if result, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build test manager: %v\n%s", err, result)
	}
}

func buildTestEngine(t *testing.T, output, version string) {
	t.Helper()
	source := filepath.Join(t.TempDir(), "main.go")
	writeFile(t, source, []byte(strings.Replace(`package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "__describe" && os.Args[2] == "--json" {
		fmt.Println("{\"component\":\"engine\",\"version\":\"VERSION\",\"protocol\":1}")
		return
	}
	fmt.Println("version=" + os.Getenv("SOP_RELEASE_VERSION"))
	fmt.Println("assets=" + os.Getenv("SOP_ASSET_ROOT"))
	fmt.Println("args=" + strings.Join(os.Args[1:], "|"))
	os.Exit(17)
}
`, "VERSION", version, 1)))
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-buildid=", "-o", output, source)
	if result, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build test engine: %v\n%s", err, result)
	}
}

func buildGoCommand(t *testing.T, repoRoot, packagePath string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), filepath.Base(packagePath)+executableSuffix())
	command := exec.Command("go", "build", "-o", binary, packagePath)
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", packagePath, err, output)
	}
	return binary
}

func buildVersionedGoCommand(t *testing.T, repoRoot, packagePath, assignment string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), filepath.Base(packagePath)+executableSuffix())
	command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags", "-buildid= -X="+assignment, "-o", binary, packagePath)
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build versioned %s: %v\n%s", packagePath, err, output)
	}
	return binary
}

func executableSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func environmentForTargetEngine(stateHome, versionRoot, version string) []string {
	return append(
		os.Environ(),
		"SOP_STATE_HOME="+stateHome,
		"SOP_RELEASE_VERSION="+version,
		"SOP_ASSET_ROOT="+filepath.Join(versionRoot, "assets"),
	)
}
