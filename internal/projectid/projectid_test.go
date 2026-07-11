package projectid

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestIdentifierTreatsSymlinkAliasAsTheSameProject(t *testing.T) {
	parent := t.TempDir()
	project := filepath.Join(parent, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "project-alias")
	if err := os.Symlink(project, alias); err != nil {
		t.Skipf("filesystem does not allow a directory symlink: %v", err)
	}

	projectID, err := Identifier(project)
	if err != nil {
		t.Fatal(err)
	}
	aliasID, err := Identifier(alias)
	if err != nil {
		t.Fatal(err)
	}
	if aliasID != projectID {
		t.Fatalf("symlink alias ID = %s, want %s", aliasID, projectID)
	}
}

func TestIdentifierTreatsNativeCaseAliasAsTheSameProject(t *testing.T) {
	parent := t.TempDir()
	project := filepath.Join(parent, "CaseProject")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "caseproject")
	projectInfo, err := os.Stat(project)
	if err != nil {
		t.Fatal(err)
	}
	aliasInfo, err := os.Stat(alias)
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("filesystem is case-sensitive")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(projectInfo, aliasInfo) {
		t.Skip("case variants are different native directories")
	}

	projectID, err := Identifier(project)
	if err != nil {
		t.Fatal(err)
	}
	aliasID, err := Identifier(alias)
	if err != nil {
		t.Fatal(err)
	}
	if aliasID != projectID {
		t.Fatalf("native case alias ID = %s, want %s", aliasID, projectID)
	}
}

func TestIdentifierDistinguishesCaseVariantsOnCaseSensitiveFilesystem(t *testing.T) {
	parent := t.TempDir()
	upper := filepath.Join(parent, "CaseProject")
	lower := filepath.Join(parent, "caseproject")
	if err := os.Mkdir(upper, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(lower, 0o755); errors.Is(err, os.ErrExist) {
		t.Skip("filesystem is case-insensitive")
	} else if err != nil {
		t.Fatal(err)
	}

	upperID, err := Identifier(upper)
	if err != nil {
		t.Fatal(err)
	}
	lowerID, err := Identifier(lower)
	if err != nil {
		t.Fatal(err)
	}
	if upperID == lowerID {
		t.Fatalf("case-distinct directories share ID %s", upperID)
	}
}
