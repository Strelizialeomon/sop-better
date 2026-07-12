package integration_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Strelizialeomon/sop-better/internal/releasebundle"
)

const testReleaseVersion = "1.2.3"

func TestPublicReleaseBuildCannotBypassFormalGate(t *testing.T) {
	repoRoot := repositoryRoot(t)
	command := exec.Command("go", "run", "./cmd/sop-release", "build")
	command.Dir = repoRoot
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "use sop-release gate") {
		t.Fatalf("public build bypass error = %v, want formal-gate refusal\n%s", err, output)
	}
}

func TestBuildCreatesVersionedVerifiedBundle(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	bundleRoot := filepath.Join(t.TempDir(), "bundle")

	build := releaseBuildCommand(t, repoRoot, sourceRoot, bundleRoot, testReleaseVersion)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build release bundle: %v\n%s", err, output)
	}

	assertJSONField(t, filepath.Join(bundleRoot, "release.json"), "version", testReleaseVersion)
	assertJSONField(t, filepath.Join(bundleRoot, "release.json"), "release_notes", "Test release notes for "+testReleaseVersion)
	assertJSONField(t, filepath.Join(bundleRoot, "release.json"), "upgrade_impact", "Test upgrade impact for "+testReleaseVersion)
	assertJSONField(
		t,
		filepath.Join(bundleRoot, "marketplace", "plugins", "sop-better", ".codex-plugin", "plugin.json"),
		"version",
		testReleaseVersion,
	)
	assertJSONField(
		t,
		filepath.Join(bundleRoot, "marketplace", ".agents", "plugins", "marketplace.json"),
		"name",
		"sop-better-stable-v1-2-3",
	)

	assertSameFile(t, filepath.Join(sourceRoot, "STANDARD.md"), filepath.Join(bundleRoot, "assets", "STANDARD.md"))
	assertSameFile(t, filepath.Join(sourceRoot, "manifest.json"), filepath.Join(bundleRoot, "assets", "manifest.json"))
	assertSameFile(t, filepath.Join(sourceRoot, "master", "base.txt"), filepath.Join(bundleRoot, "assets", "master", "base.txt"))
	assertSameFile(t, filepath.Join(sourceRoot, "schemas", "profile.schema.json"), filepath.Join(bundleRoot, "assets", "schemas", "profile.schema.json"))
	assertSameFile(t, filepath.Join(sourceRoot, "skills", "sop-init", "SKILL.md"), filepath.Join(bundleRoot, "marketplace", "plugins", "sop-better", "skills", "sop-init", "SKILL.md"))
	assertSameFile(t, filepath.Join(sourceRoot, "skills", "sop-run", "SKILL.md"), filepath.Join(bundleRoot, "marketplace", "plugins", "sop-better", "skills", "sop-run", "SKILL.md"))
	assertSameFile(t, filepath.Join(sourceRoot, "STANDARD.md"), filepath.Join(bundleRoot, "marketplace", "plugins", "sop-better", "rules", "STANDARD.md"))
	for _, skill := range []string{"sop-init", "sop-audit"} {
		skillDir := filepath.Join(bundleRoot, "marketplace", "plugins", "sop-better", "skills", skill)
		for _, relative := range []string{"../../rules/STANDARD.md", "../../rules/schemas/profile.schema.json"} {
			if _, err := os.Stat(filepath.Clean(filepath.Join(skillDir, filepath.FromSlash(relative)))); err != nil {
				t.Fatalf("packaged %s cannot resolve %s from SKILL.md directory: %v", skill, relative, err)
			}
		}
	}

	verify := exec.Command("go", "run", "./cmd/sop-release", "verify", "--bundle", bundleRoot)
	verify.Dir = repoRoot
	if output, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("verify release bundle: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(bundleRoot, "SHA256SUMS")); err != nil {
		t.Fatalf("SHA256SUMS is missing: %v", err)
	}
}

func TestBuildIncludesVersionedManagerAndEngine(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	binaries := t.TempDir()
	bootstrapBinary := filepath.Join(binaries, "bootstrap-input")
	installerBinary := filepath.Join(binaries, "installer-input")
	managerBinary := filepath.Join(binaries, "manager-input")
	engineBinary := filepath.Join(binaries, "engine-input")
	writeFile(t, bootstrapBinary, []byte("fixed bootstrap protocol 1\n"))
	writeFile(t, installerBinary, []byte("installer "+testReleaseVersion+"\n"))
	writeFile(t, managerBinary, []byte("manager "+testReleaseVersion+"\n"))
	writeFile(t, engineBinary, []byte("engine "+testReleaseVersion+"\n"))
	bundleRoot := filepath.Join(t.TempDir(), "bundle")

	build := exec.Command(
		"go", "run", "./cmd/sop-release", "assemble-unverified",
		"--source", sourceRoot,
		"--plugin-root", filepath.Join(repoRoot, "plugin"),
		"--output", bundleRoot,
		"--version", testReleaseVersion,
		"--tag", "v"+testReleaseVersion,
		"--commit", strings.Repeat("a", 40),
		"--release-notes", "Test release notes for "+testReleaseVersion,
		"--upgrade-impact", "Test upgrade impact for "+testReleaseVersion,
		"--bootstrap-binary", bootstrapBinary,
		"--installer-binary", installerBinary,
		"--manager-binary", managerBinary,
		"--engine-binary", engineBinary,
		"--binary-version", testReleaseVersion,
		"--target-os", runtime.GOOS,
		"--target-arch", runtime.GOARCH,
	)
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build release bundle with executables: %v\n%s", err, output)
	}

	assertSameFile(t, managerBinary, filepath.Join(bundleRoot, "bin", "sopctl-manager"+executableSuffix()))
	assertSameFile(t, engineBinary, filepath.Join(bundleRoot, "bin", "sopctl-engine"+executableSuffix()))
	assertSameFile(t, bootstrapBinary, filepath.Join(bundleRoot, "bootstrap", "sopctl"+executableSuffix()))
	assertSameFile(t, installerBinary, filepath.Join(bundleRoot, "bin", "sop-install"+executableSuffix()))
	data, err := os.ReadFile(filepath.Join(bundleRoot, "release.json"))
	if err != nil {
		t.Fatal(err)
	}
	var release map[string]any
	if err := json.Unmarshal(data, &release); err != nil {
		t.Fatal(err)
	}
	executables := release["executables"].(map[string]any)
	bootstrap := executables["bootstrap"].(map[string]any)
	if bootstrap["protocol"] != float64(1) || len(bootstrap["sha256"].(string)) != 64 {
		t.Fatalf("bootstrap metadata is invalid: %v", bootstrap)
	}
	for _, name := range []string{"installer", "manager", "engine"} {
		metadata := executables[name].(map[string]any)
		if metadata["version"] != testReleaseVersion {
			t.Fatalf("%s version = %v, want %s", name, metadata["version"], testReleaseVersion)
		}
		if len(metadata["sha256"].(string)) != 64 {
			t.Fatalf("%s checksum is invalid", name)
		}
	}
}

func TestBundledInstallerIsSelfLocatingAndRejectsPipedConfirmationWithoutStateWrites(t *testing.T) {
	repoRoot := repositoryRoot(t)
	bundleRoot := filepath.Join(t.TempDir(), "bundle")
	buildInstallableBundle(t, repoRoot, bundleRoot, testReleaseVersion)
	installer := filepath.Join(bundleRoot, "bin", "sop-install"+executableSuffix())

	describe := exec.Command(installer, "__describe", "--json")
	describe.Dir = t.TempDir()
	description, err := describe.CombinedOutput()
	if err != nil {
		t.Fatalf("run bundled installer description away from checkout: %v\n%s", err, description)
	}
	if !strings.Contains(string(description), `"component":"installer"`) || !strings.Contains(string(description), `"version":"`+testReleaseVersion+`"`) {
		t.Fatalf("bundled installer description is incomplete: %s", description)
	}

	stateParent := t.TempDir()
	stateHome := filepath.Join(stateParent, "new-state")
	command := exec.Command(installer, "--state-home", stateHome)
	command.Dir = t.TempDir()
	command.Stdin = strings.NewReader(testReleaseVersion + "\n")
	command.Env = append(os.Environ(), "CODEX_HOME="+t.TempDir())
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "interactive TTY") {
		t.Fatalf("piped initial install error = %v, want TTY rejection\n%s", err, output)
	}
	if _, err := os.Stat(stateHome); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("non-interactive initial install changed state: %v", err)
	}
}

func TestBuildRequiresVersionedRuntimeBinaries(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	command := exec.Command(
		"go", "run", "./cmd/sop-release", "assemble-unverified",
		"--source", sourceRoot,
		"--plugin-root", filepath.Join(repoRoot, "plugin"),
		"--output", filepath.Join(t.TempDir(), "bundle"),
		"--version", testReleaseVersion,
		"--tag", "v"+testReleaseVersion,
		"--commit", strings.Repeat("a", 40),
		"--release-notes", "Test release notes for "+testReleaseVersion,
		"--upgrade-impact", "Test upgrade impact for "+testReleaseVersion,
	)
	command.Dir = repoRoot
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "must be provided together") {
		t.Fatalf("runtime-free bundle error = %v, want required binaries rejection\n%s", err, output)
	}
}

func TestInstallRejectsARebuiltBundleWithTheSameVersionAndCommit(t *testing.T) {
	repoRoot := repositoryRoot(t)
	version := testReleaseVersion
	buildBundle := func(label string) string {
		t.Helper()
		sourceRoot := createReleaseSource(t, version)
		binaries := t.TempDir()
		bootstrapBinary := filepath.Join(binaries, "bootstrap")
		installerBinary := filepath.Join(binaries, "installer")
		managerBinary := filepath.Join(binaries, "manager")
		engineBinary := filepath.Join(binaries, "engine")
		writeFile(t, bootstrapBinary, []byte("fixed bootstrap\n"))
		writeFile(t, installerBinary, []byte("installer "+label+"\n"))
		writeFile(t, managerBinary, []byte("manager "+label+"\n"))
		writeFile(t, engineBinary, []byte("engine "+label+"\n"))
		bundleRoot := filepath.Join(t.TempDir(), "bundle")
		command := exec.Command(
			"go", "run", "./cmd/sop-release", "assemble-unverified",
			"--source", sourceRoot,
			"--plugin-root", filepath.Join(repoRoot, "plugin"),
			"--output", bundleRoot,
			"--version", version,
			"--tag", "v"+version,
			"--commit", strings.Repeat("a", 40),
			"--release-notes", "Test release notes for "+version,
			"--upgrade-impact", "Test upgrade impact for "+version,
			"--bootstrap-binary", bootstrapBinary,
			"--installer-binary", installerBinary,
			"--manager-binary", managerBinary,
			"--engine-binary", engineBinary,
			"--binary-version", version,
			"--target-os", runtime.GOOS,
			"--target-arch", runtime.GOARCH,
		)
		command.Dir = repoRoot
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build %s bundle: %v\n%s", label, err, output)
		}
		return bundleRoot
	}
	first := buildBundle("first")
	second := buildBundle("second")
	destination := filepath.Join(t.TempDir(), version)
	if _, err := releasebundle.Install(first, destination); err != nil {
		t.Fatalf("install first bundle: %v", err)
	}

	if _, err := releasebundle.Install(second, destination); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("install rebuilt same-version bundle error = %v, want identity rejection", err)
	}
}

func TestBuildRejectsAuthorAbsolutePath(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	leakingSkill := []byte("---\nname: sop-init\ndescription: Initialize a test SOP.\n---\n\nAuthor checkout: " + sourceRoot + "\n")
	writeFile(t, filepath.Join(sourceRoot, "skills", "sop-init", "SKILL.md"), leakingSkill)

	build := releaseBuildCommand(t, repoRoot, sourceRoot, filepath.Join(t.TempDir(), "bundle"), testReleaseVersion)
	output, err := build.CombinedOutput()
	if err == nil {
		t.Fatalf("build unexpectedly accepted an author absolute path:\n%s", output)
	}
	if !strings.Contains(string(output), "absolute path") {
		t.Fatalf("build reported the wrong error:\n%s", output)
	}
}

func TestBuildRejectsGenericWindowsAbsolutePathOnAnyBuildHost(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	leakingSkill := []byte("---\nname: sop-init\ndescription: Initialize a test SOP.\n---\n\nPrivate checkout: D:\\private\\sop-better\n")
	writeFile(t, filepath.Join(sourceRoot, "skills", "sop-init", "SKILL.md"), leakingSkill)

	build := releaseBuildCommand(t, repoRoot, sourceRoot, filepath.Join(t.TempDir(), "bundle"), testReleaseVersion)
	output, err := build.CombinedOutput()
	if err == nil {
		t.Fatalf("build unexpectedly accepted a Windows absolute path:\n%s", output)
	}
	if !strings.Contains(string(output), "absolute path") {
		t.Fatalf("build reported the wrong error:\n%s", output)
	}
}

func TestBuildDoesNotMistakeURLPathsForAuthorFilesystemPaths(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	skillPath := filepath.Join(sourceRoot, "skills", "sop-init", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\nReference: https://example.com/home/user/guide\n")...)
	writeFile(t, skillPath, data)

	build := releaseBuildCommand(t, repoRoot, sourceRoot, filepath.Join(t.TempDir(), "bundle"), testReleaseVersion)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build confused a URL with a filesystem path: %v\n%s", err, output)
	}
}

func TestBuildRejectsAbsolutePathBehindSymlinkedSource(t *testing.T) {
	repoRoot := repositoryRoot(t)
	realSource := createReleaseSource(t, testReleaseVersion)
	leakingSkill := []byte("---\nname: sop-init\ndescription: Initialize a test SOP.\n---\n\nResolved checkout: " + realSource + "\n")
	writeFile(t, filepath.Join(realSource, "skills", "sop-init", "SKILL.md"), leakingSkill)
	sourceAlias := filepath.Join(t.TempDir(), "source-alias")
	if err := os.Symlink(realSource, sourceAlias); err != nil {
		t.Skipf("cannot create source symlink: %v", err)
	}

	build := releaseBuildCommand(t, repoRoot, sourceAlias, filepath.Join(t.TempDir(), "bundle"), testReleaseVersion)
	output, err := build.CombinedOutput()
	if err == nil {
		t.Fatalf("build unexpectedly accepted a resolved author path behind a source symlink:\n%s", output)
	}
	if !strings.Contains(string(output), "absolute path") {
		t.Fatalf("build reported the wrong error:\n%s", output)
	}
}

func TestBuildRejectsManifestVersionMismatch(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, "9.9.9")
	build := releaseBuildCommand(t, repoRoot, sourceRoot, filepath.Join(t.TempDir(), "bundle"), testReleaseVersion)
	output, err := build.CombinedOutput()
	if err == nil {
		t.Fatalf("build unexpectedly accepted inconsistent versions:\n%s", output)
	}
	if !strings.Contains(string(output), "manifest.sop_version") {
		t.Fatalf("build reported the wrong error:\n%s", output)
	}
}

func TestBuildRejectsManifestChecksumMismatch(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	mutateManifest(t, sourceRoot, func(manifest map[string]any) {
		manifest["standard"].(map[string]any)["sha256"] = strings.Repeat("0", 64)
	})
	build := releaseBuildCommand(t, repoRoot, sourceRoot, filepath.Join(t.TempDir(), "bundle"), testReleaseVersion)
	output, err := build.CombinedOutput()
	if err == nil {
		t.Fatalf("build unexpectedly accepted a bad STANDARD.md checksum:\n%s", output)
	}
	if !strings.Contains(string(output), "manifest.standard.sha256") {
		t.Fatalf("build reported the wrong error:\n%s", output)
	}
}

func TestBuildRejectsUnsupportedManifestSchemaVersion(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	mutateManifest(t, sourceRoot, func(manifest map[string]any) {
		manifest["schema_version"] = float64(2)
	})
	build := releaseBuildCommand(t, repoRoot, sourceRoot, filepath.Join(t.TempDir(), "bundle"), testReleaseVersion)
	output, err := build.CombinedOutput()
	if err == nil {
		t.Fatalf("build unexpectedly accepted manifest schema version 2:\n%s", output)
	}
	if !strings.Contains(string(output), "manifest.schema_version") || !strings.Contains(string(output), "must be 1") {
		t.Fatalf("build reported the wrong error:\n%s", output)
	}
}

func TestBuildRejectsUnsupportedProfileSchemaVersion(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	mutateManifest(t, sourceRoot, func(manifest map[string]any) {
		manifest["profile_schema_version"] = float64(2)
	})
	build := releaseBuildCommand(t, repoRoot, sourceRoot, filepath.Join(t.TempDir(), "bundle"), testReleaseVersion)
	output, err := build.CombinedOutput()
	if err == nil {
		t.Fatalf("build unexpectedly accepted profile schema version 2:\n%s", output)
	}
	if !strings.Contains(string(output), "manifest.profile_schema_version") || !strings.Contains(string(output), "must be 1") {
		t.Fatalf("build reported the wrong error:\n%s", output)
	}
}

func TestBuildRejectsMarketplacePluginMismatch(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	pluginRoot := filepath.Join(t.TempDir(), "plugin")
	copyTestTree(t, filepath.Join(repoRoot, "plugin"), pluginRoot)
	marketplacePath := filepath.Join(pluginRoot, ".agents", "plugins", "marketplace.json")
	mutateJSONFile(t, marketplacePath, func(payload map[string]any) {
		plugins := payload["plugins"].([]any)
		entry := plugins[0].(map[string]any)
		entry["source"].(map[string]any)["path"] = "./plugins/not-sop-better"
	})

	build := releaseBuildCommandWithPluginRoot(
		t,
		repoRoot,
		sourceRoot,
		pluginRoot,
		filepath.Join(t.TempDir(), "bundle"),
		testReleaseVersion,
	)
	output, err := build.CombinedOutput()
	if err == nil {
		t.Fatalf("build unexpectedly accepted a mismatched marketplace plugin:\n%s", output)
	}
	if !strings.Contains(string(output), "marketplace plugin entry") {
		t.Fatalf("build reported the wrong error:\n%s", output)
	}
}

func TestBuildRejectsOutputInsideSourceSnapshot(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	outputRoot := filepath.Join(sourceRoot, "master", "review-bundle")
	binary := buildReleaseBinary(t, repoRoot)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	args := []string{
		"assemble-unverified",
		"--source", sourceRoot,
		"--plugin-root", filepath.Join(repoRoot, "plugin"),
		"--output", outputRoot,
		"--version", testReleaseVersion,
		"--tag", "v" + testReleaseVersion,
		"--commit", strings.Repeat("a", 40),
	}
	args = append(args, releaseBinaryArgs(t, testReleaseVersion)...)
	build := exec.CommandContext(ctx, binary, args...)
	build.Dir = repoRoot
	output, err := build.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("build hung while recursively copying its own staging directory")
	}
	if err == nil {
		t.Fatalf("build unexpectedly accepted output inside master/:\n%s", output)
	}
	if !strings.Contains(string(output), "output overlaps source snapshot") {
		t.Fatalf("build reported the wrong error:\n%s", output)
	}
}

func TestBuildRejectsFIFOManifestWithoutHanging(t *testing.T) {
	mkfifo, err := exec.LookPath("mkfifo")
	if err != nil {
		t.Skip("mkfifo is not available on this platform")
	}
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	manifestPath := filepath.Join(sourceRoot, "manifest.json")
	if err := os.Remove(manifestPath); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(mkfifo, manifestPath).CombinedOutput(); err != nil {
		t.Fatalf("create FIFO: %v\n%s", err, output)
	}
	binary := buildReleaseBinary(t, repoRoot)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	args := []string{
		"assemble-unverified",
		"--source", sourceRoot,
		"--plugin-root", filepath.Join(repoRoot, "plugin"),
		"--output", filepath.Join(t.TempDir(), "bundle"),
		"--version", testReleaseVersion,
		"--tag", "v" + testReleaseVersion,
		"--commit", strings.Repeat("a", 40),
	}
	args = append(args, releaseBinaryArgs(t, testReleaseVersion)...)
	build := exec.CommandContext(ctx, binary, args...)
	build.Dir = repoRoot
	output, err := build.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("build hung while reading a FIFO manifest")
	}
	if err == nil {
		t.Fatalf("build unexpectedly accepted a FIFO manifest:\n%s", output)
	}
	if !strings.Contains(string(output), "non-regular file") {
		t.Fatalf("build reported the wrong error:\n%s", output)
	}
}

func TestBuildDoesNotRequireHomeDirectoryToExist(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	bundleRoot := filepath.Join(t.TempDir(), "bundle")
	binary := buildReleaseBinary(t, repoRoot)
	args := []string{
		"assemble-unverified",
		"--source", sourceRoot,
		"--plugin-root", filepath.Join(repoRoot, "plugin"),
		"--output", bundleRoot,
		"--version", testReleaseVersion,
		"--tag", "v" + testReleaseVersion,
		"--commit", strings.Repeat("a", 40),
	}
	args = append(args, releaseBinaryArgs(t, testReleaseVersion)...)
	build := exec.Command(binary, args...)
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "HOME="+filepath.Join(t.TempDir(), "missing-home"))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build with a nonexistent HOME: %v\n%s", err, output)
	}
}

func TestBuildIsDeterministic(t *testing.T) {
	repoRoot := repositoryRoot(t)
	firstSource := createReleaseSource(t, testReleaseVersion)
	secondSource := createReleaseSource(t, testReleaseVersion)
	firstBundle := filepath.Join(t.TempDir(), "bundle")
	secondBundle := filepath.Join(t.TempDir(), "bundle")

	for _, pair := range [][2]string{{firstSource, firstBundle}, {secondSource, secondBundle}} {
		build := releaseBuildCommand(t, repoRoot, pair[0], pair[1], testReleaseVersion)
		if output, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build release bundle: %v\n%s", err, output)
		}
	}

	first := snapshotTree(t, firstBundle)
	second := snapshotTree(t, secondBundle)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("identical release inputs produced different bundle bytes")
	}
}

func TestBuildRejectsUnsupportedReleaseVersionVariants(t *testing.T) {
	repoRoot := repositoryRoot(t)
	for _, test := range []struct {
		version string
		want    string
	}{
		{version: "1.2.3-foo", want: "must be strict semver"},
		{version: "1.2.3+foo", want: "must be strict semver"},
	} {
		version := test.version
		sourceRoot := createReleaseSource(t, version)
		bundleRoot := filepath.Join(t.TempDir(), "bundle")
		build := releaseBuildCommand(t, repoRoot, sourceRoot, bundleRoot, version)
		output, err := build.CombinedOutput()
		if err == nil || !strings.Contains(string(output), test.want) {
			t.Fatalf("release build %s error = %v, want %q\n%s", version, err, test.want, output)
		}
	}
}

func TestBuildRejectsAnEmptyGenerationContract(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	mutateManifest(t, sourceRoot, func(manifest map[string]any) {
		manifest["components"] = map[string]any{}
		manifest["outputs"] = []any{}
	})
	build := releaseBuildCommand(t, repoRoot, sourceRoot, filepath.Join(t.TempDir(), "bundle"), testReleaseVersion)
	output, err := build.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "manifest.outputs") {
		t.Fatalf("empty contract build error = %v, want manifest outputs rejection\n%s", err, output)
	}
}

func TestVerifyRejectsTamperedBundle(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	bundleRoot := filepath.Join(t.TempDir(), "bundle")
	build := releaseBuildCommand(t, repoRoot, sourceRoot, bundleRoot, testReleaseVersion)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build release bundle: %v\n%s", err, output)
	}
	writeFile(t, filepath.Join(bundleRoot, "assets", "master", "base.txt"), []byte("tampered\n"))

	verify := exec.Command("go", "run", "./cmd/sop-release", "verify", "--bundle", bundleRoot)
	verify.Dir = repoRoot
	output, err := verify.CombinedOutput()
	if err == nil {
		t.Fatalf("verify unexpectedly accepted a tampered bundle:\n%s", output)
	}
	if !strings.Contains(string(output), "snapshot trees differ") {
		t.Fatalf("verify reported the wrong error:\n%s", output)
	}
}

func TestVerifyRejectsTamperedBootstrap(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	bundleRoot := filepath.Join(t.TempDir(), "bundle")
	build := releaseBuildCommand(t, repoRoot, sourceRoot, bundleRoot, testReleaseVersion)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build release bundle: %v\n%s", err, output)
	}
	writeFile(t, filepath.Join(bundleRoot, "bootstrap", "sopctl"+executableSuffix()), []byte("tampered bootstrap\n"))

	verify := exec.Command("go", "run", "./cmd/sop-release", "verify", "--bundle", bundleRoot)
	verify.Dir = repoRoot
	output, err := verify.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "bootstrap executable checksum") {
		t.Fatalf("verify tampered bootstrap error = %v, want checksum rejection\n%s", err, output)
	}
}

func TestVerifyRejectsTamperedBundledInstaller(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	bundleRoot := filepath.Join(t.TempDir(), "bundle")
	build := releaseBuildCommand(t, repoRoot, sourceRoot, bundleRoot, testReleaseVersion)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build release bundle: %v\n%s", err, output)
	}
	writeFile(t, filepath.Join(bundleRoot, "bin", "sop-install"+executableSuffix()), []byte("tampered installer\n"))

	verify := exec.Command("go", "run", "./cmd/sop-release", "verify", "--bundle", bundleRoot)
	verify.Dir = repoRoot
	output, err := verify.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "installer executable checksum") {
		t.Fatalf("verify tampered installer error = %v, want checksum rejection\n%s", err, output)
	}
}

func TestVerifyRejectsBundledStandardPathMismatch(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	bundleRoot := filepath.Join(t.TempDir(), "bundle")
	build := releaseBuildCommand(t, repoRoot, sourceRoot, bundleRoot, testReleaseVersion)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build release bundle: %v\n%s", err, output)
	}
	mutateJSONFile(t, filepath.Join(bundleRoot, "assets", "manifest.json"), func(manifest map[string]any) {
		manifest["standard"].(map[string]any)["path"] = "rules/OTHER.md"
	})

	verify := exec.Command("go", "run", "./cmd/sop-release", "verify", "--bundle", bundleRoot)
	verify.Dir = repoRoot
	output, err := verify.CombinedOutput()
	if err == nil {
		t.Fatalf("verify unexpectedly accepted a mismatched standard.path:\n%s", output)
	}
	if !strings.Contains(string(output), "manifest.standard.path") {
		t.Fatalf("verify reported the wrong error:\n%s", output)
	}
}

func TestVerifyRejectsDivergentMasterSnapshots(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	bundleRoot := filepath.Join(t.TempDir(), "bundle")
	build := releaseBuildCommand(t, repoRoot, sourceRoot, bundleRoot, testReleaseVersion)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build release bundle: %v\n%s", err, output)
	}
	writeFile(
		t,
		filepath.Join(bundleRoot, "marketplace", "plugins", "sop-better", "rules", "master", "base.txt"),
		[]byte("different snapshot\n"),
	)

	verify := exec.Command("go", "run", "./cmd/sop-release", "verify", "--bundle", bundleRoot)
	verify.Dir = repoRoot
	output, err := verify.CombinedOutput()
	if err == nil {
		t.Fatalf("verify unexpectedly accepted divergent master snapshots:\n%s", output)
	}
	if !strings.Contains(string(output), "snapshot trees differ") {
		t.Fatalf("verify reported the wrong error:\n%s", output)
	}
}

func TestVerifyRejectsFIFOWithoutHanging(t *testing.T) {
	mkfifo, err := exec.LookPath("mkfifo")
	if err != nil {
		t.Skip("mkfifo is not available on this platform")
	}
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	bundleRoot := filepath.Join(t.TempDir(), "bundle")
	build := releaseBuildCommand(t, repoRoot, sourceRoot, bundleRoot, testReleaseVersion)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build release bundle: %v\n%s", err, output)
	}

	fifoPath := filepath.Join(bundleRoot, "assets", "master", "base.txt")
	if err := os.Remove(fifoPath); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(mkfifo, fifoPath).CombinedOutput(); err != nil {
		t.Fatalf("create FIFO: %v\n%s", err, output)
	}
	binary := buildReleaseBinary(t, repoRoot)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	verify := exec.CommandContext(ctx, binary, "verify", "--bundle", bundleRoot)
	verify.Dir = repoRoot
	output, err := verify.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("verify hung while reading a FIFO")
	}
	if err == nil {
		t.Fatalf("verify unexpectedly accepted a FIFO:\n%s", output)
	}
	if !strings.Contains(string(output), "non-regular file") {
		t.Fatalf("verify reported the wrong error:\n%s", output)
	}
}

func TestVerifySupportsBundleRootSymlink(t *testing.T) {
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	bundleRoot := filepath.Join(t.TempDir(), "bundle")
	build := releaseBuildCommand(t, repoRoot, sourceRoot, bundleRoot, testReleaseVersion)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build release bundle: %v\n%s", err, output)
	}
	bundleAlias := filepath.Join(t.TempDir(), "bundle-alias")
	if err := os.Symlink(bundleRoot, bundleAlias); err != nil {
		t.Skipf("cannot create bundle symlink: %v", err)
	}

	verify := exec.Command("go", "run", "./cmd/sop-release", "verify", "--bundle", bundleAlias)
	verify.Dir = repoRoot
	if output, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("verify through bundle root symlink: %v\n%s", err, output)
	}
}

func TestGeneratedPluginPassesOfficialValidator(t *testing.T) {
	validator := os.Getenv("SOP_PLUGIN_VALIDATOR")
	if validator == "" {
		t.Skip("set SOP_PLUGIN_VALIDATOR to run the official plugin validator")
	}
	uv, err := exec.LookPath("uv")
	if err != nil {
		t.Skip("uv is required to provide the validator's isolated PyYAML dependency")
	}
	repoRoot := repositoryRoot(t)
	sourceRoot := createReleaseSource(t, testReleaseVersion)
	bundleRoot := filepath.Join(t.TempDir(), "bundle")
	build := releaseBuildCommand(t, repoRoot, sourceRoot, bundleRoot, testReleaseVersion)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build release bundle: %v\n%s", err, output)
	}

	pluginRoot := filepath.Join(bundleRoot, "marketplace", "plugins", "sop-better")
	validate := exec.Command(uv, "run", "--with", "pyyaml", "python", validator, pluginRoot)
	validate.Dir = repoRoot
	if output, err := validate.CombinedOutput(); err != nil {
		t.Fatalf("official plugin validator failed: %v\n%s", err, output)
	}
}

func createReleaseSource(t *testing.T, version string) string {
	t.Helper()
	root := t.TempDir()
	standard := []byte("# Test standard\n\n<!-- rule:TEST-RULE -->\n")
	writeFile(t, filepath.Join(root, "STANDARD.md"), standard)
	standardSum := sha256.Sum256(standard)

	manifest := map[string]any{
		"schema_version":         1,
		"sop_version":            version,
		"profile_schema_version": 1,
		"rules_version":          "rules-test-1",
		"standard": map[string]any{
			"path":   "STANDARD.md",
			"sha256": hex.EncodeToString(standardSum[:]),
		},
		"slots": map[string]any{},
		"components": map[string]any{
			"base": map[string]any{
				"template":   "master/base.txt",
				"rule_ids":   []any{"TEST-RULE"},
				"slots":      []any{},
				"references": []any{},
			},
		},
		"outputs": []any{
			map[string]any{
				"id":           "root",
				"target":       "AGENTS.md",
				"when":         "always",
				"management":   "block",
				"marker_style": "html",
				"components": []any{
					map[string]any{"id": "base", "when": "always"},
				},
			},
		},
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	manifestData = append(manifestData, '\n')
	writeFile(t, filepath.Join(root, "manifest.json"), manifestData)

	writeFile(t, filepath.Join(root, "master", "base.txt"), []byte("base template\n"))
	writeFile(t, filepath.Join(root, "schemas", "profile.schema.json"), []byte("{\"type\":\"object\"}\n"))
	writeFile(t, filepath.Join(root, "skills", "sop-init", "SKILL.md"), []byte("---\nname: sop-init\ndescription: Initialize a test SOP.\n---\n\n# SOP init\n\nRules: ../../rules/STANDARD.md\nSchema: ../../rules/schemas/profile.schema.json\n"))
	writeFile(t, filepath.Join(root, "skills", "sop-audit", "SKILL.md"), []byte("---\nname: sop-audit\ndescription: Audit a test SOP.\n---\n\n# SOP audit\n\nRules: ../../rules/STANDARD.md\nSchema: ../../rules/schemas/profile.schema.json\n"))
	writeFile(t, filepath.Join(root, "skills", "sop-run", "SKILL.md"), []byte("---\nname: sop-run\ndescription: Run a test SOP task.\n---\n\n# SOP run\n"))
	return root
}

func mutateManifest(t *testing.T, sourceRoot string, mutate func(map[string]any)) {
	t.Helper()
	mutateJSONFile(t, filepath.Join(sourceRoot, "manifest.json"), mutate)
}

func mutateJSONFile(t *testing.T, path string, mutate func(map[string]any)) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	mutate(manifest)
	data, err = json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, append(data, '\n'))
}

func copyTestTree(t *testing.T, sourceRoot, destinationRoot string) {
	t.Helper()
	err := filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(destinationRoot, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertJSONField(t *testing.T, path, field string, want any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if got := payload[field]; got != want {
		t.Fatalf("%s field %q = %#v, want %#v", path, field, got, want)
	}
}

func jsonStringField(t *testing.T, path, field string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	value, ok := payload[field].(string)
	if !ok {
		t.Fatalf("%s field %q is not a string", path, field)
	}
	return value
}

func assertSameFile(t *testing.T, wantPath, gotPath string) {
	t.Helper()
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s does not match %s", gotPath, wantPath)
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(relative)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate integration test")
	}
	return filepath.Dir(filepath.Dir(file))
}

func buildReleaseBinary(t *testing.T, repoRoot string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "sop-release")
	compile := exec.Command("go", "build", "-o", binary, "./cmd/sop-release")
	compile.Dir = repoRoot
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("build sop-release: %v\n%s", err, output)
	}
	return binary
}

func buildInstallableBundle(t *testing.T, repoRoot, bundleRoot, version string) {
	t.Helper()
	sourceRoot := createReleaseSource(t, version)
	binaries := t.TempDir()
	bootstrap := filepath.Join(binaries, "sopctl"+executableSuffix())
	installer := filepath.Join(binaries, "sop-install"+executableSuffix())
	manager := filepath.Join(binaries, "sopctl-manager"+executableSuffix())
	engine := filepath.Join(binaries, "sopctl-engine"+executableSuffix())
	buildFixedBootstrap(t, repoRoot, bootstrap)
	buildTestManager(t, manager, version)
	buildTestEngine(t, engine, version)
	build := exec.Command(
		"go", "build", "-trimpath", "-buildvcs=false",
		"-ldflags", "-buildid= -X=github.com/Strelizialeomon/sop-better/internal/releasemanager.BuildVersion="+version,
		"-o", installer, "./cmd/sop-install",
	)
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build bundled installer: %v\n%s", err, output)
	}
	command := exec.Command(
		"go", "run", "./cmd/sop-release", "assemble-unverified",
		"--source", sourceRoot,
		"--plugin-root", filepath.Join(repoRoot, "plugin"),
		"--output", bundleRoot,
		"--version", version,
		"--tag", "v"+version,
		"--commit", strings.Repeat("a", 40),
		"--release-notes", "Test release notes for "+version,
		"--upgrade-impact", "Test upgrade impact for "+version,
		"--bootstrap-binary", bootstrap,
		"--installer-binary", installer,
		"--manager-binary", manager,
		"--engine-binary", engine,
		"--binary-version", version,
		"--target-os", runtime.GOOS,
		"--target-arch", runtime.GOARCH,
	)
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build installable release: %v\n%s", err, output)
	}
}

func releaseBuildCommand(t *testing.T, repoRoot, sourceRoot, bundleRoot, version string) *exec.Cmd {
	t.Helper()
	return releaseBuildCommandWithPluginRoot(
		t,
		repoRoot,
		sourceRoot,
		filepath.Join(repoRoot, "plugin"),
		bundleRoot,
		version,
	)
}

func releaseBuildCommandWithPluginRoot(t *testing.T, repoRoot, sourceRoot, pluginRoot, bundleRoot, version string) *exec.Cmd {
	t.Helper()
	args := []string{
		"run", "./cmd/sop-release", "assemble-unverified",
		"--source", sourceRoot,
		"--plugin-root", pluginRoot,
		"--output", bundleRoot,
		"--version", version,
		"--tag", "v" + version,
		"--commit", strings.Repeat("a", 40),
	}
	args = append(args, releaseBinaryArgs(t, version)...)
	command := exec.Command("go", args...)
	command.Dir = repoRoot
	return command
}

func releaseBinaryArgs(t *testing.T, version string) []string {
	t.Helper()
	root := t.TempDir()
	bootstrap := filepath.Join(root, "sopctl-bootstrap-input")
	installer := filepath.Join(root, "sop-install-input")
	manager := filepath.Join(root, "sopctl-manager-input")
	engine := filepath.Join(root, "sopctl-engine-input")
	writeFile(t, bootstrap, []byte("fixed test bootstrap protocol 1\n"))
	writeFile(t, installer, []byte("test installer "+version+"\n"))
	writeFile(t, manager, []byte("test manager "+version+"\n"))
	writeFile(t, engine, []byte("test engine "+version+"\n"))
	return []string{
		"--release-notes", "Test release notes for " + version,
		"--upgrade-impact", "Test upgrade impact for " + version,
		"--bootstrap-binary", bootstrap,
		"--installer-binary", installer,
		"--manager-binary", manager,
		"--engine-binary", engine,
		"--binary-version", version,
		"--target-os", runtime.GOOS,
		"--target-arch", runtime.GOARCH,
	}
}
