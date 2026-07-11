package acceptance_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestLegalProfileMatrixKeepsStructureTriggersOrthogonal(t *testing.T) {
	repoRoot := matrixRepositoryRoot(t)
	sopctl := buildMatrixSopctl(t, repoRoot)

	for humans := 1; humans <= 2; humans++ {
		for ends := 1; ends <= 2; ends++ {
			for _, parallel := range []bool{false, true} {
				name := fmt.Sprintf("humans=%d/ends=%d/parallel=%t", humans, ends, parallel)
				t.Run(name, func(t *testing.T) {
					branch := fmt.Sprintf("trunk-h%d-e%d-p%t", humans, ends, parallel)
					profile := matrixProfile(t, humans, ends, parallel, branch)

					if ends == 1 && parallel {
						assertSingleEndParallelIsRejected(t, sopctl, repoRoot, profile)
						return
					}

					firstRoot := t.TempDir()
					secondRoot := t.TempDir()
					writeMatrixProfile(t, firstRoot, profile)
					writeMatrixProfile(t, secondRoot, profile)

					renderAndCheckMatrixProject(t, sopctl, repoRoot, firstRoot)
					renderAndCheckMatrixProject(t, sopctl, repoRoot, secondRoot)

					first := matrixGeneratedSnapshot(t, firstRoot)
					second := matrixGeneratedSnapshot(t, secondRoot)
					assertMatrixSnapshotsEqual(t, first, second)
					assertMatrixTriggers(t, first, humans, ends, parallel)
					assertMatrixUsesProfileBranch(t, first, branch)
				})
			}
		}
	}
}

func matrixRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate legal matrix test file")
	}
	return filepath.Dir(filepath.Dir(file))
}

func buildMatrixSopctl(t *testing.T, repoRoot string) string {
	t.Helper()
	name := "sopctl"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", path, "./cmd/sopctl-engine")
	cmd.Dir = repoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build sopctl: %v\n%s", err, output)
	}
	return path
}

func matrixProfile(t *testing.T, humans int, ends int, parallel bool, branch string) []byte {
	t.Helper()
	endValues := []map[string]any{
		{
			"name":         "backend",
			"display_name": "Backend",
			"path":         "backend",
			"stack":        "Go",
			"docs":         []string{"README.md"},
		},
	}
	if ends == 2 {
		endValues = append(endValues, map[string]any{
			"name":         "frontend",
			"display_name": "Frontend",
			"path":         "frontend",
			"stack":        "React",
			"docs":         []string{"README.md"},
		})
	}

	humanValues := []map[string]any{{"id": "owner", "roles": []string{"product", "developer"}}}
	if humans == 2 {
		humanValues = []map[string]any{
			{"id": "product-owner", "roles": []string{"product"}},
			{"id": "developer", "roles": []string{"developer"}},
		}
	}

	profile := map[string]any{
		"schema_version":  1,
		"ends":            endValues,
		"house_style":     []string{},
		"humans":          humanValues,
		"parallel_agents": parallel,
		"project": map[string]any{
			"default_branch":     branch,
			"description":        "Legal trigger matrix fixture",
			"name":               fmt.Sprintf("matrix-h%d-e%d-p%t", humans, ends, parallel),
			"sop_initialized_on": "2026-07-10",
		},
		"risk":        "reversible",
		"risk_items":  []string{"push protected branch"},
		"sop_version": "0.1.0",
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}

func writeMatrixProfile(t *testing.T, projectRoot string, profile []byte) {
	t.Helper()
	dir := filepath.Join(projectRoot, ".sop")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "profile.json"), profile, 0o644); err != nil {
		t.Fatal(err)
	}
}

func renderAndCheckMatrixProject(t *testing.T, sopctl string, assetRoot string, projectRoot string) {
	t.Helper()
	if output, err := runMatrixSopctl(sopctl, projectRoot, assetRoot, "render"); err != nil {
		t.Fatalf("render legal profile: %v\n%s", err, output)
	}
	if output, err := runMatrixSopctl(sopctl, projectRoot, assetRoot, "check"); err != nil {
		t.Fatalf("check rendered legal profile: %v\n%s", err, output)
	}
}

func runMatrixSopctl(sopctl string, projectRoot string, assetRoot string, command string) ([]byte, error) {
	cmd := exec.Command(
		sopctl,
		command,
		"--project-root", projectRoot,
	)
	cmd.Env = append(os.Environ(),
		"SOP_RELEASE_VERSION=0.1.0-dev",
		"SOP_ASSET_ROOT="+assetRoot,
	)
	return cmd.CombinedOutput()
}

func assertSingleEndParallelIsRejected(t *testing.T, sopctl string, assetRoot string, profile []byte) {
	t.Helper()
	projectRoot := t.TempDir()
	writeMatrixProfile(t, projectRoot, profile)
	output, err := runMatrixSopctl(sopctl, projectRoot, assetRoot, "render")
	if err == nil {
		t.Fatalf("single-end parallel profile unexpectedly rendered:\n%s", output)
	}
	if !strings.Contains(string(output), "profile.parallel_agents: requires at least 2 ends") {
		t.Fatalf("single-end parallel profile returned the wrong error:\n%s", output)
	}
	if generated := matrixGeneratedSnapshot(t, projectRoot); len(generated) != 0 {
		t.Fatalf("rejected profile wrote generated state: %v", matrixSnapshotPaths(generated))
	}
}

func matrixGeneratedSnapshot(t *testing.T, projectRoot string) map[string][]byte {
	t.Helper()
	snapshot := make(map[string][]byte)
	err := filepath.WalkDir(projectRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == ".sop/profile.json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[relative] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func assertMatrixSnapshotsEqual(t *testing.T, first map[string][]byte, second map[string][]byte) {
	t.Helper()
	firstPaths := matrixSnapshotPaths(first)
	secondPaths := matrixSnapshotPaths(second)
	if fmt.Sprint(firstPaths) != fmt.Sprint(secondPaths) {
		t.Fatalf("same profile rendered different file trees:\nfirst:  %v\nsecond: %v", firstPaths, secondPaths)
	}
	for _, path := range firstPaths {
		if !bytes.Equal(first[path], second[path]) {
			firstHash := sha256.Sum256(first[path])
			secondHash := sha256.Sum256(second[path])
			t.Fatalf("same profile rendered different %s bytes: first=%x second=%x", path, firstHash, secondHash)
		}
		if bytes.Contains(first[path], []byte{'\r'}) {
			t.Fatalf("fresh render %s is not canonical LF text", path)
		}
	}
}

func assertMatrixTriggers(t *testing.T, snapshot map[string][]byte, humans int, ends int, parallel bool) {
	t.Helper()
	wantCollaborationFile := humans == 2 || parallel
	wantMultiend := ends == 2

	assertMatrixPath(t, snapshot, "docs/project/collaboration.md", wantCollaborationFile)
	assertMatrixPath(t, snapshot, "docs/contracts/README.md", wantMultiend)
	assertMatrixPath(t, snapshot, "docs/contracts/multiend-contracts.md", wantMultiend)
	assertMatrixPath(t, snapshot, "backend/AGENTS.md", wantMultiend)
	assertMatrixPath(t, snapshot, "frontend/AGENTS.md", wantMultiend)
	assertMatrixPath(t, snapshot, "docs/project/worktree-isolation.md", parallel)

	if wantCollaborationFile {
		collaboration := string(snapshot["docs/project/collaboration.md"])
		assertMatrixText(t, "human collaboration block", collaboration, "## 真人协作者 handoff", humans == 2)
		assertMatrixText(t, "parallel coordination block", collaboration, "## 多端多 agent 协调", parallel)
	}

	if wantMultiend {
		for _, path := range []string{"backend/AGENTS.md", "frontend/AGENTS.md"} {
			contents := string(snapshot[path])
			assertMatrixText(t, path+" scope trigger", contents, "scope:", parallel)
			assertMatrixText(t, path+" worktree trigger", strings.ToLower(contents), "worktree", parallel)
		}
	}
}

func assertMatrixPath(t *testing.T, snapshot map[string][]byte, path string, want bool) {
	t.Helper()
	_, got := snapshot[path]
	if got != want {
		t.Errorf("generated path %s present=%t, want %t; tree=%v", path, got, want, matrixSnapshotPaths(snapshot))
	}
}

func assertMatrixText(t *testing.T, label string, contents string, fragment string, want bool) {
	t.Helper()
	got := strings.Contains(contents, fragment)
	if got != want {
		t.Errorf("%s contains %q=%t, want %t", label, fragment, got, want)
	}
}

var matrixOriginRefPattern = regexp.MustCompile(`origin/[A-Za-z0-9._/-]+`)

func assertMatrixUsesProfileBranch(t *testing.T, snapshot map[string][]byte, branch string) {
	t.Helper()
	contents := matrixSnapshotText(snapshot)
	wantRef := "origin/" + branch
	if !strings.Contains(contents, wantRef) {
		t.Errorf("generated project never uses profile branch %s", wantRef)
	}
	for _, ref := range matrixOriginRefPattern.FindAllString(contents, -1) {
		if ref != wantRef && ref != "origin/HEAD" {
			t.Errorf("generated Git command uses %s instead of profile branch %s", ref, wantRef)
		}
	}

	for label, pattern := range map[string]string{
		"origin/master":        `origin/master`,
		"behind master":        `(?i)behind[ \t]+master`,
		"checkout master":      `(?im)git[ \t]+(?:checkout|switch)[^\n]*\bmaster\b`,
		"worktree from master": `(?im)git[ \t]+worktree[ \t]+add[^\n]*\bmaster\b`,
		"HEAD fixed to master": `(?im)\bHEAD\b[^\n]*\bmaster\b`,
	} {
		if regexp.MustCompile(pattern).MatchString(contents) {
			t.Errorf("generated project contains hard-coded %s", label)
		}
	}
}

func matrixSnapshotText(snapshot map[string][]byte) string {
	var text strings.Builder
	for _, path := range matrixSnapshotPaths(snapshot) {
		fmt.Fprintf(&text, "\n===== %s =====\n", path)
		text.Write(snapshot[path])
	}
	return text.String()
}

func matrixSnapshotPaths(snapshot map[string][]byte) []string {
	paths := make([]string, 0, len(snapshot))
	for path := range snapshot {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
