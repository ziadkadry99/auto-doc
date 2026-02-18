package docs

import (
	"context"
	"fmt"
	"strings"

	"github.com/ziadkadry99/auto-doc/internal/llm"
)

// ServiceInfo represents a service/repository for system-level documentation.
type ServiceInfo struct {
	Name        string
	DisplayName string
	Summary     string
	FileCount   int
	SourceType  string
	Status      string
}

// ServiceLinkInfo represents a cross-service dependency for documentation.
type ServiceLinkInfo struct {
	FromRepo  string
	ToRepo    string
	LinkType  string
	Reason    string
	Endpoints []string
}

// GenerateSystemDiagram creates a Mermaid diagram showing all services and their cross-service links.
func GenerateSystemDiagram(ctx context.Context, repos []ServiceInfo, links []ServiceLinkInfo, provider llm.Provider, model string) (string, error) {
	if len(repos) == 0 {
		return "", nil
	}

	// If no LLM available or few repos, use programmatic approach.
	if provider == nil || len(repos) <= 3 {
		return buildProgrammaticDiagram(repos, links), nil
	}

	// Build context for the LLM.
	prompt := buildSystemDiagramPrompt(repos, links)

	resp, err := provider.Complete(ctx, llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: systemDiagramSystemPrompt},
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   2048,
		Temperature: 0.2,
	})
	if err != nil {
		// Fallback to programmatic diagram on error.
		return buildProgrammaticDiagram(repos, links), nil
	}

	diagram := extractMermaidFromResponse(resp.Content)
	if diagram == "" {
		return buildProgrammaticDiagram(repos, links), nil
	}

	return sanitizeMermaid(diagram), nil
}

const systemDiagramSystemPrompt = `You are a system architecture diagram generator. Create clean, readable Mermaid flowchart diagrams showing service interactions.

Rules:
- Use 'flowchart LR' or 'flowchart TB' direction
- Use descriptive node labels with the service name
- Use different arrow styles for different communication types:
  - HTTP: -->|HTTP|
  - gRPC: -->|gRPC|
  - Kafka: -.->|Kafka|
  - AMQP: -.->|AMQP|
- Group related services using subgraph blocks if there are more than 5 services
- Maximum 15 nodes in the diagram
- Keep it clean and readable
- Return ONLY the Mermaid diagram code, no markdown fences`

func buildSystemDiagramPrompt(repos []ServiceInfo, links []ServiceLinkInfo) string {
	var b strings.Builder

	b.WriteString("Create a Mermaid system architecture diagram for these services:\n\n")

	b.WriteString("## Services\n")
	for _, repo := range repos {
		b.WriteString(fmt.Sprintf("- %s: %s (%d files)\n", repo.Name, repo.Summary, repo.FileCount))
	}

	if len(links) > 0 {
		b.WriteString("\n## Dependencies\n")
		for _, link := range links {
			endpoints := ""
			if len(link.Endpoints) > 0 {
				endpoints = fmt.Sprintf(" [%s]", strings.Join(link.Endpoints, ", "))
			}
			b.WriteString(fmt.Sprintf("- %s --> %s (%s): %s%s\n", link.FromRepo, link.ToRepo, link.LinkType, link.Reason, endpoints))
		}
	}

	return b.String()
}

func buildProgrammaticDiagram(repos []ServiceInfo, links []ServiceLinkInfo) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")

	// Create nodes.
	for _, repo := range repos {
		id := sanitizeMermaidID(repo.Name)
		label := repo.DisplayName
		if label == "" {
			label = repo.Name
		}
		b.WriteString(fmt.Sprintf("    %s[%s]\n", id, label))
	}

	// Create edges.
	for _, link := range links {
		fromID := sanitizeMermaidID(link.FromRepo)
		toID := sanitizeMermaidID(link.ToRepo)

		switch link.LinkType {
		case "kafka", "amqp":
			b.WriteString(fmt.Sprintf("    %s -.->|%s| %s\n", fromID, link.LinkType, toID))
		default:
			b.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", fromID, link.LinkType, toID))
		}
	}

	return b.String()
}

func extractMermaidFromResponse(content string) string {
	// Try to extract mermaid code from markdown fences.
	if idx := strings.Index(content, "```mermaid"); idx >= 0 {
		content = content[idx+len("```mermaid"):]
		if endIdx := strings.Index(content, "```"); endIdx >= 0 {
			return strings.TrimSpace(content[:endIdx])
		}
	}
	if idx := strings.Index(content, "```"); idx >= 0 {
		content = content[idx+3:]
		if endIdx := strings.Index(content, "```"); endIdx >= 0 {
			return strings.TrimSpace(content[:endIdx])
		}
	}
	// Return as-is if it looks like a mermaid diagram.
	if strings.HasPrefix(strings.TrimSpace(content), "flowchart") || strings.HasPrefix(strings.TrimSpace(content), "graph") {
		return strings.TrimSpace(content)
	}
	return ""
}
