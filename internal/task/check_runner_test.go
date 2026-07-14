package task

import (
	"context"
	"runtime"
	"testing"
)

func TestShellCheckExecutorRunsOnlySelectedGroupsAndRecordsEvidence(t *testing.T) {
	command := "test \"$SOP_CHECK\" = affected"
	if runtime.GOOS == "windows" {
		command = `if ($env:SOP_CHECK -ne "affected") { exit 1 }`
	}
	executor := ShellCheckExecutor{Environment: []string{"SOP_CHECK=affected"}}
	runs, err := executor.Run(context.Background(), t.TempDir(), "head-2", map[string][]string{
		"build": {"this-command-must-not-run"},
		"test":  {command},
	}, []string{"test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Group != "test" || runs[0].HeadSHA != "head-2" || !runs[0].Passed || runs[0].DurationMillis < 0 {
		t.Fatalf("runs = %+v", runs)
	}
}
