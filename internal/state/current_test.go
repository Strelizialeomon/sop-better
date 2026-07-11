package state

import "testing"

func TestReleaseVersionsRejectUnsupportedSemverVariants(t *testing.T) {
	for _, version := range []string{"1.0.0-rc.1", "1.0.0+first", "1.0.0-rc.1+build.7"} {
		if ValidVersion(version) {
			t.Fatalf("ValidVersion(%q) = true; phase 1 release versions must use X.Y.Z", version)
		}
		if err := (Current{Format: CurrentFormat, Version: version}).Validate(); err == nil {
			t.Fatalf("Current.Validate accepted unsupported release version %q", version)
		}
	}
}
