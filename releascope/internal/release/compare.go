// Package release implements the pure, side-effect-free business logic
// of Releascope: deciding whether a fetched GitHub release is "new"
// relative to what is stored, and extracting OpenTelemetry Collector
// Contrib component names from release text. Keeping this logic free of
// AWS/GitHub SDK types is what makes it trivially unit-testable and
// reusable from every Lambda plus the local CLI.
package release

import (
	"strings"

	"golang.org/x/mod/semver"
)

// IsNewRelease implements the detection rule from spec section 6:
// latest.tag_name != stored.latestVersion. An empty storedVersion (a
// repository never checked before) always counts as new so the first
// check populates history instead of silently skipping it.
func IsNewRelease(storedVersion, latestVersion string) bool {
	if latestVersion == "" {
		return false
	}
	return storedVersion != latestVersion
}

// Compare orders two release tags. OpenTelemetry Collector tags are
// valid Go-style semver ("v0.100.0"), so semver.Compare is used when
// both sides parse as valid semver; otherwise it falls back to a plain
// string comparison rather than erroring, since tag naming is not
// guaranteed for arbitrary future repositories (spec section 32).
func Compare(a, b string) int {
	na, nb := normalize(a), normalize(b)
	if semver.IsValid(na) && semver.IsValid(nb) {
		return semver.Compare(na, nb)
	}
	return strings.Compare(a, b)
}

func normalize(v string) string {
	if v == "" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}
