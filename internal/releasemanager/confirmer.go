package releasemanager

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type TTYConfirmer struct {
	Input  *os.File
	Output io.Writer
}

func (confirmer TTYConfirmer) Confirm(ctx context.Context, expected string) (bool, error) {
	if confirmer.Input == nil {
		return false, errors.New("release changes require an interactive TTY")
	}
	info, err := confirmer.Input.Stat()
	if err != nil {
		return false, fmt.Errorf("inspect confirmation input: %w", err)
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return false, errors.New("release changes require an interactive TTY; no installation state was changed")
	}
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	output := confirmer.Output
	if output == nil {
		output = io.Discard
	}
	if _, err := fmt.Fprintf(output, "Type the full target version %s to continue: ", expected); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(confirmer.Input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read release confirmation: %w", err)
	}
	return strings.TrimSpace(line) == expected, nil
}
