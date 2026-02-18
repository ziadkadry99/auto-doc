package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/ziadkadry99/auto-doc/internal/contextengine"
	"github.com/ziadkadry99/auto-doc/internal/docs"
)

// handleSearchAcrossRepos performs semantic search across all indexed repos.
func (s *Server) handleSearchAcrossRepos(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: query"), nil
	}

	limit := request.GetInt("limit", 10)
	if limit <= 0 {
		limit = 10
	}

	// Search across all repos (no repo filter).
	results, err := s.store.Search(ctx, query, limit, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No results found across any repositories."), nil
	}

	return mcp.NewToolResultText(formatSearchResults(results)), nil
}

// handleGetServiceContext combines facts and search results to build complete service context.
func (s *Server) handleGetServiceContext(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	service, err := request.RequireString("service")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: service"), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Service Context: %s\n\n", service))

	// Get facts from context engine if available.
	if s.phase4 != nil && s.phase4.CtxStore != nil {
		facts, err := s.phase4.CtxStore.GetCurrentFacts(ctx, "", "service", service)
		if err == nil && len(facts) > 0 {
			sb.WriteString("## Known Facts\n\n")
			for _, f := range facts {
				sb.WriteString(fmt.Sprintf("- **%s**: %s (source: %s)\n", f.Key, f.Value, f.Source))
			}
			sb.WriteString("\n")
		}
	}

	// Get ownership info if available.
	if s.phase4 != nil && s.phase4.OrgStore != nil {
		ownerships, err := s.phase4.OrgStore.GetOwnership(ctx, service)
		if err == nil && len(ownerships) > 0 {
			sb.WriteString("## Ownership\n\n")
			for _, o := range ownerships {
				sb.WriteString(fmt.Sprintf("- Team: %s (confidence: %s, source: %s)\n", o.TeamID, o.Confidence, o.Source))
			}
			sb.WriteString("\n")
		}
	}

	// Search for related documentation.
	results, err := s.store.Search(ctx, service, 5, nil)
	if err == nil && len(results) > 0 {
		sb.WriteString("## Related Documentation\n\n")
		sb.WriteString(formatSearchResults(results))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// handleGetBlastRadius searches for references to a service across all docs.
func (s *Server) handleGetBlastRadius(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	service, err := request.RequireString("service")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: service"), nil
	}

	endpoint := request.GetString("endpoint", "")

	query := service
	if endpoint != "" {
		query = fmt.Sprintf("%s %s", service, endpoint)
	}

	// Search for all mentions of this service.
	results, err := s.store.Search(ctx, query, 20, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Blast Radius: %s", service))
	if endpoint != "" {
		sb.WriteString(fmt.Sprintf(" (endpoint: %s)", endpoint))
	}
	sb.WriteString("\n\n")

	if len(results) == 0 {
		sb.WriteString("No references found to this service in any documentation.\n")
		return mcp.NewToolResultText(sb.String()), nil
	}

	// Group results by file to identify affected services.
	affected := make(map[string]bool)
	for _, r := range results {
		if r.Document.Metadata.FilePath != "" {
			affected[r.Document.Metadata.FilePath] = true
		}
	}

	sb.WriteString(fmt.Sprintf("Found %d reference(s) across %d file(s):\n\n", len(results), len(affected)))

	// Check flows for this service if available.
	if s.phase4 != nil && s.phase4.FlowStore != nil {
		allFlows, err := s.phase4.FlowStore.ListFlows(ctx)
		if err == nil {
			var affectedFlows []string
			for _, f := range allFlows {
				for _, svc := range f.Services {
					if strings.EqualFold(svc, service) {
						affectedFlows = append(affectedFlows, f.Name)
						break
					}
				}
			}
			if len(affectedFlows) > 0 {
				sb.WriteString("## Affected Flows\n\n")
				for _, name := range affectedFlows {
					sb.WriteString(fmt.Sprintf("- %s\n", name))
				}
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("## References\n\n")
	sb.WriteString(formatSearchResults(results))

	return mcp.NewToolResultText(sb.String()), nil
}

// handleGetFlow retrieves a flow by name.
func (s *Server) handleGetFlow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	flowName, err := request.RequireString("flow_name")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: flow_name"), nil
	}

	if s.phase4 == nil || s.phase4.FlowStore == nil {
		return mcp.NewToolResultError("Flow store not configured. Phase 4 dependencies are required for this tool."), nil
	}

	// Search flows by name.
	allFlows, err := s.phase4.FlowStore.SearchFlows(ctx, flowName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("searching flows: %v", err)), nil
	}

	if len(allFlows) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("No flow found matching %q.", flowName)), nil
	}

	// Return the first match.
	f := allFlows[0]
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshaling flow: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Flow: %s\n\n", f.Name))
	if f.Description != "" {
		sb.WriteString(fmt.Sprintf("**Description**: %s\n\n", f.Description))
	}
	if f.Narrative != "" {
		sb.WriteString(fmt.Sprintf("## Narrative\n\n%s\n\n", f.Narrative))
	}
	if f.MermaidDiagram != "" {
		sb.WriteString(fmt.Sprintf("## Diagram\n\n```mermaid\n%s\n```\n\n", f.MermaidDiagram))
	}
	if len(f.Services) > 0 {
		sb.WriteString("## Services Involved\n\n")
		for _, svc := range f.Services {
			sb.WriteString(fmt.Sprintf("- %s\n", svc))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("## Raw Data\n\n```json\n")
	sb.Write(b)
	sb.WriteString("\n```\n")

	return mcp.NewToolResultText(sb.String()), nil
}

// handleAskArchitecture answers a free-form architecture question.
func (s *Server) handleAskArchitecture(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	question, err := request.RequireString("question")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: question"), nil
	}

	if s.phase4 == nil || s.phase4.CtxEngine == nil {
		return mcp.NewToolResultError("Context engine not configured. Phase 4 dependencies are required for this tool."), nil
	}

	answer, err := s.phase4.CtxEngine.AskQuestion(ctx, question)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("answering question: %v", err)), nil
	}

	return mcp.NewToolResultText(answer), nil
}

// handleGetTeamServices returns all services owned by a team.
func (s *Server) handleGetTeamServices(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	team, err := request.RequireString("team")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: team"), nil
	}

	if s.phase4 == nil || s.phase4.OrgStore == nil {
		return mcp.NewToolResultError("Org structure store not configured. Phase 4 dependencies are required for this tool."), nil
	}

	// Try finding the team by listing all teams and matching by name or ID.
	teams, err := s.phase4.OrgStore.ListTeams(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("listing teams: %v", err)), nil
	}

	var teamID string
	var teamName string
	for _, t := range teams {
		if strings.EqualFold(t.Name, team) || t.ID == team {
			teamID = t.ID
			teamName = t.Name
			if t.DisplayName != "" {
				teamName = t.DisplayName
			}
			break
		}
	}

	if teamID == "" {
		return mcp.NewToolResultError(fmt.Sprintf("Team %q not found.", team)), nil
	}

	ownerships, err := s.phase4.OrgStore.ListOwnerships(ctx, teamID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("listing ownerships: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Services Owned by %s\n\n", teamName))

	if len(ownerships) == 0 {
		sb.WriteString("No services found for this team.\n")
	} else {
		for _, o := range ownerships {
			sb.WriteString(fmt.Sprintf("- **%s** (confidence: %s, source: %s)\n", o.RepoID, o.Confidence, o.Source))
		}
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// handleProvideContext saves a user-provided fact about a service.
func (s *Server) handleProvideContext(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	service, err := request.RequireString("service")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: service"), nil
	}

	ctxValue, err := request.RequireString("context")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: context"), nil
	}

	if s.phase4 == nil || s.phase4.CtxStore == nil {
		return mcp.NewToolResultError("Context store not configured. Phase 4 dependencies are required for this tool."), nil
	}

	fact := contextengine.Fact{
		Scope:      "service",
		ScopeID:    service,
		Key:        "context",
		Value:      ctxValue,
		Source:     "mcp",
		ProvidedBy: "ai-assistant",
	}

	saved, err := s.phase4.CtxStore.SaveFact(ctx, fact)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("saving context: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Context saved for service %q (fact ID: %s, version: %d).", service, saved.ID, saved.Version)), nil
}

// handleListRepos lists all registered repositories.
func (s *Server) handleListRepos(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.phase4 == nil || s.phase4.RepoStore == nil {
		return mcp.NewToolResultError("Repository store not configured. Phase 4 dependencies are required for this tool."), nil
	}

	repos, err := s.phase4.RepoStore.List(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("listing repos: %v", err)), nil
	}

	if len(repos) == 0 {
		return mcp.NewToolResultText("No repositories registered. Use `autodoc repo add` to register repositories."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Registered Repositories (%d)\n\n", len(repos)))
	for _, r := range repos {
		sb.WriteString(fmt.Sprintf("## %s\n", r.DisplayName))
		sb.WriteString(fmt.Sprintf("- **Name**: %s\n", r.Name))
		sb.WriteString(fmt.Sprintf("- **Status**: %s\n", r.Status))
		sb.WriteString(fmt.Sprintf("- **Files**: %d\n", r.FileCount))
		sb.WriteString(fmt.Sprintf("- **Type**: %s\n", r.SourceType))
		if r.Summary != "" {
			sb.WriteString(fmt.Sprintf("- **Summary**: %s\n", r.Summary))
		}
		if r.LastIndexedAt != "" {
			sb.WriteString(fmt.Sprintf("- **Last indexed**: %s\n", r.LastIndexedAt))
		}
		sb.WriteString("\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// handleGetRepoDetails gets full details for a specific repo.
func (s *Server) handleGetRepoDetails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: name"), nil
	}

	if s.phase4 == nil || s.phase4.RepoStore == nil {
		return mcp.NewToolResultError("Repository store not configured. Phase 4 dependencies are required for this tool."), nil
	}

	repo, err := s.phase4.RepoStore.Get(ctx, name)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("getting repo: %v", err)), nil
	}
	if repo == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Repository %q not found.", name)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Repository: %s\n\n", repo.DisplayName))
	sb.WriteString(fmt.Sprintf("- **Name**: %s\n", repo.Name))
	sb.WriteString(fmt.Sprintf("- **Status**: %s\n", repo.Status))
	sb.WriteString(fmt.Sprintf("- **Files**: %d\n", repo.FileCount))
	sb.WriteString(fmt.Sprintf("- **Source type**: %s\n", repo.SourceType))
	if repo.SourceURL != "" {
		sb.WriteString(fmt.Sprintf("- **Source URL**: %s\n", repo.SourceURL))
	}
	sb.WriteString(fmt.Sprintf("- **Local path**: %s\n", repo.LocalPath))
	if repo.Summary != "" {
		sb.WriteString(fmt.Sprintf("- **Summary**: %s\n", repo.Summary))
	}
	if repo.LastCommitSHA != "" {
		sb.WriteString(fmt.Sprintf("- **Last commit**: %s\n", repo.LastCommitSHA))
	}
	if repo.LastIndexedAt != "" {
		sb.WriteString(fmt.Sprintf("- **Last indexed**: %s\n", repo.LastIndexedAt))
	}

	// Include cross-service links.
	links, err := s.phase4.RepoStore.GetLinks(ctx, name)
	if err == nil && len(links) > 0 {
		sb.WriteString("\n## Cross-Service Links\n\n")
		for _, link := range links {
			direction := "outgoing"
			other := link.ToRepo
			if link.ToRepo == name {
				direction = "incoming"
				other = link.FromRepo
			}
			sb.WriteString(fmt.Sprintf("- **%s** %s (%s): %s\n", other, direction, link.LinkType, link.Reason))
		}
	}

	// Include ownership info.
	if s.phase4.OrgStore != nil {
		ownerships, err := s.phase4.OrgStore.GetOwnership(ctx, name)
		if err == nil && len(ownerships) > 0 {
			sb.WriteString("\n## Ownership\n\n")
			for _, o := range ownerships {
				sb.WriteString(fmt.Sprintf("- Team: %s (confidence: %s, source: %s)\n", o.TeamID, o.Confidence, o.Source))
			}
		}
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// handleGetSystemDiagram returns the combined Mermaid system diagram.
func (s *Server) handleGetSystemDiagram(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.phase4 == nil || s.phase4.RepoStore == nil {
		return mcp.NewToolResultError("Repository store not configured. Phase 4 dependencies are required for this tool."), nil
	}

	repos, err := s.phase4.RepoStore.List(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("listing repos: %v", err)), nil
	}

	links, err := s.phase4.RepoStore.GetLinks(ctx, "")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("getting links: %v", err)), nil
	}

	// Convert to docs types.
	docRepos := make([]docs.ServiceInfo, len(repos))
	for i, r := range repos {
		docRepos[i] = docs.ServiceInfo{
			Name: r.Name, DisplayName: r.DisplayName, Summary: r.Summary,
			FileCount: r.FileCount, SourceType: r.SourceType, Status: r.Status,
		}
	}
	docLinks := make([]docs.ServiceLinkInfo, len(links))
	for i, l := range links {
		docLinks[i] = docs.ServiceLinkInfo{
			FromRepo: l.FromRepo, ToRepo: l.ToRepo, LinkType: l.LinkType,
			Reason: l.Reason, Endpoints: l.Endpoints,
		}
	}

	diagram, err := docs.GenerateSystemDiagram(ctx, docRepos, docLinks, nil, "")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("generating diagram: %v", err)), nil
	}

	if diagram == "" {
		return mcp.NewToolResultText("No system diagram available (no repositories registered)."), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("```mermaid\n%s\n```", diagram)), nil
}

// handleGetServiceMapData returns the service map JSON data.
func (s *Server) handleGetServiceMapData(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.phase4 == nil || s.phase4.RepoStore == nil {
		return mcp.NewToolResultError("Repository store not configured. Phase 4 dependencies are required for this tool."), nil
	}

	repos, err := s.phase4.RepoStore.List(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("listing repos: %v", err)), nil
	}

	links, err := s.phase4.RepoStore.GetLinks(ctx, "")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("getting links: %v", err)), nil
	}

	type mapNode struct {
		Name       string `json:"name"`
		Display    string `json:"display_name"`
		Summary    string `json:"summary"`
		FileCount  int    `json:"file_count"`
		SourceType string `json:"source_type"`
		Status     string `json:"status"`
	}
	type mapEdge struct {
		From string `json:"from"`
		To   string `json:"to"`
		Type string `json:"type"`
		Reason string `json:"reason"`
	}
	type mapData struct {
		Nodes []mapNode `json:"nodes"`
		Edges []mapEdge `json:"edges"`
	}

	data := mapData{}
	for _, r := range repos {
		data.Nodes = append(data.Nodes, mapNode{
			Name: r.Name, Display: r.DisplayName, Summary: r.Summary,
			FileCount: r.FileCount, SourceType: r.SourceType, Status: r.Status,
		})
	}
	for _, l := range links {
		data.Edges = append(data.Edges, mapEdge{
			From: l.FromRepo, To: l.ToRepo, Type: l.LinkType, Reason: l.Reason,
		})
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshaling data: %v", err)), nil
	}

	return mcp.NewToolResultText(string(b)), nil
}

