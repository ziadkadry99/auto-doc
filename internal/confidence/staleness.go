package confidence

import "time"

// externalSources are sources where staleness is checked on a time-based threshold.
var externalSources = map[Source]bool{
	SourceConfluence:       true,
	SourceReadme:           true,
	SourceCodeowners:       true,
	SourceGitHub:           true,
	SourceUserConversation: true,
}

// stalenessThreshold is the maximum age for external-source metadata before it is
// considered potentially stale.
const stalenessThreshold = 6 * 30 * 24 * time.Hour // ~6 months

// CheckStaleness determines whether the given metadata should be considered stale
// based on code change timestamps and source type.
// It returns true if stale and a human-readable reason.
func CheckStaleness(meta Metadata, codeLastChanged time.Time) (bool, string) {
	// If the code changed after the last verification, the metadata may be outdated.
	if !codeLastChanged.IsZero() && codeLastChanged.After(meta.LastVerified) {
		return true, "code changed after last verification"
	}

	// For external sources, flag as stale if not verified within the threshold.
	if externalSources[meta.Source] && time.Since(meta.LastVerified) > stalenessThreshold {
		return true, "external source not re-verified for over 6 months"
	}

	return false, ""
}
