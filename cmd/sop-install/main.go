package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Strelizialeomon/sop-better/internal/codexplugin"
	"github.com/Strelizialeomon/sop-better/internal/platform"
	"github.com/Strelizialeomon/sop-better/internal/releasebundle"
	"github.com/Strelizialeomon/sop-better/internal/releasemanager"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 2 && args[0] == "__describe" && args[1] == "--json" {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"component": "installer", "version": releasemanager.BuildVersion, "protocol": 1,
		})
	}
	flags := flag.NewFlagSet("sop-install", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	stateHomeFlag := flags.String("state-home", "", "installation state directory")
	releaseSource := flags.String("release-source", "", "local directory containing version-named release bundles")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: sop-install [--state-home PATH] [--release-source PATH]")
	}
	bundleRoot, err := ownBundleRoot()
	if err != nil {
		return err
	}
	manifest, err := releasebundle.Inspect(bundleRoot)
	if err != nil {
		return fmt.Errorf("verify containing release bundle: %w", err)
	}
	if releasemanager.BuildVersion != manifest.Version {
		return fmt.Errorf("installer version %s does not match containing release %s", releasemanager.BuildVersion, manifest.Version)
	}
	stateHome := *stateHomeFlag
	if stateHome == "" {
		stateHome, err = platform.StateHome()
		if err != nil {
			return err
		}
	} else {
		stateHome, err = filepath.Abs(stateHome)
		if err != nil {
			return fmt.Errorf("resolve state home: %w", err)
		}
	}
	installer := releasemanager.InitialInstaller{
		StateHome:     stateHome,
		BundleRoot:    bundleRoot,
		ReleaseSource: *releaseSource,
		Plugin:        codexplugin.Controller{StateHome: stateHome},
		Confirmer:     releasemanager.TTYConfirmer{Input: os.Stdin, Output: os.Stderr},
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
	}
	return installer.Install(context.Background())
}

func ownBundleRoot() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate sop-install executable: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve sop-install executable: %w", err)
	}
	return filepath.Dir(filepath.Dir(executable)), nil
}
