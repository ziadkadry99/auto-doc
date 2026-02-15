package indexer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SaveAnalyses persists file analyses to .autodoc/analyses.json inside the given directory.
func SaveAnalyses(dir string, analyses map[string]FileAnalysis) error {
	autodocDir := filepath.Join(dir, ".autodoc")
	if err := os.MkdirAll(autodocDir, 0o755); err != nil {
		return fmt.Errorf("create .autodoc dir: %w", err)
	}

	data, err := json.MarshalIndent(analyses, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal analyses: %w", err)
	}

	return os.WriteFile(filepath.Join(autodocDir, "analyses.json"), data, 0o644)
}

// LoadAnalyses reads file analyses from .autodoc/analyses.json inside the given directory.
// Returns an empty map if the file does not exist.
func LoadAnalyses(dir string) (map[string]FileAnalysis, error) {
	path := filepath.Join(dir, ".autodoc", "analyses.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]FileAnalysis), nil
		}
		return nil, fmt.Errorf("read analyses: %w", err)
	}

	var analyses map[string]FileAnalysis
	if err := json.Unmarshal(data, &analyses); err != nil {
		return nil, fmt.Errorf("unmarshal analyses: %w", err)
	}
	if analyses == nil {
		analyses = make(map[string]FileAnalysis)
	}
	return analyses, nil
}

// ExpandChangedFiles uses a reverse dependency graph to find all files transitively
// affected by the directly changed files. It returns:
//   - expanded: the full set of files that need re-analysis (directly changed + dep-affected)
//   - depAffected: only the files pulled in via dependencies (not directly changed)
func ExpandChangedFiles(changedFiles []string, allAnalyses map[string]FileAnalysis) (expanded, depAffected []string) {
	if len(allAnalyses) == 0 {
		return changedFiles, nil
	}

	// Build a set of directories that changed files belong to.
	changedDirs := make(map[string]bool)
	changedSet := make(map[string]bool)
	for _, f := range changedFiles {
		changedSet[f] = true
		dir := filepath.ToSlash(filepath.Dir(f))
		changedDirs[dir] = true
	}

	// Build reverse dependency map: for each directory, which files depend on it.
	// Key = directory path, Value = list of file paths that import/depend on that directory.
	reverseDeps := make(map[string][]string)
	for filePath, analysis := range allAnalyses {
		for _, dep := range analysis.Dependencies {
			for dir := range changedDirs {
				if depMatchesDir(dep.Name, dir) {
					reverseDeps[dir] = append(reverseDeps[dir], filePath)
				}
			}
		}
	}

	// BFS from changed directories to find transitively affected files.
	visited := make(map[string]bool)
	for _, f := range changedFiles {
		visited[f] = true
	}

	queue := make([]string, 0)
	// Seed the queue with files that depend on changed directories.
	for dir := range changedDirs {
		for _, depFile := range reverseDeps[dir] {
			if !visited[depFile] {
				visited[depFile] = true
				queue = append(queue, depFile)
			}
		}
	}

	// BFS: for each newly affected file, check if other files depend on its directory too.
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		depAffected = append(depAffected, current)

		currentDir := filepath.ToSlash(filepath.Dir(current))
		// Find files that depend on this newly-affected file's directory.
		for filePath, analysis := range allAnalyses {
			if visited[filePath] {
				continue
			}
			for _, dep := range analysis.Dependencies {
				if depMatchesDir(dep.Name, currentDir) {
					visited[filePath] = true
					queue = append(queue, filePath)
					break
				}
			}
		}
	}

	// Build expanded list: directly changed + dep-affected.
	expanded = make([]string, 0, len(changedFiles)+len(depAffected))
	expanded = append(expanded, changedFiles...)
	expanded = append(expanded, depAffected...)
	return expanded, depAffected
}

// depMatchesDir returns true if the dependency name plausibly refers to the given
// directory path. Uses aggressive fuzzy matching similar to depMatchesPkg.
func depMatchesDir(depName, dir string) bool {
	// Normalize to forward slashes for consistent matching.
	depName = filepath.ToSlash(depName)
	dir = filepath.ToSlash(dir)

	if depName == "" || dir == "" || dir == "." {
		return false
	}

	// Exact match.
	if depName == dir {
		return true
	}

	// Dep ends with the dir path (e.g., "github.com/foo/internal/config" ends with "internal/config").
	if strings.HasSuffix(depName, "/"+dir) {
		return true
	}

	// Same base segment (e.g., dep "config" matches dir "internal/config").
	base := filepath.Base(dir)
	if base != "." && depName == base {
		return true
	}

	// Dir path contained as a component in the dep name.
	if strings.Contains(depName, "/"+dir+"/") {
		return true
	}

	return false
}
