package releasebundle

import "testing"

func TestReleaseBundleVersionsRejectUnsupportedSemverVariants(t *testing.T) {
	for _, version := range []string{"1.0.0-rc.1", "1.0.0+first", "1.0.0-rc.1+build.7"} {
		if semverPattern.MatchString(version) {
			t.Fatalf("release bundle version %q accepted a non-X.Y.Z version", version)
		}
	}
}
