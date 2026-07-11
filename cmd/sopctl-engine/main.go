package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Strelizialeomon/sop-better/internal/config"
	"github.com/Strelizialeomon/sop-better/internal/engine"
)

const managerEngineProtocol = 1

var buildVersion = "0.1.0-dev"

type description struct {
	Component string `json:"component"`
	Version   string `json:"version"`
	Protocol  int    `json:"protocol"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 2 && args[0] == "__describe" && args[1] == "--json" {
		return json.NewEncoder(os.Stdout).Encode(description{Component: "engine", Version: buildVersion, Protocol: managerEngineProtocol})
	}
	if len(args) == 0 {
		return errors.New("usage: sopctl <command>; run sopctl --help")
	}
	switch args[0] {
	case "--help", "-h", "help":
		printUsage()
		return nil
	case "version", "--version":
		fmt.Fprintf(os.Stdout, "sopctl %s\n", buildVersion)
		return nil
	}
	assetRoot, err := runtimeAssetRoot()
	if err != nil {
		return err
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], assetRoot)
	case "diff":
		return runDiff(args[1:], assetRoot)
	case "render":
		return runRender(args[1:], assetRoot)
	case "project":
		return runProject(args[1:], assetRoot)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runtimeAssetRoot() (string, error) {
	expected := os.Getenv("SOP_RELEASE_VERSION")
	if expected == "" {
		return "", errors.New("sopctl-engine must be launched by its version manager")
	}
	if buildVersion != "0.1.0-dev" && expected != buildVersion {
		return "", fmt.Errorf("manager/engine version mismatch: manager selected %s but engine is %s", expected, buildVersion)
	}
	assetRoot := os.Getenv("SOP_ASSET_ROOT")
	if assetRoot == "" {
		return "", errors.New("version manager did not provide SOP_ASSET_ROOT")
	}
	return assetRoot, nil
}

func runProject(args []string, assetRoot string) error {
	if len(args) == 0 {
		return errors.New("usage: sopctl project <checkpoints|rollback>")
	}
	if args[0] == "checkpoints" {
		flags := flag.NewFlagSet("project checkpoints", flag.ContinueOnError)
		projectRoot := flags.String("project-root", ".", "project root")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		checkpoints, err := engine.ListProjectCheckpoints(*projectRoot)
		if err != nil {
			return err
		}
		if len(checkpoints) == 0 {
			fmt.Fprintln(os.Stdout, "No checkpoints.")
			return nil
		}
		for _, checkpoint := range checkpoints {
			if checkpoint.Status != "ready" {
				fmt.Fprintf(os.Stdout, "DAMAGED %s ERROR %s\n", checkpoint.ID, checkpoint.Problem)
				continue
			}
			fmt.Fprintf(os.Stdout, "CHECKPOINT %s SOP %s CREATED %s\n", checkpoint.ID, checkpoint.SOPVersion, checkpoint.CreatedAt)
		}
		return nil
	}
	if args[0] != "rollback" {
		return errors.New("usage: sopctl project <checkpoints|rollback>")
	}
	flags := flag.NewFlagSet("project rollback", flag.ContinueOnError)
	projectRoot := flags.String("project-root", ".", "project root")
	checkpoint := flags.String("to", "", "checkpoint id")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *checkpoint == "" {
		return errors.New("project rollback: --to <checkpoint> is required")
	}
	if err := engine.RecoverProject(*projectRoot); err != nil {
		return err
	}
	profile, manifest, err := loadContract(*projectRoot, assetRoot)
	if err != nil {
		return err
	}
	result, err := engine.RollbackProject(*projectRoot, *checkpoint, profile, manifest)
	if err != nil {
		return err
	}
	if err := engine.CheckProject(*projectRoot, result.Profile, result.Lock); err != nil {
		return fmt.Errorf("project rollback verification failed: %w", err)
	}
	fmt.Fprintf(os.Stdout, "Rolled back project to %s.\n", *checkpoint)
	return nil
}

func runCheck(args []string, assetRoot string) error {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	projectRoot := flags.String("project-root", ".", "project root")
	if err := flags.Parse(args); err != nil {
		return err
	}
	profile, manifest, err := loadContract(*projectRoot, assetRoot)
	if err != nil {
		return err
	}
	_, expectedLock, err := engine.Render(profile, manifest, assetRoot, buildVersion)
	if err != nil {
		return err
	}
	if heldProjectID := os.Getenv("SOP_PROJECT_LOCK_HELD"); heldProjectID != "" {
		return engine.CheckProjectUnderHeldLock(*projectRoot, heldProjectID, profile, expectedLock)
	}
	return engine.CheckProject(*projectRoot, profile, expectedLock)
}

func runDiff(args []string, assetRoot string) error {
	flags := flag.NewFlagSet("diff", flag.ContinueOnError)
	projectRoot := flags.String("project-root", ".", "project root")
	profileOverride := flags.String("profile", "", "candidate profile path for read-only preview")
	if err := flags.Parse(args); err != nil {
		return err
	}
	profilePath := *profileOverride
	if profilePath == "" {
		profilePath = filepath.Join(*projectRoot, ".sop", "profile.json")
	}
	profileData, err := os.ReadFile(profilePath)
	if err != nil {
		return fmt.Errorf("read candidate profile: %w", err)
	}
	profile, err := config.ParseProfile(profileData)
	if err != nil {
		return err
	}
	manifest, err := config.LoadManifest(filepath.Join(assetRoot, "manifest.json"))
	if err != nil {
		return err
	}
	if err := manifest.ValidateAssets(assetRoot); err != nil {
		return err
	}
	if err := validateProfileManifest(profile, manifest); err != nil {
		return err
	}
	outputs, lock, err := engine.Render(profile, manifest, assetRoot, buildVersion)
	if err != nil {
		return err
	}
	var report string
	if heldProjectID := os.Getenv("SOP_PROJECT_LOCK_HELD"); heldProjectID != "" {
		report, err = engine.DiffUnderHeldLock(*projectRoot, heldProjectID, outputs, lock, profile, profileData)
	} else {
		report, err = engine.Diff(*projectRoot, outputs, lock, profile, profileData)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(os.Stdout, report)
	return err
}

func runRender(args []string, assetRoot string) error {
	flags := flag.NewFlagSet("render", flag.ContinueOnError)
	projectRoot := flags.String("project-root", ".", "project root")
	profileOverride := flags.String("profile", "", "candidate profile path to commit with rendered outputs")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := engine.RecoverProject(*projectRoot); err != nil {
		return err
	}
	profilePath := *profileOverride
	if profilePath == "" {
		profilePath = filepath.Join(*projectRoot, ".sop", "profile.json")
	}
	profileData, err := os.ReadFile(profilePath)
	if err != nil {
		return fmt.Errorf("read candidate profile: %w", err)
	}
	profile, err := config.LoadProfile(profilePath)
	if err != nil {
		return err
	}
	manifest, err := config.LoadManifest(filepath.Join(assetRoot, "manifest.json"))
	if err != nil {
		return err
	}
	if err := manifest.ValidateAssets(assetRoot); err != nil {
		return err
	}
	if err := validateProfileManifest(profile, manifest); err != nil {
		return err
	}
	outputs, lock, err := engine.Render(profile, manifest, assetRoot, buildVersion)
	if err != nil {
		return err
	}
	return engine.Write(*projectRoot, outputs, lock, profile, profileData)
}

func loadContract(projectRoot, assetRoot string) (config.Profile, config.Manifest, error) {
	profile, err := config.LoadProfile(filepath.Join(projectRoot, ".sop", "profile.json"))
	if err != nil {
		return config.Profile{}, config.Manifest{}, err
	}
	manifest, err := config.LoadManifest(filepath.Join(assetRoot, "manifest.json"))
	if err != nil {
		return config.Profile{}, config.Manifest{}, err
	}
	if err := manifest.ValidateAssets(assetRoot); err != nil {
		return config.Profile{}, config.Manifest{}, err
	}
	if err := validateProfileManifest(profile, manifest); err != nil {
		return config.Profile{}, config.Manifest{}, err
	}
	return profile, manifest, nil
}

func validateProfileManifest(profile config.Profile, manifest config.Manifest) error {
	if profile.SchemaVersion != manifest.ProfileSchemaVersion {
		return fmt.Errorf("profile.schema_version %d is incompatible with manifest profile schema %d", profile.SchemaVersion, manifest.ProfileSchemaVersion)
	}
	if profile.SOPVersion != manifest.SOPVersion {
		return fmt.Errorf("profile.sop_version %s is incompatible with manifest SOP version %s", profile.SOPVersion, manifest.SOPVersion)
	}
	return nil
}

func printUsage() {
	fmt.Fprint(os.Stdout, `sopctl manages reproducible project SOP files.

Commands:
  check
  diff
  render
  project checkpoints
  project rollback
  release check
  release diff
  release upgrade
  release rollback
  version
`)
}
