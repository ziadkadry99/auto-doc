package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ziadkadry99/auto-doc/internal/contextengine"
	"github.com/ziadkadry99/auto-doc/internal/flows"
	"github.com/ziadkadry99/auto-doc/internal/indexer"
	"github.com/ziadkadry99/auto-doc/internal/llm"
)

// Linker discovers cross-service dependencies when repos are added/updated.
type Linker struct {
	store    *Store
	ctxStore *contextengine.Store
	flowStore *flows.Store
}

// NewLinker creates a new cross-service link discovery engine.
func NewLinker(store *Store, ctxStore *contextengine.Store, flowStore *flows.Store) *Linker {
	return &Linker{
		store:     store,
		ctxStore:  ctxStore,
		flowStore: flowStore,
	}
}

// linkDiscoveryResult is the expected JSON structure from the LLM.
type linkDiscoveryResult struct {
	Dependencies []linkDep  `json:"dependencies"`
	Flows        []linkFlow `json:"flows"`
}

type linkDep struct {
	From      string   `json:"from"`
	To        string   `json:"to"`
	Type      string   `json:"type"`
	Reason    string   `json:"reason"`
	Endpoints []string `json:"endpoints"`
}

type linkFlow struct {
	Name      string   `json:"name"`
	Services  []string `json:"services"`
	Narrative string   `json:"narrative"`
}

// DiscoverLinks analyzes a newly added/updated repo and discovers how it relates to other repos.
func (l *Linker) DiscoverLinks(ctx context.Context, repo *Repository, provider llm.Provider, model string) error {
	// Load all repos.
	allRepos, err := l.store.List(ctx)
	if err != nil {
		return fmt.Errorf("listing repos: %w", err)
	}

	// Need at least 2 repos for link discovery.
	if len(allRepos) < 2 {
		return nil
	}

	// Load the new repo's analyses.
	analyses, err := indexer.LoadAnalyses(repo.LocalPath)
	if err != nil {
		return fmt.Errorf("loading analyses for %s: %w", repo.Name, err)
	}

	// Detect cross-service calls from source files.
	detector := flows.NewDetector()
	imp := &Importer{store: l.store, detector: detector}
	calls := imp.DetectCrossServiceCalls(repo.LocalPath)

	// Build the LLM prompt.
	prompt := buildLinkDiscoveryPrompt(repo, allRepos, analyses, calls, detector)

	resp, err := provider.Complete(ctx, llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: linkDiscoverySystemPrompt},
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   4096,
		Temperature: 0.2,
		JSONMode:    true,
	})
	if err != nil {
		return fmt.Errorf("LLM completion for link discovery: %w", err)
	}

	// Parse the result.
	result, err := parseLinkDiscoveryResult(resp.Content)
	if err != nil {
		return fmt.Errorf("parsing link discovery result: %w", err)
	}

	// Delete old links for this repo before saving new ones.
	l.store.DeleteLinks(ctx, repo.Name)

	// Save discovered dependencies as service links.
	for _, dep := range result.Dependencies {
		link := &ServiceLink{
			FromRepo:  dep.From,
			ToRepo:    dep.To,
			LinkType:  dep.Type,
			Reason:    dep.Reason,
			Endpoints: dep.Endpoints,
		}
		if err := l.store.SaveLink(ctx, link); err != nil {
			// Non-fatal: log and continue.
			_ = err
		}
	}

	// Save discovered flows.
	if l.flowStore != nil {
		for _, f := range result.Flows {
			flow := &flows.Flow{
				Name:        f.Name,
				Description: f.Narrative,
				Narrative:   f.Narrative,
				Services:    f.Services,
			}
			l.flowStore.CreateFlow(ctx, flow)
		}
	}

	// Save facts about the relationships.
	if l.ctxStore != nil {
		for _, dep := range result.Dependencies {
			fact := contextengine.Fact{
				Scope:      "service",
				ScopeID:    dep.From,
				Key:        "dependency",
				Value:      fmt.Sprintf("Depends on %s via %s: %s", dep.To, dep.Type, dep.Reason),
				Source:     "auto_detected",
				ProvidedBy: "link-discovery",
			}
			l.ctxStore.SaveFact(ctx, fact)
		}
	}

	return nil
}

// GetAffectedRepos returns repos whose documentation might need updating when changedRepo changes.
func (l *Linker) GetAffectedRepos(ctx context.Context, changedRepo string) ([]string, error) {
	links, err := l.store.GetLinks(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("getting links: %w", err)
	}

	// Build adjacency lists.
	dependsOn := make(map[string][]string)  // repo -> repos it depends on
	dependedBy := make(map[string][]string) // repo -> repos that depend on it

	for _, link := range links {
		dependsOn[link.FromRepo] = append(dependsOn[link.FromRepo], link.ToRepo)
		dependedBy[link.ToRepo] = append(dependedBy[link.ToRepo], link.FromRepo)
	}

	// BFS from changedRepo through both directions.
	affected := make(map[string]bool)
	queue := []string{changedRepo}
	visited := map[string]bool{changedRepo: true}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Repos that depend on the current one need updating.
		for _, dep := range dependedBy[current] {
			if !visited[dep] {
				visited[dep] = true
				affected[dep] = true
				queue = append(queue, dep)
			}
		}

		// Repos that the current one depends on are also affected.
		for _, dep := range dependsOn[current] {
			if !visited[dep] {
				visited[dep] = true
				affected[dep] = true
			}
		}
	}

	result := make([]string, 0, len(affected))
	for repo := range affected {
		result = append(result, repo)
	}
	return result, nil
}

const linkDiscoverySystemPrompt = `You are analyzing how services in a distributed system interact.
You will be given information about multiple repositories/services and must identify dependencies between them.

You MUST respond with valid JSON matching this schema:
{
  "dependencies": [
    {"from": "service-a", "to": "service-b", "type": "http|grpc|kafka|amqp", "reason": "why this dependency exists", "endpoints": ["/api/endpoint"]}
  ],
  "flows": [
    {"name": "Flow Name", "services": ["service-a", "service-b"], "narrative": "1-paragraph description of the flow"}
  ]
}

Rules:
- Only report dependencies you have evidence for (from detected calls, shared topics, etc.)
- The "from" field is the service that initiates the call
- The "to" field is the service that receives the call
- If no dependencies are found, return empty arrays
- Be conservative: only report clear dependencies, not speculative ones`

func buildLinkDiscoveryPrompt(newRepo *Repository, allRepos []Repository, analyses map[string]indexer.FileAnalysis, calls []flows.CrossServiceCall, _ *flows.Detector) string {
	var b strings.Builder

	b.WriteString("## Known Services\n\n")
	for _, repo := range allRepos {
		if repo.Name == newRepo.Name {
			continue
		}
		b.WriteString(fmt.Sprintf("SERVICE: %s\n", repo.Name))
		if repo.Summary != "" {
			b.WriteString(fmt.Sprintf("SUMMARY: %s\n", repo.Summary))
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("## New/Updated Service: %s\n", newRepo.Name))
	if newRepo.Summary != "" {
		b.WriteString(fmt.Sprintf("SUMMARY: %s\n\n", newRepo.Summary))
	}

	// Include entry points from analyses.
	b.WriteString("### Entry Points (exported functions/handlers)\n")
	entryCount := 0
	for filePath, analysis := range analyses {
		for _, fn := range analysis.Functions {
			if len(fn.Name) > 0 && fn.Name[0] >= 'A' && fn.Name[0] <= 'Z' {
				b.WriteString(fmt.Sprintf("- %s in %s: %s\n", fn.Name, filePath, fn.Summary))
				entryCount++
				if entryCount > 30 {
					break
				}
			}
		}
		if entryCount > 30 {
			break
		}
	}

	// Include detected outbound calls.
	if len(calls) > 0 {
		b.WriteString("\n### Detected Outbound Calls\n")
		for i, call := range calls {
			b.WriteString(fmt.Sprintf("- %s call to %s (%s) in %s:%d\n", call.Type, call.Target, call.Method, call.FilePath, call.Line))
			if i > 50 {
				b.WriteString("(truncated)\n")
				break
			}
		}
	}

	// Include dependencies from analyses.
	b.WriteString("\n### Dependencies from File Analyses\n")
	depCount := 0
	for filePath, analysis := range analyses {
		for _, dep := range analysis.Dependencies {
			if dep.Type == "api_call" || dep.Type == "grpc" || dep.Type == "database" || dep.Type == "event" {
				b.WriteString(fmt.Sprintf("- %s depends on %s (%s)\n", filePath, dep.Name, dep.Type))
				depCount++
				if depCount > 30 {
					break
				}
			}
		}
		if depCount > 30 {
			break
		}
	}

	b.WriteString("\nAnalyze the above and identify cross-service dependencies. Respond with JSON.\n")

	return b.String()
}

func parseLinkDiscoveryResult(content string) (*linkDiscoveryResult, error) {
	jsonStr := content
	if idx := strings.Index(content, "{"); idx >= 0 {
		jsonStr = content[idx:]
	}
	if idx := strings.LastIndex(jsonStr, "}"); idx >= 0 {
		jsonStr = jsonStr[:idx+1]
	}

	var result linkDiscoveryResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return &linkDiscoveryResult{}, nil
	}

	return &result, nil
}
