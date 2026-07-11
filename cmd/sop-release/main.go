package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/Strelizialeomon/sop-better/internal/releasebundle"
	"github.com/Strelizialeomon/sop-better/internal/releasegate"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: sop-release <gate|verify>")
	}
	switch args[0] {
	case "gate":
		return runGate(args[1:])
	case "build":
		return errors.New("sop-release build cannot bypass release identity and quality gates; use sop-release gate")
	case "assemble-unverified":
		return runBuild(args[1:])
	case "verify":
		return runVerify(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runGate(args []string) error {
	flags := flag.NewFlagSet("gate", flag.ContinueOnError)
	source := flags.String("source", ".", "clean tagged source repository root")
	pluginRoot := flags.String("plugin-root", "plugin", "plugin marketplace scaffold root")
	output := flags.String("output", "", "release bundle output directory outside the source checkout")
	version := flags.String("version", "", "strict semantic version")
	tag := flags.String("tag", "", "Git tag pointing to the release commit")
	commit := flags.String("commit", "", "source HEAD full Git commit SHA")
	releaseNotes := flags.String("release-notes", "", "human-readable release notes")
	upgradeImpact := flags.String("upgrade-impact", "", "human-readable upgrade impact summary")
	targetOS := flags.String("target-os", "", "target operating system")
	targetArch := flags.String("target-arch", "", "target architecture")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return releasegate.Run(releasegate.Options{
		SourceRoot: *source, PluginRoot: *pluginRoot, OutputRoot: *output,
		Version: *version, GitTag: *tag, GitCommit: *commit,
		ReleaseNotes: *releaseNotes, UpgradeImpact: *upgradeImpact,
		TargetOS: *targetOS, TargetArch: *targetArch,
	})
}

func runBuild(args []string) error {
	flags := flag.NewFlagSet("assemble-unverified", flag.ContinueOnError)
	source := flags.String("source", ".", "source repository root")
	pluginRoot := flags.String("plugin-root", "plugin", "plugin marketplace scaffold root")
	output := flags.String("output", "", "release bundle output directory")
	version := flags.String("version", "", "strict semantic version")
	tag := flags.String("tag", "", "Git tag")
	commit := flags.String("commit", "", "full Git commit SHA")
	releaseNotes := flags.String("release-notes", "", "human-readable release notes")
	upgradeImpact := flags.String("upgrade-impact", "", "human-readable upgrade impact summary")
	bootstrapBinary := flags.String("bootstrap-binary", "", "fixed bootstrap executable")
	installerBinary := flags.String("installer-binary", "", "initial installer executable distributed inside the bundle")
	managerBinary := flags.String("manager-binary", "", "version manager executable")
	engineBinary := flags.String("engine-binary", "", "version engine executable")
	binaryVersion := flags.String("binary-version", "", "manager/engine embedded version")
	targetOS := flags.String("target-os", "", "target operating system")
	targetArch := flags.String("target-arch", "", "target architecture")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return releasebundle.Build(releasebundle.Options{
		SourceRoot:      *source,
		PluginRoot:      *pluginRoot,
		OutputRoot:      *output,
		Version:         *version,
		GitTag:          *tag,
		GitCommit:       *commit,
		ReleaseNotes:    *releaseNotes,
		UpgradeImpact:   *upgradeImpact,
		BootstrapBinary: *bootstrapBinary,
		InstallerBinary: *installerBinary,
		ManagerBinary:   *managerBinary,
		EngineBinary:    *engineBinary,
		BinaryVersion:   *binaryVersion,
		TargetOS:        *targetOS,
		TargetArch:      *targetArch,
	})
}

func runVerify(args []string) error {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	bundle := flags.String("bundle", "", "release bundle directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *bundle == "" {
		return errors.New("--bundle is required")
	}
	return releasebundle.Verify(*bundle)
}
