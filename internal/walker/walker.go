package walker

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DefaultMaxFileSize is the maximum file size to process (1 MB).
const DefaultMaxFileSize int64 = 1 << 20

// FileInfo holds metadata about a single file discovered during traversal.
type FileInfo struct {
	Path        string // Absolute path on disk.
	RelPath     string // Path relative to the root directory.
	Size        int64  // File size in bytes.
	Language    string // Detected programming language.
	ContentHash string // SHA-256 hex digest of the file content.
	IsTest      bool   // Whether the file appears to be a test file.
}

// WalkerConfig controls the behaviour of the Walk function.
type WalkerConfig struct {
	RootDir     string   // Root directory to walk.
	Include     []string // Glob patterns — only matching files are included.
	Exclude     []string // Glob patterns — matching files are excluded.
	MaxFileSize int64    // Files larger than this are skipped (0 = use default).
}

// Walk traverses the directory tree rooted at config.RootDir and returns
// metadata for every source file that passes filtering. It skips binary
// files, respects include/exclude patterns, and honours .gitignore files.
func Walk(config WalkerConfig) ([]FileInfo, error) {
	root, err := filepath.Abs(config.RootDir)
	if err != nil {
		return nil, fmt.Errorf("walker: resolve root: %w", err)
	}

	maxSize := config.MaxFileSize
	if maxSize <= 0 {
		maxSize = DefaultMaxFileSize
	}

	// Load .gitignore patterns from root if present.
	gitignorePatterns := loadGitignore(filepath.Join(root, ".gitignore"))

	var files []FileInfo

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip entries we cannot read instead of aborting.
			return nil
		}

		name := d.Name()

		// Skip default-excluded directories.
		if d.IsDir() {
			if shouldExcludeDir(name) {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process regular files.
		if !d.Type().IsRegular() {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}

		// Check .gitignore patterns.
		if matchesGitignore(relPath, gitignorePatterns) {
			return nil
		}

		// Apply user-defined include/exclude filters.
		if !MatchesInclude(relPath, config.Include) {
			return nil
		}
		if MatchesExclude(relPath, config.Exclude) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Skip files exceeding the size limit.
		if info.Size() > maxSize {
			return nil
		}

		// Skip binary files.
		if isBinary(path) {
			return nil
		}

		hash, err := hashFile(path)
		if err != nil {
			return nil
		}

		files = append(files, FileInfo{
			Path:        path,
			RelPath:     filepath.ToSlash(relPath),
			Size:        info.Size(),
			Language:    DetectLanguage(name),
			ContentHash: hash,
			IsTest:      isTestFile(name, relPath),
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walker: traversal: %w", err)
	}

	return files, nil
}

// isBinary reads the first 512 bytes of a file and checks for NUL bytes,
// which is a simple but effective heuristic for binary content.
func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true // treat unreadable files as binary
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return true
	}

	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}

// hashFile computes the SHA-256 digest of the given file.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// isTestFile returns true if the filename or path looks like a test file.
func isTestFile(name, relPath string) bool {
	lower := strings.ToLower(name)

	// Go test files.
	if strings.HasSuffix(lower, "_test.go") {
		return true
	}
	// Python test files.
	if strings.HasPrefix(lower, "test_") || strings.HasSuffix(lower, "_test.py") {
		return true
	}
	// JavaScript/TypeScript test files.
	for _, suffix := range []string{".test.js", ".test.ts", ".test.tsx", ".spec.js", ".spec.ts", ".spec.tsx"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	// Files inside a test/ or tests/ directory.
	relSlash := filepath.ToSlash(strings.ToLower(relPath))
	if strings.Contains(relSlash, "/test/") || strings.Contains(relSlash, "/tests/") ||
		strings.HasPrefix(relSlash, "test/") || strings.HasPrefix(relSlash, "tests/") {
		return true
	}

	return false
}

// loadGitignore reads a .gitignore file and returns its non-empty,
// non-comment lines as patterns.
func loadGitignore(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

// matchesGitignore checks if a relative path matches any gitignore pattern.
func matchesGitignore(relPath string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	normalized := filepath.ToSlash(relPath)

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)

		// Handle directory-only patterns (trailing /).
		dirOnly := strings.HasSuffix(pattern, "/")
		pattern = strings.TrimSuffix(pattern, "/")

		// If the pattern has no slash, match against any path component.
		if !strings.Contains(pattern, "/") {
			// Check if any component of the path matches.
			parts := strings.Split(normalized, "/")
			for _, part := range parts {
				if matched, _ := filepath.Match(pattern, part); matched {
					if !dirOnly {
						return true
					}
				}
			}
			// Also match the full basename.
			base := filepath.Base(normalized)
			if matched, _ := filepath.Match(pattern, base); matched && !dirOnly {
				return true
			}
		} else {
			// Pattern contains a slash — match against the full relative path.
			if matched, _ := filepath.Match(pattern, normalized); matched {
				return true
			}
		}
	}
	return false
}
