package main

import (
	"fmt"
	"os"

	"github.com/Strelizialeomon/sop-better/internal/codexplugin"
	"github.com/Strelizialeomon/sop-better/internal/platform"
	"github.com/Strelizialeomon/sop-better/internal/releasemanager"
)

func main() {
	stateHome, err := platform.StateHome()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	manager := releasemanager.Manager{
		StateHome:     stateHome,
		ReleaseSource: os.Getenv("SOP_RELEASE_SOURCE"),
		Stdin:         os.Stdin,
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
		Plugin:        codexplugin.Controller{StateHome: stateHome},
		Confirmer:     releasemanager.TTYConfirmer{Input: os.Stdin, Output: os.Stderr},
	}
	if err := manager.Run(os.Args[1:]); err != nil {
		if exitCode, ok := releasemanager.ExitCode(err); ok {
			os.Exit(exitCode)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
