package releasemanager

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCompareVersionsUsesSemanticVersionPrecedence(t *testing.T) {
	for _, test := range []struct {
		left  string
		right string
		want  int
	}{
		{left: "2.0.0", right: "1.99.99", want: 1},
		{left: "999999999999999999999.0.0", right: "2.0.0", want: 1},
		{left: "100000000000000000000.0.0", right: "99999999999999999999.0.0", want: 1},
	} {
		got := compareVersions(test.left, test.right)
		if got != test.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestCompatibilityLockUsesStrictJSON(t *testing.T) {
	for _, content := range []string{
		`{"schema_version":1,"sop_version":"1.0.0","generator_version":"1.0.0","rules_version":"rules","profile_hash":"0000000000000000000000000000000000000000000000000000000000000000","outputs":[],"surprise":true}`,
		`{"schema_version":1,"sop_version":"1.0.0","generator_version":"1.0.0","rules_version":"rules","profile_hash":"0000000000000000000000000000000000000000000000000000000000000000","outputs":[]} {}`,
	} {
		path := filepath.Join(t.TempDir(), "lock.json")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readCompatibilityLock(path); err == nil {
			t.Fatalf("readCompatibilityLock accepted %q", content)
		}
	}
}

func TestPersistedReleaseSourceIsStrictAndIdempotent(t *testing.T) {
	stateHome := t.TempDir()
	sourceRoot := t.TempDir()
	normalized, err := normalizeLocalReleaseSource(sourceRoot)
	if err != nil {
		t.Fatal(err)
	}
	created, err := installReleaseSource(stateHome, normalized)
	if err != nil || !created {
		t.Fatalf("first source install = created:%t err:%v", created, err)
	}
	created, err = installReleaseSource(stateHome, normalized)
	if err != nil || created {
		t.Fatalf("idempotent source install = created:%t err:%v", created, err)
	}
	root, err := (Manager{StateHome: stateHome}).releaseSourceRoot()
	if err != nil || root != normalized {
		t.Fatalf("persisted source = %q, %v; want %q", root, err, normalized)
	}

	for _, content := range []string{
		fmt.Sprintf(`{"format":1,"type":"local","root":%q,"surprise":true}`, normalized),
		fmt.Sprintf(`{"format":1,"type":"local","root":%q} {}`, normalized),
	} {
		if err := os.WriteFile(releaseSourcePath(stateHome), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := (Manager{StateHome: stateHome}).releaseSourceRoot(); err == nil {
			t.Fatalf("release source accepted non-strict JSON: %s", content)
		}
	}
}

func TestStateHomeUsageCommandQuotesPOSIXAndPowerShellPaths(t *testing.T) {
	if got, want := formatStateHomeCommand("darwin", "/tmp/custom state's", "/tmp/custom state's/bin/sopctl"), `SOP_STATE_HOME='/tmp/custom state'"'"'s' '/tmp/custom state'"'"'s/bin/sopctl' release check`; got != want {
		t.Fatalf("POSIX usage = %q, want %q", got, want)
	}
	if got, want := formatStateHomeCommand("windows", `C:\Custom State\owner's`, `C:\Custom State\owner's\bin\sopctl.exe`), `$env:SOP_STATE_HOME = 'C:\Custom State\owner''s'; & 'C:\Custom State\owner''s\bin\sopctl.exe' release check`; got != want {
		t.Fatalf("PowerShell usage = %q, want %q", got, want)
	}
}
