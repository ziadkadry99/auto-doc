package docs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ziadkadry99/auto-doc/internal/diagrams"
	"github.com/ziadkadry99/auto-doc/internal/indexer"
	"github.com/ziadkadry99/auto-doc/internal/llm"
)

// archData holds the data passed to the architecture markdown template.
type archData struct {
	Overview       string
	Components     []diagrams.Component
	DataFlow       string
	DesignPatterns []string
	ArchDiagram    string
	DepDiagram     string
}

// GenerateArchitecture creates an architecture overview document by aggregating
// file analyses and sending them to an LLM for high-level summarisation.
// The result is written to {OutputDir}/docs/architecture.md.
func (g *DocGenerator) GenerateArchitecture(ctx context.Context, analyses []indexer.FileAnalysis, provider llm.Provider, model string) error {
	// Build a compact representation of all files for the LLM.
	var summary strings.Builder
	for _, a := range analyses {
		fmt.Fprintf(&summary, "- %s: %s\n", a.FilePath, a.Summary)
	}

	prompt := fmt.Sprintf(`Given the following source files and their summaries, describe the overall architecture of this project.

Files:
%s

Please respond with the following sections separated by the markers shown:

===OVERVIEW===
A 2-4 paragraph overview of the architecture.

===COMPONENTS===
List each major component, one per line, in the format: ComponentName: Description

===DATAFLOW===
Describe how data flows through the system in 1-2 paragraphs.

===PATTERNS===
List design patterns used, one per line.`, summary.String())

	resp, err := provider.Complete(ctx, llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a software architect analyzing a codebase. Be concise and factual."},
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   2048,
		Temperature: 0.3,
	})
	if err != nil {
		return fmt.Errorf("architecture LLM call failed: %w", err)
	}

	data := parseArchResponse(resp.Content)

	// Build dependency diagram from file analyses.
	depMap := make(map[string][]string)
	for _, a := range analyses {
		if len(a.Dependencies) > 0 {
			var deps []string
			for _, d := range a.Dependencies {
				deps = append(deps, d.Name)
			}
			depMap[a.FilePath] = deps
		}
	}
	if len(depMap) > 0 {
		data.DepDiagram = diagrams.DependencyDiagram(depMap)
	}

	// Build architecture diagram from parsed components.
	if len(data.Components) > 1 {
		var rels []diagrams.Relationship
		for i := 0; i < len(data.Components)-1; i++ {
			rels = append(rels, diagrams.Relationship{
				From: data.Components[i].Name,
				To:   data.Components[i+1].Name,
			})
		}
		data.ArchDiagram = diagrams.ArchitectureDiagram(data.Components, rels)
	}

	tmpl, err := template.New("arch").Parse(architectureTemplate)
	if err != nil {
		return err
	}

	docsDir := filepath.Join(g.OutputDir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		return err
	}

	outPath := filepath.Join(docsDir, "architecture.md")
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

// parseArchResponse extracts structured sections from the LLM response.
func parseArchResponse(content string) archData {
	var data archData

	sections := map[string]*string{
		"===OVERVIEW===": &data.Overview,
		"===DATAFLOW===": &data.DataFlow,
	}

	// Split by section markers and populate fields.
	remaining := content
	for marker, field := range sections {
		if idx := strings.Index(remaining, marker); idx >= 0 {
			after := remaining[idx+len(marker):]
			// Find the next marker to delimit this section.
			end := len(after)
			for m := range sections {
				if m == marker {
					continue
				}
				if i := strings.Index(after, m); i >= 0 && i < end {
					end = i
				}
			}
			// Also check for ===COMPONENTS=== and ===PATTERNS===.
			for _, m := range []string{"===COMPONENTS===", "===PATTERNS==="} {
				if i := strings.Index(after, m); i >= 0 && i < end {
					end = i
				}
			}
			*field = strings.TrimSpace(after[:end])
		}
	}

	// Parse components.
	if idx := strings.Index(content, "===COMPONENTS==="); idx >= 0 {
		after := content[idx+len("===COMPONENTS==="):]
		end := len(after)
		for _, m := range []string{"===OVERVIEW===", "===DATAFLOW===", "===PATTERNS==="} {
			if i := strings.Index(after, m); i >= 0 && i < end {
				end = i
			}
		}
		lines := strings.Split(strings.TrimSpace(after[:end]), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, ":", 2)
			c := diagrams.Component{Name: strings.TrimSpace(parts[0])}
			if len(parts) == 2 {
				c.Description = strings.TrimSpace(parts[1])
			}
			data.Components = append(data.Components, c)
		}
	}

	// Parse patterns.
	if idx := strings.Index(content, "===PATTERNS==="); idx >= 0 {
		after := content[idx+len("===PATTERNS==="):]
		end := len(after)
		for _, m := range []string{"===OVERVIEW===", "===DATAFLOW===", "===COMPONENTS==="} {
			if i := strings.Index(after, m); i >= 0 && i < end {
				end = i
			}
		}
		lines := strings.Split(strings.TrimSpace(after[:end]), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "* ")
			if line != "" {
				data.DesignPatterns = append(data.DesignPatterns, line)
			}
		}
	}

	// Fallback: if no markers were found, use the whole content as overview.
	if data.Overview == "" && data.DataFlow == "" && len(data.Components) == 0 {
		data.Overview = strings.TrimSpace(content)
	}

	return data
}
