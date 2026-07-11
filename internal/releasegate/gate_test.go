package releasegate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyGitIdentityRequiresExactCleanTaggedCommit(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init")
	git(t, root, "config", "user.name", "Release Test")
	git(t, root, "config", "user.email", "release@example.test")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("release\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "tracked.txt")
	git(t, root, "commit", "-m", "release")
	commit := strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))
	git(t, root, "tag", "v1.2.3")
	canonical, err := canonicalDirectory(root)
	if err != nil {
		t.Fatal(err)
	}

	if err := verifyGitIdentity(canonical, "v1.2.3", commit); err != nil {
		t.Fatalf("clean tagged identity: %v", err)
	}
	if err := verifyGitIdentity(canonical, "v1.2.3", strings.Repeat("0", 40)); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("wrong commit error = %v", err)
	}
	if err := verifyGitIdentity(canonical, "v9.9.9", commit); err == nil || !strings.Contains(err.Error(), "missing or invalid") {
		t.Fatalf("missing tag error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyGitIdentity(canonical, "v1.2.3", commit); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("dirty source error = %v", err)
	}
}

func TestReleaseGateRefusesOutputInsideSourceCheckoutBeforeBuilding(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "output"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := Run(Options{SourceRoot: root, OutputRoot: filepath.Join(root, "output", "bundle")})
	if err == nil || !strings.Contains(err.Error(), "outside the source checkout") {
		t.Fatalf("inside-source output error = %v", err)
	}
}

func TestFormalGateRequiresOfficialPluginValidator(t *testing.T) {
	t.Setenv("SOP_PLUGIN_VALIDATOR", "")
	err := runOfficialPluginValidator(t.TempDir(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "SOP_PLUGIN_VALIDATOR") {
		t.Fatalf("missing validator error = %v", err)
	}
}

func git(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}
