package mcp

import "github.com/mark3labs/mcp-go/mcp"

// searchCodebaseTool defines the search_codebase MCP tool.
var searchCodebaseTool = mcp.NewTool("search_codebase",
	mcp.WithDescription("Search the codebase documentation semantically. Returns relevant files, functions, and context."),
	mcp.WithString("query",
		mcp.Required(),
		mcp.Description("Natural language search query"),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of results to return (default 10)"),
	),
	mcp.WithString("type_filter",
		mcp.Description("Filter results by document type"),
		mcp.Enum("file", "function", "class", "module", "architecture"),
	),
)

// getFileDocsTool defines the get_file_docs MCP tool.
var getFileDocsTool = mcp.NewTool("get_file_docs",
	mcp.WithDescription("Get complete AI-generated documentation for a specific file."),
	mcp.WithString("file_path",
		mcp.Required(),
		mcp.Description("Path to the file relative to the project root"),
	),
)

// getArchitectureTool defines the get_architecture MCP tool.
var getArchitectureTool = mcp.NewTool("get_architecture",
	mcp.WithDescription("Get the high-level architecture overview including component descriptions, data flow, and design patterns."),
)

// getDiagramTool defines the get_diagram MCP tool.
var getDiagramTool = mcp.NewTool("get_diagram",
	mcp.WithDescription("Get a Mermaid diagram of the codebase."),
	mcp.WithString("diagram_type",
		mcp.Required(),
		mcp.Description("Type of diagram to retrieve"),
		mcp.Enum("architecture", "dependency", "sequence"),
	),
)
