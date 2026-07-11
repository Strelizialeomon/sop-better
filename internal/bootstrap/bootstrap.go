package bootstrap

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Strelizialeomon/sop-better/internal/platform"
	"github.com/Strelizialeomon/sop-better/internal/state"
)

type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func Run(ctx context.Context, stateHome string, args []string, streams Streams) (int, error) {
	current, err := state.ReadCurrent(stateHome)
	if err != nil {
		return 1, err
	}
	managerPath := filepath.Join(
		stateHome,
		"versions",
		current.Version,
		"bin",
		platform.ExecutableName("sopctl-manager"),
	)
	info, err := os.Lstat(managerPath)
	if err != nil {
		return 1, fmt.Errorf("inspect active manager: %w", err)
	}
	if !info.Mode().IsRegular() {
		return 1, errorsNewActiveManager()
	}
	command := exec.CommandContext(ctx, managerPath, args...)
	command.Stdin = streams.Stdin
	command.Stdout = streams.Stdout
	command.Stderr = streams.Stderr
	command.Env = append(os.Environ(), "SOP_STATE_HOME="+stateHome)
	if err := command.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return exitError.ExitCode(), nil
		}
		return 1, fmt.Errorf("run active manager: %w", err)
	}
	return 0, nil
}

func errorsNewActiveManager() error {
	return fmt.Errorf("active manager must be a regular file")
}
