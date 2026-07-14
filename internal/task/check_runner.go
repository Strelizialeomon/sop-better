package task

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

type CheckRun struct {
	Group          string   `json:"group"`
	Commands       []string `json:"commands"`
	HeadSHA        string   `json:"head_sha"`
	Passed         bool     `json:"passed"`
	DurationMillis int64    `json:"duration_millis"`
}

type CheckSelection struct {
	Groups []string `json:"groups"`
	Full   bool     `json:"full"`
	Reason string   `json:"reason"`
}

type CheckExecutor interface {
	Run(context.Context, string, string, map[string][]string, []string) ([]CheckRun, error)
}

type ShellCheckExecutor struct {
	Environment []string
}

func (executor ShellCheckExecutor) Run(
	ctx context.Context,
	workspace string,
	headSHA string,
	checks map[string][]string,
	groups []string,
) ([]CheckRun, error) {
	if strings.TrimSpace(workspace) == "" || strings.TrimSpace(headSHA) == "" {
		return nil, errors.New("check execution requires workspace and HEAD SHA")
	}
	selected := append([]string(nil), groups...)
	sort.Strings(selected)
	for index := 1; index < len(selected); index++ {
		if selected[index] == selected[index-1] {
			return nil, fmt.Errorf("check group %q is duplicated", selected[index])
		}
	}
	runs := make([]CheckRun, 0, len(selected))
	for _, group := range selected {
		commands, exists := checks[group]
		if !exists || len(commands) == 0 {
			return runs, fmt.Errorf("selected check group %q is not configured", group)
		}
		started := time.Now()
		run := CheckRun{Group: group, Commands: append([]string(nil), commands...), HeadSHA: headSHA}
		for _, check := range commands {
			var command *exec.Cmd
			if runtime.GOOS == "windows" {
				command = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", check)
			} else {
				command = exec.CommandContext(ctx, "/bin/sh", "-c", check)
			}
			command.Dir = workspace
			command.Env = append(os.Environ(), executor.Environment...)
			output, err := command.CombinedOutput()
			if err != nil {
				run.DurationMillis = time.Since(started).Milliseconds()
				runs = append(runs, run)
				message := strings.TrimSpace(string(output))
				if len(message) > 4000 {
					message = message[len(message)-4000:]
				}
				return runs, fmt.Errorf("runtime check %s failed (%s): %w\n%s", group, check, err, message)
			}
		}
		run.Passed = true
		run.DurationMillis = time.Since(started).Milliseconds()
		runs = append(runs, run)
	}
	return runs, nil
}

func CheckRunsPassed(runs []CheckRun) map[string]bool {
	passed := make(map[string]bool, len(runs))
	for _, run := range runs {
		passed[run.Group] = run.Passed
	}
	return passed
}

func fullCheckSelection(checks map[string][]string) CheckSelection {
	groups := make([]string, 0, len(checks))
	for group := range checks {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return CheckSelection{Groups: groups, Full: true, Reason: "lightweight runtime always runs all checks"}
}
