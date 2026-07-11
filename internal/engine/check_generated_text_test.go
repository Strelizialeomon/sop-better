package engine

import (
	"strings"
	"testing"

	"github.com/Strelizialeomon/sop-better/internal/config"
)

func TestCheckGeneratedTextRejectsDeterministicCorruptionMatrix(t *testing.T) {
	profile := config.Profile{
		Project: config.Project{DefaultBranch: "main"},
		Ends:    []config.End{{Name: "backend", Path: "backend"}},
	}
	tests := []struct {
		name   string
		output LockOutput
		body   string
		want   string
	}{
		{name: "leftover placeholder", body: "# demo\n{{leftover}}\n", want: "unresolved template placeholder"},
		{name: "manual cleanup note", body: "# demo\n无则删本行\n", want: "template authoring note"},
		{name: "wrong baseline branch", body: "git diff origin/master...HEAD\n", want: "uses origin/master instead of origin/main"},
		{name: "missing standard snapshot", body: "See [STANDARD](STANDARD.md).\n", want: "must not reference STANDARD.md"},
		{name: "deprecated Claude runtime reference", body: "旧运行时说明写到 `CLAUDE.md` 和 `.claude/`。\n", want: "deprecated Claude runtime reference"},
		{name: "broken relative link", body: "See [missing](docs/missing.md).\n", want: "link docs/missing.md does not exist"},
		{name: "author mac path", body: "/Users/example/code/sop-better\n", want: "machine-specific absolute path"},
		{name: "author windows path", body: `C:\Users\example\code\sop-better`, want: "machine-specific absolute path"},
		{name: "generic windows drive path", body: `D:\private\repo\rules.md`, want: "machine-specific absolute path"},
		{name: "old direct push rule", body: "直接推主分支，无需 PR。\n", want: "obsolete direct-push rule"},
		{
			name:   "serial end leaks parallel scope",
			output: LockOutput{ID: "end-agents-backend", Target: "backend/AGENTS.md"},
			body:   "scope: backend\n",
			want:   "serial end guidance contains parallel-only scope",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := test.output
			if output.ID == "" {
				output = LockOutput{ID: "root-agents", Target: "AGENTS.md"}
			}
			err := checkGeneratedText(t.TempDir(), output, test.body, profile, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("checkGeneratedText() error = %v, want containing %q", err, test.want)
			}
		})
	}
}
