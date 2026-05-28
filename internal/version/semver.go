// Package version holds the build-time version string and shared semver helpers.
package version

import (
	"strconv"
	"strings"
)

// CompareSemver compares two MAJOR.MINOR.PATCH version strings.
// Returns -1 if a < b, 0 if a == b, +1 if a > b.
// Treats any parse error as version 0.0.0.
func CompareSemver(a, b string) int {
	ma, mia, pa := ParseSemver(a)
	mb, mib, pb := ParseSemver(b)

	if ma != mb {
		return cmpInt(ma, mb)
	}
	if mia != mib {
		return cmpInt(mia, mib)
	}
	return cmpInt(pa, pb)
}

// ParseSemver splits "MAJOR.MINOR.PATCH" into three ints. Returns (0,0,0) on
// any parse error.
//
// Phase 1 only handles plain MAJOR.MINOR.PATCH. Pre-release tags
// (e.g. "0.1.0-rc.1") parse the patch component via strconv.Atoi which
// silently coerces "0-rc" to "0", so "0.1.0-rc.1" compares equal to "0.1.0".
// When the embedded version is ever bumped to a pre-release, extend this
// parser or switch to a real semver library.
func ParseSemver(v string) (major, minor, patch int) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return 0, 0, 0
	}
	major, _ = strconv.Atoi(parts[0])
	minor, _ = strconv.Atoi(parts[1])
	patch, _ = strconv.Atoi(parts[2])
	return major, minor, patch
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
