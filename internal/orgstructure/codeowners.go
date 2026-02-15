package orgstructure

import (
	"path/filepath"
	"strings"
)

// CodeownersRule represents a single rule from a CODEOWNERS file.
type CodeownersRule struct {
	Pattern string
	Owner   string
}

// ParseCodeowners parses CODEOWNERS file content into a list of rules.
// Blank lines and comment lines (starting with #) are skipped.
// Each rule line has a pattern followed by one or more owners; only the first owner is kept.
func ParseCodeowners(content string) ([]CodeownersRule, error) {
	var rules []CodeownersRule
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		rules = append(rules, CodeownersRule{
			Pattern: fields[0],
			Owner:   fields[1],
		})
	}
	return rules, nil
}

// MatchFile returns the owner for a given file path based on CODEOWNERS rules.
// The last matching rule wins, consistent with GitHub CODEOWNERS behavior.
// Returns empty string if no rule matches.
func MatchFile(rules []CodeownersRule, filePath string) string {
	owner := ""
	for _, rule := range rules {
		if matchPattern(rule.Pattern, filePath) {
			owner = rule.Owner
		}
	}
	return owner
}

// matchPattern checks if a file path matches a CODEOWNERS pattern.
func matchPattern(pattern, filePath string) bool {
	// Normalize to forward slashes.
	filePath = filepath.ToSlash(filePath)
	pattern = filepath.ToSlash(pattern)

	// Strip leading slash from pattern (CODEOWNERS patterns are repo-relative).
	pattern = strings.TrimPrefix(pattern, "/")

	// Wildcard extension pattern like *.go matches any file with that extension.
	if strings.HasPrefix(pattern, "*.") {
		ext := pattern[1:] // e.g. ".go"
		return strings.HasSuffix(filePath, ext)
	}

	// Directory pattern (ends with /): matches anything under that directory.
	if strings.HasSuffix(pattern, "/") {
		dir := strings.TrimSuffix(pattern, "/")
		return strings.HasPrefix(filePath, dir+"/") || filePath == dir
	}

	// Pattern contains a wildcard *: use filepath.Match on each path segment combination.
	if strings.Contains(pattern, "*") {
		matched, _ := filepath.Match(pattern, filePath)
		if matched {
			return true
		}
		// Try matching just the filename for patterns like "*.txt".
		matched, _ = filepath.Match(pattern, filepath.Base(filePath))
		return matched
	}

	// Exact directory match: if pattern has no extension and no wildcard, treat as directory prefix.
	if !strings.Contains(filepath.Base(pattern), ".") && !strings.Contains(pattern, "/") {
		// Could be a directory name; match as prefix.
		if strings.HasPrefix(filePath, pattern+"/") {
			return true
		}
	}

	// Exact file match.
	return filePath == pattern
}
