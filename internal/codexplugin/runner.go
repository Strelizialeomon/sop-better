package codexplugin

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runner executes Codex CLI arguments and returns stdout. Implementations must
// not include the leading codex executable in args.
type Runner interface {
	Run(context.Context, ...string) ([]byte, error)
}

// CommandRunner invokes the installed Codex CLI. An empty Binary uses "codex".
// Env entries override inherited variables while leaving the rest of the
// parent environment available to Codex.
type CommandRunner struct {
	Binary string
	Env    []string
}

func (runner CommandRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	binary := runner.Binary
	if binary == "" {
		binary = "codex"
	}
	command := exec.CommandContext(ctx, binary, args...)
	if len(runner.Env) > 0 {
		command.Env = append(os.Environ(), runner.Env...)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return stdout.Bytes(), fmt.Errorf("codex %s: %w: %s", strings.Join(args, " "), err, detail)
		}
		return stdout.Bytes(), fmt.Errorf("codex %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}
