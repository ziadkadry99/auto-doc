package site

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// RepoInfo holds information about a registered repository for central site generation.
type RepoInfo struct {
	Name        string
	DisplayName string
	Summary     string
	Status      string
	FileCount   int
	SourceType  string
	DocsDir     string // path to the repo's .autodoc/docs/ directory
}

// LinkInfo represents a cross-service dependency for site generation.
type LinkInfo struct {
	FromRepo  string
	ToRepo    string
	LinkType  string
	Reason    string
	Endpoints []string
}

// FlowInfo represents a cross-service flow for site generation.
type FlowInfo struct {
	Name        string
	Description string
	Narrative   string
	Diagram     string
	Services    []string
}

// CentralSiteGenerator creates a combined static site from multiple repositories.
type CentralSiteGenerator struct {
	OutputDir   string
	ProjectName string
	Repos       []RepoInfo
	Links       []LinkInfo
	Flows       []FlowInfo
	LogoPath    string
}

// Generate builds the combined multi-repo static site.
// It creates a staging docs directory with generated content and per-repo docs,
// then delegates to the standard SiteGenerator for HTML rendering.
func (g *CentralSiteGenerator) Generate() (int, error) {
	// Create staging docs directory.
	stagingDir := filepath.Join(g.OutputDir, ".staging-docs")
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return 0, fmt.Errorf("creating staging dir: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	// 1. Generate landing page.
	if err := g.writeLandingPage(stagingDir); err != nil {
		return 0, fmt.Errorf("writing landing page: %w", err)
	}

	// 2. Copy each repo's docs into a subdirectory.
	for _, repo := range g.Repos {
		if repo.DocsDir == "" {
			continue
		}
		if _, err := os.Stat(repo.DocsDir); os.IsNotExist(err) {
			continue
		}
		destDir := filepath.Join(stagingDir, repo.Name)
		if err := copyDir(repo.DocsDir, destDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not copy docs for %s: %v\n", repo.Name, err)
		}
		// Generate a repo index if the repo docs don't have one.
		indexPath := filepath.Join(destDir, "index.md")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			g.writeRepoIndex(destDir, repo)
		}
	}

	// 3. Generate system overview page.
	if err := g.writeSystemOverview(stagingDir); err != nil {
		return 0, fmt.Errorf("writing system overview: %w", err)
	}

	// 4. Generate flows page.
	if len(g.Flows) > 0 {
		if err := g.writeFlowsPage(stagingDir); err != nil {
			return 0, fmt.Errorf("writing flows page: %w", err)
		}
	}

	// 5. Copy service-map.html and other HTML artifacts from repos.
	for _, repo := range g.Repos {
		if repo.DocsDir == "" {
			continue
		}
		g.copyHTMLArtifacts(repo.DocsDir, stagingDir, repo.Name)
	}

	// 6. Delegate to standard SiteGenerator for HTML rendering.
	siteGen := NewSiteGenerator(stagingDir, g.OutputDir, g.ProjectName)
	siteGen.LogoPath = g.LogoPath
	return siteGen.Generate()
}

// writeLandingPage creates the main index.md with service cards and navigation.
func (g *CentralSiteGenerator) writeLandingPage(stagingDir string) error {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s\n\n", g.ProjectName))
	b.WriteString("Welcome to the central documentation hub. This site aggregates documentation from all registered services.\n\n")

	// Quick navigation.
	b.WriteString("## Quick Navigation\n\n")
	b.WriteString("- [System Overview](system-overview.md) — Architecture, dependencies, and system-level diagrams\n")
	if len(g.Flows) > 0 {
		b.WriteString("- [Cross-Service Flows](flows.md) — Data flows across services\n")
	}
	b.WriteString("\n")

	// Service cards table.
	if len(g.Repos) > 0 {
		b.WriteString("## Services\n\n")
		b.WriteString("| Service | Status | Files | Type | Summary |\n")
		b.WriteString("|---------|--------|-------|------|---------|\n")
		for _, repo := range g.Repos {
			displayName := repo.DisplayName
			if displayName == "" {
				displayName = repo.Name
			}
			summary := repo.Summary
			if len(summary) > 80 {
				summary = summary[:77] + "..."
			}
			// Link to the repo's docs subdirectory.
			link := fmt.Sprintf("[%s](%s/index.md)", displayName, repo.Name)
			statusBadge := repo.Status
			if statusBadge == "" {
				statusBadge = "unknown"
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %d | %s | %s |\n",
				link, statusBadge, repo.FileCount, repo.SourceType, summary))
		}
		b.WriteString("\n")
	}

	// Cross-service dependencies summary.
	if len(g.Links) > 0 {
		b.WriteString("## Dependencies Overview\n\n")
		b.WriteString("| From | To | Type | Reason |\n")
		b.WriteString("|------|----|------|--------|\n")
		for _, link := range g.Links {
			reason := link.Reason
			if len(reason) > 80 {
				reason = reason[:77] + "..."
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				link.FromRepo, link.ToRepo, link.LinkType, reason))
		}
		b.WriteString("\n")
	}

	return os.WriteFile(filepath.Join(stagingDir, "index.md"), []byte(b.String()), 0o644)
}

// writeRepoIndex creates an index.md for a repo subdirectory.
func (g *CentralSiteGenerator) writeRepoIndex(destDir string, repo RepoInfo) {
	var b strings.Builder

	displayName := repo.DisplayName
	if displayName == "" {
		displayName = repo.Name
	}

	b.WriteString(fmt.Sprintf("# %s\n\n", displayName))
	if repo.Summary != "" {
		b.WriteString(repo.Summary + "\n\n")
	}

	b.WriteString(fmt.Sprintf("- **Status:** %s\n", repo.Status))
	b.WriteString(fmt.Sprintf("- **Files:** %d\n", repo.FileCount))
	b.WriteString(fmt.Sprintf("- **Source:** %s\n", repo.SourceType))
	b.WriteString("\n")

	// List the docs files in this directory.
	entries, err := os.ReadDir(destDir)
	if err == nil && len(entries) > 0 {
		b.WriteString("## Documentation\n\n")
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "index.md" {
				continue
			}
			title := strings.TrimSuffix(e.Name(), ".md")
			b.WriteString(fmt.Sprintf("- [%s](%s)\n", title, e.Name()))
		}
		b.WriteString("\n")
	}

	_ = os.WriteFile(filepath.Join(destDir, "index.md"), []byte(b.String()), 0o644)
}

// writeSystemOverview creates the system-overview.md page.
func (g *CentralSiteGenerator) writeSystemOverview(stagingDir string) error {
	var b strings.Builder

	b.WriteString("# System Overview\n\n")

	// Services table.
	if len(g.Repos) > 0 {
		b.WriteString("## Registered Services\n\n")
		b.WriteString("| Service | Status | Files | Type | Summary |\n")
		b.WriteString("|---------|--------|-------|------|---------|\n")
		for _, repo := range g.Repos {
			displayName := repo.DisplayName
			if displayName == "" {
				displayName = repo.Name
			}
			summary := repo.Summary
			if len(summary) > 100 {
				summary = summary[:97] + "..."
			}
			b.WriteString(fmt.Sprintf("| **%s** | %s | %d | %s | %s |\n",
				displayName, repo.Status, repo.FileCount, repo.SourceType, summary))
		}
		b.WriteString("\n")
	}

	// Dependencies table.
	if len(g.Links) > 0 {
		b.WriteString("## Cross-Service Dependencies\n\n")
		b.WriteString("| From | To | Type | Reason |\n")
		b.WriteString("|------|----|------|--------|\n")
		for _, link := range g.Links {
			reason := link.Reason
			if len(reason) > 100 {
				reason = reason[:97] + "..."
			}
			endpoints := ""
			if len(link.Endpoints) > 0 {
				endpoints = " (" + strings.Join(link.Endpoints, ", ") + ")"
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s%s |\n",
				link.FromRepo, link.ToRepo, link.LinkType, reason, endpoints))
		}
		b.WriteString("\n")
	}

	// Flows summary.
	if len(g.Flows) > 0 {
		b.WriteString("## Cross-Service Flows\n\n")
		for _, f := range g.Flows {
			b.WriteString(fmt.Sprintf("### %s\n\n", f.Name))
			if f.Description != "" {
				b.WriteString(f.Description + "\n\n")
			}
			if len(f.Services) > 0 {
				b.WriteString("**Services:** " + strings.Join(f.Services, ", ") + "\n\n")
			}
			if f.Diagram != "" {
				b.WriteString("```mermaid\n")
				b.WriteString(f.Diagram)
				b.WriteString("\n```\n\n")
			}
		}
	}

	// Interactive views.
	b.WriteString("## Interactive Views\n\n")
	b.WriteString("- [Service Map](service-map.html) — Interactive D3.js visualization of all services\n")
	b.WriteString("- [Interactive Code Map](interactive-map.html) — File-level dependency graph\n")
	if len(g.Flows) > 0 {
		b.WriteString("- [Cross-Service Flows](flows.md) — Detailed flow narratives\n")
	}
	b.WriteString("\n")

	return os.WriteFile(filepath.Join(stagingDir, "system-overview.md"), []byte(b.String()), 0o644)
}

// writeFlowsPage creates the flows.md page with all cross-service flow narratives.
func (g *CentralSiteGenerator) writeFlowsPage(stagingDir string) error {
	var b strings.Builder

	b.WriteString("# Cross-Service Flows\n\n")
	b.WriteString("This page describes the data flows that span multiple services in the system.\n\n")

	for _, f := range g.Flows {
		b.WriteString(fmt.Sprintf("## %s\n\n", f.Name))
		if f.Description != "" {
			b.WriteString(f.Description + "\n\n")
		}
		if f.Narrative != "" {
			b.WriteString(f.Narrative + "\n\n")
		}
		if len(f.Services) > 0 {
			b.WriteString("**Services involved:** " + strings.Join(f.Services, ", ") + "\n\n")
		}
		if f.Diagram != "" {
			b.WriteString("```mermaid\n")
			b.WriteString(f.Diagram)
			b.WriteString("\n```\n\n")
		}
		b.WriteString("---\n\n")
	}

	return os.WriteFile(filepath.Join(stagingDir, "flows.md"), []byte(b.String()), 0o644)
}

// copyHTMLArtifacts copies standalone HTML files (service-map, interactive-map) from a repo.
func (g *CentralSiteGenerator) copyHTMLArtifacts(srcDocsDir, stagingDir, repoName string) {
	// Copy HTML files from repo docs to staging root (for service-level artifacts).
	_ = filepath.Walk(srcDocsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		rel, err := filepath.Rel(srcDocsDir, path)
		if err != nil {
			return nil
		}
		// For system-level files (service-map, interactive-map), copy to staging root.
		// For repo-specific files, copy to repo subdirectory.
		baseName := filepath.Base(rel)
		var destPath string
		if baseName == "service-map.html" || baseName == "interactive-map.html" {
			// Only copy to root if it doesn't already exist (first repo wins).
			destPath = filepath.Join(stagingDir, baseName)
			if _, err := os.Stat(destPath); err == nil {
				return nil
			}
		} else {
			destPath = filepath.Join(stagingDir, repoName, rel)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		_ = os.WriteFile(destPath, data, 0o644)
		return nil
	})
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}

		return copyFile(path, destPath)
	})
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
