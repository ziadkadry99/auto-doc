package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildTree(t *testing.T) {
	paths := []string{
		"index.md",
		"cmd/root.go.md",
		"cmd/generate.go.md",
		"internal/config/config.go.md",
		"internal/config/types.go.md",
		"internal/llm/provider.go.md",
	}

	tree := BuildTree(paths, nil)

	if tree.Name != "docs" {
		t.Errorf("root name = %q, want %q", tree.Name, "docs")
	}
	if !tree.IsDir {
		t.Error("root should be a directory")
	}

	// Root should have children: cmd/, internal/, and index.md
	// Directories first: cmd, internal, then files: index.md
	if len(tree.Children) != 3 {
		t.Fatalf("root children = %d, want 3", len(tree.Children))
	}

	// First two should be directories (sorted: cmd before internal).
	if tree.Children[0].Name != "cmd" || !tree.Children[0].IsDir {
		t.Errorf("first child = %q (dir=%v), want cmd dir", tree.Children[0].Name, tree.Children[0].IsDir)
	}
	if tree.Children[1].Name != "internal" || !tree.Children[1].IsDir {
		t.Errorf("second child = %q (dir=%v), want internal dir", tree.Children[1].Name, tree.Children[1].IsDir)
	}
	if tree.Children[2].Name != "index.md" || tree.Children[2].IsDir {
		t.Errorf("third child = %q (dir=%v), want index.md file", tree.Children[2].Name, tree.Children[2].IsDir)
	}

	// Check cmd/ has 2 files.
	cmdNode := tree.Children[0]
	if len(cmdNode.Children) != 2 {
		t.Fatalf("cmd children = %d, want 2", len(cmdNode.Children))
	}
	// Files should be sorted: generate.go.md before root.go.md
	if cmdNode.Children[0].Name != "generate.go.md" {
		t.Errorf("cmd first child = %q, want generate.go.md", cmdNode.Children[0].Name)
	}
	if cmdNode.Children[1].Name != "root.go.md" {
		t.Errorf("cmd second child = %q, want root.go.md", cmdNode.Children[1].Name)
	}

	// Check internal/ has 2 subdirs: config, llm
	internalNode := tree.Children[1]
	if len(internalNode.Children) != 2 {
		t.Fatalf("internal children = %d, want 2", len(internalNode.Children))
	}
	if internalNode.Children[0].Name != "config" || !internalNode.Children[0].IsDir {
		t.Errorf("internal first child = %q, want config dir", internalNode.Children[0].Name)
	}
	if internalNode.Children[1].Name != "llm" || !internalNode.Children[1].IsDir {
		t.Errorf("internal second child = %q, want llm dir", internalNode.Children[1].Name)
	}
}

func TestBuildTreeEmpty(t *testing.T) {
	tree := BuildTree(nil, nil)
	if len(tree.Children) != 0 {
		t.Errorf("empty tree children = %d, want 0", len(tree.Children))
	}
}

func TestBuildTreeSingleFile(t *testing.T) {
	tree := BuildTree([]string{"readme.md"}, nil)
	if len(tree.Children) != 1 {
		t.Fatalf("tree children = %d, want 1", len(tree.Children))
	}
	if tree.Children[0].Name != "readme.md" {
		t.Errorf("child name = %q, want readme.md", tree.Children[0].Name)
	}
	if tree.Children[0].IsDir {
		t.Error("child should be a file, not a directory")
	}
}

func TestTreeToHTML(t *testing.T) {
	paths := []string{
		"cmd/root.go.md",
		"index.md",
	}

	tree := BuildTree(paths, nil)
	html := tree.ToHTML("index.md", "")

	if !strings.Contains(html, `class="dir`) {
		t.Error("tree HTML should contain dir class")
	}
	if !strings.Contains(html, `class="file"`) {
		t.Error("tree HTML should contain file class")
	}
	if !strings.Contains(html, `cmd/root.go.html`) {
		t.Error("tree HTML should contain .html link for root.go")
	}
	if !strings.Contains(html, `class="active"`) {
		t.Error("tree HTML should contain active class for index.md")
	}
}

func TestMdPathToHTML(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"index.md", "index.html"},
		{"cmd/root.go.md", "cmd/root.go.html"},
		{"internal/config/config.go.md", "internal/config/config.go.html"},
		{"noext", "noext"},
	}
	for _, tt := range tests {
		got := mdPathToHTML(tt.input)
		if got != tt.want {
			t.Errorf("mdPathToHTML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCleanDisplayName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"config.go.md", "config.go"},
		{"index.md", "index"},
		{"readme", "readme"},
	}
	for _, tt := range tests {
		got := cleanDisplayName(tt.input)
		if got != tt.want {
			t.Errorf("cleanDisplayName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		content string
		relPath string
		want    string
	}{
		{"# My Title\n\nSome text", "file.md", "My Title"},
		{"\n\n# Second Line Title\n", "file.md", "Second Line Title"},
		{"No heading here", "fallback.md", "fallback"},
		{"## Not H1\n# H1 Title", "f.md", "H1 Title"},
	}
	for _, tt := range tests {
		got := extractTitle(tt.content, tt.relPath)
		if got != tt.want {
			t.Errorf("extractTitle(%q, %q) = %q, want %q", tt.content, tt.relPath, got, tt.want)
		}
	}
}

func TestPostProcessMermaid(t *testing.T) {
	input := `<p>Hello</p><pre><code class="language-mermaid">graph TD
A --> B</code></pre><p>End</p>`

	got := postProcessMermaid(input)

	// postProcessMermaid is now a no-op; mermaid code blocks are converted
	// to mermaid divs by JavaScript at runtime for proper HTML entity handling.
	if !strings.Contains(got, `language-mermaid`) {
		t.Error("should preserve language-mermaid code block for JS conversion")
	}
	if !strings.Contains(got, "graph TD") {
		t.Error("should preserve mermaid content")
	}
}

func TestRewriteMDLinks(t *testing.T) {
	input := `<a href="config.go.md">link</a> and <a href="other.md#section">section</a>`
	got := rewriteMDLinks(input)

	if strings.Contains(got, `.md"`) {
		t.Error("should have rewritten .md to .html")
	}
	if !strings.Contains(got, `config.go.html"`) {
		t.Error("should contain config.go.html")
	}
	if !strings.Contains(got, `other.html#section"`) {
		t.Error("should contain other.html#section")
	}
}

func TestSearchIndex(t *testing.T) {
	// Create temp directory with test markdown files.
	tmpDir := t.TempDir()

	writeTestFile(t, filepath.Join(tmpDir, "index.md"), "# Project\n\nThis is the summary.\n\nMore content here.")
	writeTestFile(t, filepath.Join(tmpDir, "cmd", "root.go.md"), "# cmd/root.go\n\nRoot command implementation.\n\n## Details\n\nSome details.")

	entries, err := BuildSearchIndex(tmpDir)
	if err != nil {
		t.Fatalf("BuildSearchIndex error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}

	// Find the index entry.
	var indexEntry SearchEntry
	for _, e := range entries {
		if e.Path == "index.html" {
			indexEntry = e
			break
		}
	}

	if indexEntry.Title != "Project" {
		t.Errorf("index title = %q, want %q", indexEntry.Title, "Project")
	}
	if indexEntry.Summary != "This is the summary." {
		t.Errorf("index summary = %q, want %q", indexEntry.Summary, "This is the summary.")
	}
	if indexEntry.Content == "" {
		t.Error("index content should not be empty")
	}

	// Verify JSON serialization.
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}
	if !strings.Contains(string(data), "Project") {
		t.Error("JSON should contain title")
	}
}

func TestFullSiteGeneration(t *testing.T) {
	// Create temp docs directory with test markdown.
	docsDir := t.TempDir()
	outputDir := t.TempDir()

	writeTestFile(t, filepath.Join(docsDir, "index.md"), `# Test Project

Welcome to the documentation.

## Getting Started

Check the files below.
`)

	writeTestFile(t, filepath.Join(docsDir, "cmd", "root.go.md"), `# cmd/root.go

## Summary

This is the root command.

## Functions

### Execute

`+"```go\nfunc Execute() error\n```"+`

Runs the application.
`)

	writeTestFile(t, filepath.Join(docsDir, "architecture.md"), `# Architecture Overview

## Components

The system has several components.

`+"```mermaid\ngraph TD\nA[CLI] --> B[Indexer]\nB --> C[VectorDB]\n```"+`
`)

	gen := NewSiteGenerator(docsDir, outputDir, "test-project")
	pageCount, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if pageCount != 3 {
		t.Errorf("pageCount = %d, want 3", pageCount)
	}

	// Verify output files exist.
	expectedFiles := []string{
		"index.html",
		"style.css",
		"script.js",
		"search-index.json",
		"cmd/root.go.html",
		"architecture.html",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(outputDir, filepath.FromSlash(f))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s does not exist", f)
		}
	}

	// Verify index.html content.
	indexContent, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatalf("reading index.html: %v", err)
	}
	indexStr := string(indexContent)

	if !strings.Contains(indexStr, "test-project") {
		t.Error("index.html should contain project name")
	}
	if !strings.Contains(indexStr, "Test Project") {
		t.Error("index.html should contain page title")
	}
	if !strings.Contains(indexStr, `<nav class="sidebar"`) {
		t.Error("index.html should contain sidebar")
	}
	if !strings.Contains(indexStr, "style.css") {
		t.Error("index.html should reference style.css")
	}
	if !strings.Contains(indexStr, "mermaid") {
		t.Error("index.html should include mermaid script")
	}

	// Verify architecture.html has mermaid code block (JS converts to div at runtime).
	archContent, err := os.ReadFile(filepath.Join(outputDir, "architecture.html"))
	if err != nil {
		t.Fatalf("reading architecture.html: %v", err)
	}
	archStr := string(archContent)

	if !strings.Contains(archStr, `language-mermaid`) {
		t.Error("architecture.html should contain mermaid code block")
	}

	// Verify cmd/root.go.html has syntax-highlighted code.
	cmdContent, err := os.ReadFile(filepath.Join(outputDir, "cmd", "root.go.html"))
	if err != nil {
		t.Fatalf("reading cmd/root.go.html: %v", err)
	}
	cmdStr := string(cmdContent)

	if !strings.Contains(cmdStr, "root.go") {
		t.Error("cmd/root.go.html should contain file name")
	}
	if !strings.Contains(cmdStr, `<article class="page-content">`) {
		t.Error("cmd/root.go.html should contain page-content article")
	}

	// Verify nested page has correct base path for assets.
	if !strings.Contains(cmdStr, `../style.css`) {
		t.Error("nested page should reference ../style.css")
	}

	// Verify search index.
	searchData, err := os.ReadFile(filepath.Join(outputDir, "search-index.json"))
	if err != nil {
		t.Fatalf("reading search-index.json: %v", err)
	}
	var searchEntries []SearchEntry
	if err := json.Unmarshal(searchData, &searchEntries); err != nil {
		t.Fatalf("parsing search-index.json: %v", err)
	}
	if len(searchEntries) != 3 {
		t.Errorf("search entries = %d, want 3", len(searchEntries))
	}
}

func TestGenerateNoFiles(t *testing.T) {
	docsDir := t.TempDir()
	outputDir := t.TempDir()

	gen := NewSiteGenerator(docsDir, outputDir, "test")
	_, err := gen.Generate()
	if err == nil {
		t.Error("Generate should fail with no markdown files")
	}
	if !strings.Contains(err.Error(), "no markdown files") {
		t.Errorf("error = %q, want it to mention no markdown files", err.Error())
	}
}

func TestMDLinksInGeneratedHTML(t *testing.T) {
	docsDir := t.TempDir()
	outputDir := t.TempDir()

	writeTestFile(t, filepath.Join(docsDir, "index.md"), "# Index\n\n[Go to config](internal/config/config.go.md)")
	writeTestFile(t, filepath.Join(docsDir, "internal", "config", "config.go.md"), "# Config\n\nConfig file.")

	gen := NewSiteGenerator(docsDir, outputDir, "test")
	if _, err := gen.Generate(); err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	indexHTML, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatalf("reading index.html: %v", err)
	}

	if strings.Contains(string(indexHTML), ".md\"") || strings.Contains(string(indexHTML), ".md#") {
		t.Error("generated HTML should not contain .md links (should be .html)")
	}
	if !strings.Contains(string(indexHTML), "config.go.html") {
		t.Error("generated HTML should contain .html links")
	}
}

// writeTestFile is a helper that creates a file with intermediate directories.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
