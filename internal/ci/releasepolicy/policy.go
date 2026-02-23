package releasepolicy

import "regexp"

var stableSemverTagRef = regexp.MustCompile(`^refs/tags/v[0-9]+\.[0-9]+\.[0-9]+$`)

func IsStableSemverTagRef(ref string) bool {
	return stableSemverTagRef.MatchString(ref)
}
