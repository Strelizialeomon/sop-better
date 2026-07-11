package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Strelizialeomon/sop-better/internal/platform"
)

const CurrentFormat = 1

var semverPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

type Current struct {
	Format   int    `json:"format"`
	Version  string `json:"version"`
	Previous string `json:"previous"`
}

func ReadCurrent(stateHome string) (Current, error) {
	path := filepath.Join(stateHome, "current.json")
	file, err := os.Open(path)
	if err != nil {
		return Current{}, fmt.Errorf("read current.json: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var current Current
	if err := decoder.Decode(&current); err != nil {
		return Current{}, fmt.Errorf("parse current.json: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Current{}, fmt.Errorf("parse current.json: %w", err)
	}
	if err := current.Validate(); err != nil {
		return Current{}, err
	}
	return current, nil
}

func (current Current) Validate() error {
	if current.Format != CurrentFormat {
		return fmt.Errorf("current.json format must be %d", CurrentFormat)
	}
	if !semverPattern.MatchString(current.Version) {
		return errors.New("current.json version must be strict semver")
	}
	if current.Previous != "" && !semverPattern.MatchString(current.Previous) {
		return errors.New("current.json previous must be empty or strict semver")
	}
	if current.Previous == current.Version {
		return errors.New("current.json previous must differ from version")
	}
	return nil
}

func ValidVersion(version string) bool {
	return semverPattern.MatchString(version)
}

func WriteCurrent(stateHome string, current Current) error {
	if err := current.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return platform.AtomicWrite(filepath.Join(stateHome, "current.json"), data, 0o644)
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("multiple JSON values are not allowed")
}
