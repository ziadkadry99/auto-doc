package mcp

import "github.com/mark3labs/mcp-go/mcp"

// searchAcrossReposTool searches documentation across all indexed repositories.
var searchAcrossReposTool = mcp.NewTool("search_across_repos",
	mcp.WithDescription("Search documentation semantically across all indexed repositories. Returns relevant files, functions, and context from any repo."),
	mcp.WithString("query",
		mcp.Required(),
		mcp.Description("Natural language search query"),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of results to return (default 10)"),
	),
)

// getServiceContextTool retrieves complete context for a named service.
var getServiceContextTool = mcp.NewTool("get_service_context",
	mcp.WithDescription("Get complete context for a service including known facts, ownership, related documentation, and architecture information."),
	mcp.WithString("service",
		mcp.Required(),
		mcp.Description("Name of the service to get context for"),
	),
)

// getBlastRadiusTool shows services affected if a service or endpoint changes.
var getBlastRadiusTool = mcp.NewTool("get_blast_radius",
	mcp.WithDescription("Determine which services would be affected if a given service or endpoint changes. Searches for references and dependencies across all documentation."),
	mcp.WithString("service",
		mcp.Required(),
		mcp.Description("Name of the service that is changing"),
	),
	mcp.WithString("endpoint",
		mcp.Description("Specific endpoint that is changing (optional, narrows the search)"),
	),
)

// getFlowTool retrieves a named data flow.
var getFlowTool = mcp.NewTool("get_flow",
	mcp.WithDescription("Get a named cross-service data flow including its narrative, diagram, and the services involved."),
	mcp.WithString("flow_name",
		mcp.Required(),
		mcp.Description("Name of the flow to retrieve (searches by name)"),
	),
)

// askArchitectureTool answers free-form architecture questions using known facts.
var askArchitectureTool = mcp.NewTool("ask_architecture",
	mcp.WithDescription("Ask a free-form question about the architecture. Uses all known facts and context to provide an informed answer."),
	mcp.WithString("question",
		mcp.Required(),
		mcp.Description("The architecture question to answer"),
	),
)

// getTeamServicesTool lists services owned by a team.
var getTeamServicesTool = mcp.NewTool("get_team_services",
	mcp.WithDescription("Get all services/repositories owned by a specific team, including ownership confidence and source."),
	mcp.WithString("team",
		mcp.Required(),
		mcp.Description("Team name or ID to look up"),
	),
)

// provideContextTool allows AI assistants to feed back context about a service.
var provideContextTool = mcp.NewTool("provide_context",
	mcp.WithDescription("Provide additional context or knowledge about a service. This information is saved as a fact and used to improve future documentation and answers."),
	mcp.WithString("service",
		mcp.Required(),
		mcp.Description("Name of the service this context is about"),
	),
	mcp.WithString("context",
		mcp.Required(),
		mcp.Description("The context or knowledge to save"),
	),
)
