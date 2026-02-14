package indexer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// IndexState tracks which files have been indexed and their content hashes.
type IndexState struct {
	LastCommitSHA string            `json:"last_commit_sha"`
	FileHashes    map[string]string `json:"file_hashes"`
	LastUpdated   time.Time         `json:"last_updated"`
}

// LoadState reads index state from .autodoc/state.json inside the given directory.
func LoadState(dir string) (*IndexState, error) {
	path := filepath.Join(dir, ".autodoc", "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &IndexState{
				FileHashes: make(map[string]string),
			}, nil
		}
		return nil, err
	}

	var state IndexState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.FileHashes == nil {
		state.FileHashes = make(map[string]string)
	}
	return &state, nil
}

// SaveState writes the index state to .autodoc/state.json inside the given directory.
func (s *IndexState) SaveState(dir string) error {
	autodocDir := filepath.Join(dir, ".autodoc")
	if err := os.MkdirAll(autodocDir, 0o755); err != nil {
		return err
	}

	s.LastUpdated = time.Now()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(autodocDir, "state.json"), data, 0o644)
}

// IsFileChanged returns true if the file's content hash differs from the stored hash.
func (s *IndexState) IsFileChanged(filePath, contentHash string) bool {
	stored, ok := s.FileHashes[filePath]
	if !ok {
		return true
	}
	return stored != contentHash
}

// GetGitCommitSHA returns the current HEAD commit SHA, or empty string if not in a git repo.
func GetGitCommitSHA(dir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// GetGitChangedFiles returns files that have been modified, added, or deleted
// between the given commit SHA and HEAD. If lastSHA is empty, all lists are
// returned empty (callers should use `generate` for the initial run).
func GetGitChangedFiles(dir, lastSHA string) (modified, added, deleted []string, err error) {
	if lastSHA == "" {
		return nil, nil, nil, nil
	}

	type diffQuery struct {
		filter string
		dest   *[]string
	}
	queries := []diffQuery{
		{"M", &modified},
		{"A", &added},
		{"D", &deleted},
	}

	for _, q := range queries {
		cmd := exec.Command("git", "diff", "--name-only", "--diff-filter="+q.filter, lastSHA, "HEAD")
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("git diff --diff-filter=%s: %w", q.filter, err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				*q.dest = append(*q.dest, line)
			}
		}
	}

	return modified, added, deleted, nil
}
