package platform

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

func StateHome() (string, error) {
	if override := os.Getenv("SOP_STATE_HOME"); override != "" {
		return filepath.Abs(override)
	}
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "sop-better"), nil
		}
		return "", errors.New("LOCALAPPDATA is not set")
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "sop-better"), nil
}

func ExecutableName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}
