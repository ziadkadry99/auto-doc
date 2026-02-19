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
	Overview            string
	Languages           string
	Components          []diagrams.Component
	ServiceDependencies string
	CriticalPath        string
	EntryPoints         []EntryPoint
	ExitPoints          []ExitPoint
	DataFlow            string
	DesignPatterns      []string
	ArchDiagram         string
	DepDiagram          string
}

// GenerateArchitecture creates an architecture overview document by aggregating
// file analyses and sending them to an LLM for high-level summarisation.
// The result is written to {OutputDir}/docs/architecture.md.
func (g *DocGenerator) GenerateArchitecture(ctx context.Context, analyses []indexer.FileAnalysis, provider llm.Provider, model string) error {
	// Build a compact representation of all files for the LLM.
	var summary strings.Builder
	for _, a := range analyses {
		fmt.Fprintf(&summary, "- %s [%s]: %s\n", a.FilePath, a.Language, a.Summary)
	}

	// Collect inter-service dependencies for the prompt.
	var depSummary strings.Builder
	for _, a := range analyses {
		for _, d := range a.Dependencies {
			if d.Type == "api_call" || d.Type == "grpc" || d.Type == "database" || d.Type == "event" {
				fmt.Fprintf(&depSummary, "- %s depends on %s (%s)\n", a.FilePath, d.Name, d.Type)
			}
		}
	}

	prompt := fmt.Sprintf(`Given the following source files and their summaries, describe the overall architecture of this project.

Files (with language in brackets):
%s

Inter-service dependencies:
%s

Please respond with the following sections separated by the markers shown:

===OVERVIEW===
A 2-4 paragraph overview of the architecture. Mention ALL programming languages used and how many services/components exist.

===LANGUAGES===
List each service/component and its primary programming language, one per line:
ServiceName: Language (port if known)
Example: Frontend: Go (port 8080)

===COMPONENTS===
List each major component, one per line, in the format: ComponentName: Description

===SERVICE_DEPENDENCIES===
List which services call which other services, one per line:
ServiceA -> ServiceB: protocol (reason)
Example: Frontend -> ProductCatalogService: gRPC (fetch product listings)

===ENTRY_POINTS===
List every way a user or external system can interact with this project (CLI commands, API endpoints, MCP tools, event handlers, etc.).
Each entry as:
ENTRY: Name of the entry point
TYPE: Category (e.g. CLI Command, API Endpoint, MCP Tool, Event Handler)
DESCRIPTION: What it does
Include specific port numbers and route paths when known.

===EXIT_POINTS===
List every output or side effect this project produces (files written, API calls made, databases updated, messages sent, etc.).
Each entry as:
EXIT: Name of the exit point
TYPE: Category (e.g. File Output, API Call, Database Write, Network Request)
DESCRIPTION: What it produces

===DATAFLOW===
Describe how data flows through the system in 2-3 paragraphs. Include specific service names and protocols.

===CRITICAL_PATH===
Analyze failure modes and single points of failure (SPOFs). For each service/component, classify its failure impact:
- COMPLETE OUTAGE: The entire system becomes unusable if this service fails (e.g. the only user-facing entry point)
- NEAR-COMPLETE OUTAGE: Most functionality breaks (e.g. a service called by nearly all other services)
- HIGH BLAST RADIUS: Multiple important features break but some functionality remains
- DEGRADED: Only specific features are affected, core functionality continues
List each service with its failure classification and explain why.

===PATTERNS===
List design patterns used, one per line.`, summary.String(), depSummary.String())

	resp, err := provider.Complete(ctx, llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a software architect analyzing a codebase. Be concise and factual. Always include concrete details like port numbers, specific languages per service, and exact protocol names."},
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   8192,
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

	// Build architecture diagram from parsed components and service dependencies.
	if len(data.Components) > 1 {
		// Cap at 12 components for readability.
		if len(data.Components) > 12 {
			data.Components = data.Components[:12]
		}

		// Assign layer groups via keyword matching for visual grouping.
		type layerInfo struct {
			name     string
			keywords []string
		}
		layers := []layerInfo{
			{"Interface", []string{"cli", "command", "api", "endpoint", "server", "handler", "http", "grpc", "mcp", "ui", "frontend", "gateway", "route"}},
			{"Core", []string{"engine", "core", "index", "pipeline", "process", "analyz", "logic", "walker", "traversal", "service", "manager"}},
			{"Storage", []string{"store", "storage", "database", "db", "vector", "embed", "persist", "cache", "queue", "redis", "mongo"}},
			{"Output", []string{"output", "doc", "generat", "render", "template", "diagram", "site", "report", "format", "export"}},
		}
		for i := range data.Components {
			lower := strings.ToLower(data.Components[i].Name + " " + data.Components[i].Description)
			for _, l := range layers {
				matched := false
				for _, kw := range l.keywords {
					if strings.Contains(lower, kw) {
						data.Components[i].Group = l.name
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if data.Components[i].Group == "" {
				data.Components[i].Group = "Core"
			}
		}

		rels := parseServiceDependencies(data.ServiceDependencies, data.Components)
		// Fallback: if no relationships were parsed, connect adjacent layers
		// top-down for a clean flow instead of a star pattern.
		if len(rels) == 0 {
			var layerReps []string
			for _, l := range layers {
				for _, c := range data.Components {
					if c.Group == l.name {
						layerReps = append(layerReps, c.Name)
						break
					}
				}
			}
			for i := 0; i < len(layerReps)-1; i++ {
				rels = append(rels, diagrams.Relationship{From: layerReps[i], To: layerReps[i+1]})
			}
		}
		data.ArchDiagram = diagrams.ArchitectureDiagram(data.Components, rels)
	}

	tmpl, err := template.New("arch").Funcs(templateFuncs).Parse(architectureTemplate)
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

	// All section markers used in the architecture response.
	allArchMarkers := []string{
		"===OVERVIEW===",
		"===LANGUAGES===",
		"===COMPONENTS===",
		"===SERVICE_DEPENDENCIES===",
		"===CRITICAL_PATH===",
		"===ENTRY_POINTS===",
		"===EXIT_POINTS===",
		"===DATAFLOW===",
		"===PATTERNS===",
	}

	// findEnd returns the index of the nearest following section marker.
	findEnd := func(text, currentMarker string) int {
		end := len(text)
		for _, m := range allArchMarkers {
			if m == currentMarker {
				continue
			}
			if i := strings.Index(text, m); i >= 0 && i < end {
				end = i
			}
		}
		return end
	}

	// Extract simple text sections.
	sections := map[string]*string{
		"===OVERVIEW===":              &data.Overview,
		"===LANGUAGES===":             &data.Languages,
		"===SERVICE_DEPENDENCIES===":  &data.ServiceDependencies,
		"===CRITICAL_PATH===":         &data.CriticalPath,
		"===DATAFLOW===":              &data.DataFlow,
	}
	for marker, field := range sections {
		if idx := strings.Index(content, marker); idx >= 0 {
			after := content[idx+len(marker):]
			end := findEnd(after, marker)
			*field = strings.TrimSpace(after[:end])
		}
	}

	// Parse components.
	if idx := strings.Index(content, "===COMPONENTS==="); idx >= 0 {
		after := content[idx+len("===COMPONENTS==="):]
		end := findEnd(after, "===COMPONENTS===")
		lines := strings.Split(strings.TrimSpace(after[:end]), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Strip common list prefixes from LLM output.
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "* ")
			parts := strings.SplitN(line, ":", 2)
			name := strings.TrimSpace(parts[0])
			// Strip directory-style trailing slashes and backtick wrapping.
			name = strings.TrimSuffix(name, "/")
			name = strings.Trim(name, "`")
			if name == "" {
				continue
			}
			c := diagrams.Component{Name: name}
			if len(parts) == 2 {
				c.Description = strings.TrimSpace(parts[1])
			}
			data.Components = append(data.Components, c)
		}
	}

	// Parse entry points.
	if idx := strings.Index(content, "===ENTRY_POINTS==="); idx >= 0 {
		after := content[idx+len("===ENTRY_POINTS==="):]
		end := findEnd(after, "===ENTRY_POINTS===")
		lines := strings.Split(strings.TrimSpace(after[:end]), "\n")
		var current *EntryPoint
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "ENTRY:") {
				if current != nil {
					data.EntryPoints = append(data.EntryPoints, *current)
				}
				current = &EntryPoint{
					Name: strings.TrimSpace(strings.TrimPrefix(line, "ENTRY:")),
				}
			} else if strings.HasPrefix(line, "TYPE:") && current != nil {
				current.Type = strings.TrimSpace(strings.TrimPrefix(line, "TYPE:"))
			} else if strings.HasPrefix(line, "DESCRIPTION:") && current != nil {
				current.Description = strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
			}
		}
		if current != nil {
			data.EntryPoints = append(data.EntryPoints, *current)
		}
	}

	// Parse exit points.
	if idx := strings.Index(content, "===EXIT_POINTS==="); idx >= 0 {
		after := content[idx+len("===EXIT_POINTS==="):]
		end := findEnd(after, "===EXIT_POINTS===")
		lines := strings.Split(strings.TrimSpace(after[:end]), "\n")
		var current *ExitPoint
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "EXIT:") {
				if current != nil {
					data.ExitPoints = append(data.ExitPoints, *current)
				}
				current = &ExitPoint{
					Name: strings.TrimSpace(strings.TrimPrefix(line, "EXIT:")),
				}
			} else if strings.HasPrefix(line, "TYPE:") && current != nil {
				current.Type = strings.TrimSpace(strings.TrimPrefix(line, "TYPE:"))
			} else if strings.HasPrefix(line, "DESCRIPTION:") && current != nil {
				current.Description = strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
			}
		}
		if current != nil {
			data.ExitPoints = append(data.ExitPoints, *current)
		}
	}

	// Parse patterns.
	if idx := strings.Index(content, "===PATTERNS==="); idx >= 0 {
		after := content[idx+len("===PATTERNS==="):]
		end := findEnd(after, "===PATTERNS===")
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

// parseServiceDependencies extracts relationships from the SERVICE_DEPENDENCIES
// section text. Expected format: "ServiceA -> ServiceB: protocol (reason)"
// It fuzzy-matches service names against the parsed component list.
func parseServiceDependencies(depsText string, components []diagrams.Component) []diagrams.Relationship {
	if depsText == "" {
		return nil
	}

	// Build a lookup: lowercased component name → canonical name.
	compNames := make(map[string]string)
	for _, c := range components {
		compNames[strings.ToLower(c.Name)] = c.Name
	}

	// Fuzzy match a name against known components.
	matchComponent := func(name string) string {
		name = strings.TrimSpace(name)
		lower := strings.ToLower(name)
		// Exact match.
		if canonical, ok := compNames[lower]; ok {
			return canonical
		}
		// Substring containment.
		for key, canonical := range compNames {
			if strings.Contains(lower, key) || strings.Contains(key, lower) {
				return canonical
			}
		}
		return ""
	}

	seen := make(map[string]bool)
	var rels []diagrams.Relationship
	for _, line := range strings.Split(depsText, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		// Look for "A -> B" pattern.
		arrowIdx := strings.Index(line, "->")
		if arrowIdx < 0 {
			continue
		}
		from := line[:arrowIdx]
		rest := line[arrowIdx+2:]
		// Strip label after colon: "B: protocol (reason)" → "B"
		if colonIdx := strings.Index(rest, ":"); colonIdx >= 0 {
			rest = rest[:colonIdx]
		}
		fromComp := matchComponent(from)
		toComp := matchComponent(rest)
		if fromComp == "" || toComp == "" || fromComp == toComp {
			continue
		}
		key := fromComp + "|" + toComp
		if seen[key] {
			continue
		}
		seen[key] = true
		rels = append(rels, diagrams.Relationship{From: fromComp, To: toComp})
	}
	return rels
}
