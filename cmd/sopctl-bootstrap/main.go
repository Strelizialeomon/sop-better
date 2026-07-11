package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Strelizialeomon/sop-better/internal/bootstrap"
	"github.com/Strelizialeomon/sop-better/internal/platform"
)

func main() {
	stateHome, err := platform.StateHome()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	exitCode, err := bootstrap.Run(context.Background(), stateHome, os.Args[1:], bootstrap.Streams{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(exitCode)
}
