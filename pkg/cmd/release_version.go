package cmd

import (
	"regexp"
	"strings"

	"github.com/MohamedElashri/txm/pkg/core"
)

var stableReleaseVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func stableVersionParts(version string) []string {
	return stableReleaseVersion.FindStringSubmatch(strings.TrimPrefix(version, "v"))
}

func compareStableVersions(latest, current string) (int, error) {
	a, b := stableVersionParts(latest), stableVersionParts(current)
	if a == nil || b == nil {
		return 0, core.Failure("protocol_error", "update metadata must identify a stable semantic version")
	}
	// Decimal component lengths compare numerically without integer overflow.
	for i := 1; i <= 3; i++ {
		if len(a[i]) > len(b[i]) {
			return 1, nil
		}
		if len(a[i]) < len(b[i]) {
			return -1, nil
		}
		if n := strings.Compare(a[i], b[i]); n != 0 {
			return n, nil
		}
	}
	return 0, nil
}
