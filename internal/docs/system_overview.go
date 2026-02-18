package docs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ziadkadry99/auto-doc/internal/flows"
	"github.com/ziadkadry99/auto-doc/internal/llm"
)

// GenerateSystemOverview creates a combined system-overview.md document
// aggregating information from all registered repos.
func GenerateSystemOverview(ctx context.Context, outputDir string, repos []ServiceInfo, links []ServiceLinkInfo, allFlows []flows.Flow, provider llm.Provider, model string) error {
	var b strings.Builder

	b.WriteString("# System Overview\n\n")

	// Generate LLM summary if available.
	if provider != nil && len(repos) > 0 {
		summary := generateSystemSummary(ctx, repos, links, provider, model)
		if summary != "" {
			b.WriteString(summary)
			b.WriteString("\n\n")
		}
	}

	// Service cards.
	b.WriteString("## Services\n\n")
	b.WriteString("| Service | Status | Files | Type | Summary |\n")
	b.WriteString("|---------|--------|-------|------|---------|\n")
	for _, repo := range repos {
		summary := repo.Summary
		if len(summary) > 80 {
			summary = summary[:77] + "..."
		}
		b.WriteString(fmt.Sprintf("| **%s** | %s | %d | %s | %s |\n",
			repo.DisplayName, repo.Status, repo.FileCount, repo.SourceType, summary))
	}
	b.WriteString("\n")

	// System architecture diagram.
	if len(repos) > 1 || len(links) > 0 {
		diagram, err := GenerateSystemDiagram(ctx, repos, links, provider, model)
		if err == nil && diagram != "" {
			b.WriteString("## System Architecture\n\n")
			b.WriteString("```mermaid\n")
			b.WriteString(diagram)
			b.WriteString("\n```\n\n")
		}
	}

	// Cross-service dependencies.
	if len(links) > 0 {
		b.WriteString("## Cross-Service Dependencies\n\n")
		b.WriteString("| From | To | Type | Reason |\n")
		b.WriteString("|------|----|------|--------|\n")
		for _, link := range links {
			reason := link.Reason
			if len(reason) > 80 {
				reason = reason[:77] + "..."
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				link.FromRepo, link.ToRepo, link.LinkType, reason))
		}
		b.WriteString("\n")
	}

	// Cross-service flows.
	if len(allFlows) > 0 {
		b.WriteString("## Cross-Service Flows\n\n")
		for _, f := range allFlows {
			b.WriteString(fmt.Sprintf("### %s\n\n", f.Name))
			if f.Description != "" {
				b.WriteString(f.Description + "\n\n")
			}
			if f.Narrative != "" {
				b.WriteString(f.Narrative + "\n\n")
			}
			if len(f.Services) > 0 {
				b.WriteString("**Services involved:** " + strings.Join(f.Services, ", ") + "\n\n")
			}
			if f.MermaidDiagram != "" {
				b.WriteString("```mermaid\n")
				b.WriteString(f.MermaidDiagram)
				b.WriteString("\n```\n\n")
			}
		}
	}

	// Interactive map link.
	b.WriteString("## Interactive Views\n\n")
	b.WriteString("- [Service Map](service-map.html) — Interactive D3.js visualization of all services and their connections\n")
	b.WriteString("- [Interactive Code Map](interactive-map.html) — File-level dependency graph\n")

	// Write the file.
	docsDir := filepath.Join(outputDir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(docsDir, "system-overview.md"), []byte(b.String()), 0o644)
}

func generateSystemSummary(ctx context.Context, repos []ServiceInfo, links []ServiceLinkInfo, provider llm.Provider, model string) string {
	var prompt strings.Builder
	prompt.WriteString("Provide a concise 2-3 paragraph overview of this distributed system based on the following services:\n\n")

	for _, repo := range repos {
		prompt.WriteString(fmt.Sprintf("- %s: %s (%d files, %s)\n", repo.Name, repo.Summary, repo.FileCount, repo.SourceType))
	}

	if len(links) > 0 {
		prompt.WriteString("\nDependencies:\n")
		for _, link := range links {
			prompt.WriteString(fmt.Sprintf("- %s -> %s (%s): %s\n", link.FromRepo, link.ToRepo, link.LinkType, link.Reason))
		}
	}

	prompt.WriteString("\nWrite a clear system overview paragraph. Don't use headers. Focus on the overall purpose and architecture.")

	resp, err := provider.Complete(ctx, llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a technical documentation writer. Write clear, concise system overviews."},
			{Role: llm.RoleUser, Content: prompt.String()},
		},
		MaxTokens:   1024,
		Temperature: 0.3,
	})
	if err != nil {
		return ""
	}

	return strings.TrimSpace(resp.Content)
}
