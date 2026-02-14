package walker

import (
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// DefaultExcludes are directory/file patterns excluded by default.
var DefaultExcludes = []string{
	".git",
	"node_modules",
	"vendor",
	"__pycache__",
	".autodoc",
	"dist",
	"build",
	".next",
	"target",
	".venv",
	".idea",
	".vscode",
	".DS_Store",
}

// shouldExcludeDir checks whether a directory name matches any default
// exclusion pattern. This is used during traversal to skip entire subtrees.
func shouldExcludeDir(name string) bool {
	for _, excl := range DefaultExcludes {
		if strings.EqualFold(name, excl) {
			return true
		}
	}
	return false
}

// MatchesInclude returns true if the given relative path matches any of the
// include patterns. If patterns is empty, everything is included.
func MatchesInclude(relPath string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	return matchesAny(relPath, patterns)
}

// MatchesExclude returns true if the given relative path matches any of the
// exclude patterns. If patterns is empty, nothing is excluded.
func MatchesExclude(relPath string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	return matchesAny(relPath, patterns)
}

// matchesAny checks if relPath matches any of the given glob patterns.
// It uses doublestar for ** support and falls back to filepath.Match.
func matchesAny(relPath string, patterns []string) bool {
	// Normalize to forward slashes for consistent matching.
	normalized := filepath.ToSlash(relPath)

	for _, pattern := range patterns {
		pattern = filepath.ToSlash(pattern)

		// Try doublestar matching (supports **).
		if matched, err := doublestar.PathMatch(pattern, normalized); err == nil && matched {
			return true
		}

		// Also try matching against just the filename.
		base := filepath.Base(normalized)
		if matched, err := doublestar.PathMatch(pattern, base); err == nil && matched {
			return true
		}
	}
	return false
}
