package cli

import (
	"strings"

	"golang.org/x/mod/semver"
)

// NormalizeVersion ensures a version string has a leading 'v' prefix if it is a valid SemVer
func NormalizeVersion(v string) string {
	clean := strings.TrimSpace(v)
	if clean == "" {
		return ""
	}
	if !strings.HasPrefix(clean, "v") && !strings.HasPrefix(clean, "V") {
		clean = "v" + clean
	}
	return clean
}

// CompareVersions compares two version strings using official Semantic Versioning rules.
// Returns:
//
//	-1 if v1 < v2
//	 0 if v1 == v2
//	+1 if v1 > v2
//
// If either version is "dev", "dev" is treated as unversioned development build.
func CompareVersions(v1, v2 string) int {
	clean1 := NormalizeVersion(v1)
	clean2 := NormalizeVersion(v2)

	// If either is "dev" or invalid, handle gracefully
	if strings.EqualFold(strings.TrimSpace(v1), "dev") || strings.EqualFold(strings.TrimSpace(v2), "dev") {
		if strings.EqualFold(strings.TrimSpace(v1), strings.TrimSpace(v2)) {
			return 0
		}
		// "dev" is not comparable via standard SemVer
		return 0
	}

	if !semver.IsValid(clean1) || !semver.IsValid(clean2) {
		return 0
	}

	return semver.Compare(clean1, clean2)
}

// IsUpdateAvailable returns true if remoteVersion is strictly newer than currentVersion.
// Development builds ("dev") and invalid versions return false to avoid false positives.
func IsUpdateAvailable(currentVersion, remoteVersion string) bool {
	cleanCurrent := strings.TrimSpace(currentVersion)
	if strings.EqualFold(cleanCurrent, "dev") || cleanCurrent == "" {
		return false
	}

	normCurrent := NormalizeVersion(cleanCurrent)
	normRemote := NormalizeVersion(remoteVersion)

	if !semver.IsValid(normCurrent) || !semver.IsValid(normRemote) {
		return false
	}

	return semver.Compare(normRemote, normCurrent) > 0
}
