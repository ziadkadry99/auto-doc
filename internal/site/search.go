package site

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// SearchEntry represents a single searchable page in the documentation.
type SearchEntry struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Content string `json:"content"`
}

// BuildSearchIndex reads all .md files under docsDir and builds a search index.
func BuildSearchIndex(docsDir string) ([]SearchEntry, error) {
	var entries []SearchEntry

	err := filepath.Walk(docsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		relPath, err := filepath.Rel(docsDir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		entry, err := parseMarkdownForSearch(path, relPath)
		if err != nil {
			return err
		}
		entries = append(entries, entry)
		return nil
	})

	return entries, err
}

// parseMarkdownForSearch extracts title, summary, and content from a markdown file.
func parseMarkdownForSearch(filePath, relPath string) (SearchEntry, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return SearchEntry{}, err
	}
	defer f.Close()

	entry := SearchEntry{
		Path: mdPathToHTML(relPath),
	}

	scanner := bufio.NewScanner(f)
	var lines []string
	foundTitle := false
	foundSummary := false

	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)

		if !foundTitle && strings.HasPrefix(line, "# ") {
			entry.Title = strings.TrimPrefix(line, "# ")
			foundTitle = true
			continue
		}

		if foundTitle && !foundSummary && strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "#") {
			summary := strings.TrimSpace(line)
			// Extract clean summary from verbose metadata if needed.
			if strings.Contains(summary, "Summary:") && strings.Contains(summary, "Language:") {
				fields := parseMetadataFields(summary)
				if s := fields["Summary"]; s != "" {
					summary = s
				}
			}
			entry.Summary = summary
			foundSummary = true
		}
	}

	if err := scanner.Err(); err != nil {
		return SearchEntry{}, err
	}

	// Use full content for search, truncated to a reasonable size.
	// Clean up empty sections and metadata noise.
	var cleanLines []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "## Table of Contents" || trimmed == "" {
			continue
		}
		cleanLines = append(cleanLines, l)
	}
	content := strings.Join(cleanLines, " ")
	// Extract just the summary from verbose metadata if present.
	if strings.Contains(content, "Summary:") && strings.Contains(content, "Language:") {
		fields := parseMetadataFields(content)
		if s := fields["Summary"]; s != "" {
			content = s
			if p := fields["Purpose"]; p != "" {
				content += " " + p
			}
		}
	}
	if len(content) > 2000 {
		content = content[:2000]
	}
	entry.Content = content

	if entry.Title == "" {
		entry.Title = relPath
	}

	return entry, nil
}

// WriteSearchIndex writes the search index as JSON to the given path.
func WriteSearchIndex(entries []SearchEntry, outputPath string) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, data, 0o644)
}
