package main

import (
	"testing"

	"github.com/Strelizialeomon/sop-better/internal/config"
)

func TestValidateProfileManifestAcceptsSupportedProfileSchema(t *testing.T) {
	profile := config.Profile{SchemaVersion: 2, SOPVersion: "0.1.0"}
	manifest := config.Manifest{
		SOPVersion:            "0.1.0",
		ProfileSchemaVersion:  1,
		ProfileSchemaVersions: []int{1, 2},
	}
	if err := validateProfileManifest(profile, manifest); err != nil {
		t.Fatalf("validateProfileManifest() error = %v", err)
	}
}
