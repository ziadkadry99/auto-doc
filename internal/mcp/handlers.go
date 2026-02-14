package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/ziadkadry99/auto-doc/internal/vectordb"
)

// handleSearchCodebase performs semantic search over the codebase vector store.
func (s *Server) handleSearchCodebase(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: query"), nil
	}

	limit := request.GetInt("limit", 10)
	if limit <= 0 {
		limit = 10
	}

	var filter *vectordb.SearchFilter
	if typeStr := request.GetString("type_filter", ""); typeStr != "" {
		docType := vectordb.DocumentType(typeStr)
		filter = &vectordb.SearchFilter{Type: &docType}
	}

	results, err := s.store.Search(ctx, query, limit, filter)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No results found. The codebase may not be indexed yet. Run `autodoc generate` to index it."), nil
	}

	return mcp.NewToolResultText(formatSearchResults(results)), nil
}

// handleGetFileDocs reads and returns the AI-generated documentation for a specific file.
func (s *Server) handleGetFileDocs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: file_path"), nil
	}

	docPath := filepath.Join(s.docsDir, filePath+".md")
	content, err := os.ReadFile(docPath)
	if err != nil {
		if os.IsNotExist(err) {
			return mcp.NewToolResultError(fmt.Sprintf(
				"No documentation found for %q. Run `autodoc generate` to generate documentation.",
				filePath,
			)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to read documentation: %v", err)), nil
	}

	return mcp.NewToolResultText(string(content)), nil
}

// handleGetArchitecture reads and returns the architecture overview document.
func (s *Server) handleGetArchitecture(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	docPath := filepath.Join(s.docsDir, "architecture.md")
	content, err := os.ReadFile(docPath)
	if err != nil {
		if os.IsNotExist(err) {
			return mcp.NewToolResultError(
				"No architecture documentation found. Run `autodoc generate` to generate it.",
			), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to read architecture docs: %v", err)), nil
	}

	return mcp.NewToolResultText(string(content)), nil
}

// handleGetDiagram reads and returns a Mermaid diagram file.
func (s *Server) handleGetDiagram(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	diagramType, err := request.RequireString("diagram_type")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: diagram_type"), nil
	}

	docPath := filepath.Join(s.docsDir, "diagrams", diagramType+".mmd")
	content, err := os.ReadFile(docPath)
	if err != nil {
		if os.IsNotExist(err) {
			return mcp.NewToolResultError(fmt.Sprintf(
				"No %s diagram found. Run `autodoc generate` to generate diagrams.",
				diagramType,
			)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to read diagram: %v", err)), nil
	}

	return mcp.NewToolResultText(string(content)), nil
}

// formatSearchResults converts search results into a rich text format optimized
// for AI agent consumption.
func formatSearchResults(results []vectordb.SearchResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d result(s):\n", len(results)))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("\n--- Result %d ---\n", i+1))

		// Location
		if r.Document.Metadata.FilePath != "" {
			location := r.Document.Metadata.FilePath
			if r.Document.Metadata.LineStart > 0 {
				location += fmt.Sprintf(":%d", r.Document.Metadata.LineStart)
				if r.Document.Metadata.LineEnd > r.Document.Metadata.LineStart {
					location += fmt.Sprintf("-%d", r.Document.Metadata.LineEnd)
				}
			}
			sb.WriteString(fmt.Sprintf("File: %s\n", location))
		}

		// Metadata
		if r.Document.Metadata.Type != "" {
			sb.WriteString(fmt.Sprintf("Type: %s\n", r.Document.Metadata.Type))
		}
		if r.Document.Metadata.Symbol != "" {
			sb.WriteString(fmt.Sprintf("Symbol: %s\n", r.Document.Metadata.Symbol))
		}
		if r.Document.Metadata.Language != "" {
			sb.WriteString(fmt.Sprintf("Language: %s\n", r.Document.Metadata.Language))
		}
		sb.WriteString(fmt.Sprintf("Similarity: %.1f%%\n", r.Similarity*100))

		// Content
		sb.WriteString("\n")
		sb.WriteString(r.Document.Content)
		sb.WriteString("\n")
	}

	return sb.String()
}
