package mcp

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/ziadkadry99/auto-doc/internal/embeddings"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
)

// Version is set via ldflags at build time.
var Version = "dev"

// Server wraps an MCP server that exposes codebase search tools.
type Server struct {
	store    vectordb.VectorStore
	embedder embeddings.Embedder
	docsDir  string
	mcp      *server.MCPServer
	phase4   *Phase4Deps
}

// NewServer creates a new MCP server with the given dependencies.
func NewServer(store vectordb.VectorStore, embedder embeddings.Embedder, docsDir string) *Server {
	s := &Server{
		store:    store,
		embedder: embedder,
		docsDir:  docsDir,
	}

	s.mcp = server.NewMCPServer(
		"autodoc",
		Version,
		server.WithToolCapabilities(false),
	)

	s.registerTools()

	return s
}

// registerTools adds all tool definitions and their handlers to the MCP server.
func (s *Server) registerTools() {
	s.mcp.AddTool(searchCodebaseTool, s.handleSearchCodebase)
	s.mcp.AddTool(getFileDocsTool, s.handleGetFileDocs)
	s.mcp.AddTool(getArchitectureTool, s.handleGetArchitecture)
	s.mcp.AddTool(getDiagramTool, s.handleGetDiagram)
}

// Serve starts the MCP server on stdio. Stdout is used for MCP protocol
// messages; all logging must go to stderr.
func (s *Server) Serve() error {
	return server.ServeStdio(s.mcp)
}
