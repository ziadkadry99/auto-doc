package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/ziadkadry99/auto-doc/internal/vectordb"
)

// mockEmbedder implements embeddings.Embedder for testing.
type mockEmbedder struct{}

func (m *mockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = make([]float32, 3)
	}
	return result, nil
}
func (m *mockEmbedder) Dimensions() int { return 3 }
func (m *mockEmbedder) Name() string    { return "mock" }

// mockStore implements vectordb.VectorStore for testing.
type mockStore struct {
	docs []vectordb.Document
}

func (m *mockStore) AddDocuments(_ context.Context, docs []vectordb.Document) error {
	m.docs = append(m.docs, docs...)
	return nil
}

func (m *mockStore) Search(_ context.Context, query string, limit int, filter *vectordb.SearchFilter) ([]vectordb.SearchResult, error) {
	var results []vectordb.SearchResult
	for _, doc := range m.docs {
		if filter != nil && filter.Type != nil && doc.Metadata.Type != *filter.Type {
			continue
		}
		results = append(results, vectordb.SearchResult{
			Document:   doc,
			Similarity: 0.95,
		})
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

func (m *mockStore) GetByFilePath(_ context.Context, filePath string) ([]vectordb.Document, error) {
	var results []vectordb.Document
	for _, doc := range m.docs {
		if doc.Metadata.FilePath == filePath {
			results = append(results, doc)
		}
	}
	return results, nil
}

func (m *mockStore) DeleteByFilePath(_ context.Context, _ string) error { return nil }
func (m *mockStore) Persist(_ context.Context, _ string) error          { return nil }
func (m *mockStore) Load(_ context.Context, _ string) error             { return nil }
func (m *mockStore) Count() int                                         { return len(m.docs) }

func TestToolDefinitions(t *testing.T) {
	// Verify tool names and required properties.
	tests := []struct {
		name     string
		tool     mcp.Tool
		wantName string
	}{
		{"search_codebase", searchCodebaseTool, "search_codebase"},
		{"get_file_docs", getFileDocsTool, "get_file_docs"},
		{"get_architecture", getArchitectureTool, "get_architecture"},
		{"get_diagram", getDiagramTool, "get_diagram"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tool.Name != tt.wantName {
				t.Errorf("tool name = %q, want %q", tt.tool.Name, tt.wantName)
			}
			if tt.tool.Description == "" {
				t.Error("tool description should not be empty")
			}
		})
	}
}

func TestNewServer(t *testing.T) {
	store := &mockStore{}
	embedder := &mockEmbedder{}
	srv := NewServer(store, embedder, "/tmp/docs")

	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	if srv.mcp == nil {
		t.Fatal("MCP server not initialized")
	}
	if srv.store != store {
		t.Error("store not set correctly")
	}
	if srv.docsDir != "/tmp/docs" {
		t.Errorf("docsDir = %q, want %q", srv.docsDir, "/tmp/docs")
	}
}

func TestHandleSearchCodebase(t *testing.T) {
	fileType := vectordb.DocTypeFile
	store := &mockStore{
		docs: []vectordb.Document{
			{
				ID:      "1",
				Content: "Package main provides the entry point.",
				Metadata: vectordb.DocumentMetadata{
					FilePath:  "main.go",
					LineStart: 1,
					LineEnd:   10,
					Type:      vectordb.DocTypeFile,
					Language:  "go",
				},
			},
			{
				ID:      "2",
				Content: "func handleRequest processes HTTP requests.",
				Metadata: vectordb.DocumentMetadata{
					FilePath:  "handler.go",
					LineStart: 15,
					LineEnd:   30,
					Type:      vectordb.DocTypeFunction,
					Symbol:    "handleRequest",
					Language:  "go",
				},
			},
		},
	}
	srv := NewServer(store, &mockEmbedder{}, "/tmp/docs")
	ctx := context.Background()

	t.Run("basic search", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"query": "entry point",
		}

		result, err := srv.handleSearchCodebase(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}
	})

	t.Run("search with type filter", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"query":       "handle",
			"type_filter": string(fileType),
		}

		result, err := srv.handleSearchCodebase(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}
	})

	t.Run("missing query", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{}

		result, err := srv.handleSearchCodebase(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for missing query")
		}
	})

	t.Run("empty store", func(t *testing.T) {
		emptySrv := NewServer(&mockStore{}, &mockEmbedder{}, "/tmp/docs")
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"query": "anything",
		}

		result, err := emptySrv.handleSearchCodebase(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Error("empty results should not be an error")
		}
	})
}

func TestHandleGetFileDocs(t *testing.T) {
	docsDir := t.TempDir()

	// Create a test doc file.
	docContent := "# main.go\n\nThis is the main entry point.\n"
	docDir := filepath.Join(docsDir, "src")
	if err := os.MkdirAll(docDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docDir, "main.go.md"), []byte(docContent), 0644); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(&mockStore{}, &mockEmbedder{}, docsDir)
	ctx := context.Background()

	t.Run("existing file", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"file_path": "src/main.go",
		}

		result, err := srv.handleGetFileDocs(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"file_path": "nonexistent.go",
		}

		result, err := srv.handleGetFileDocs(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for missing file")
		}
	})
}

func TestHandleGetArchitecture(t *testing.T) {
	docsDir := t.TempDir()
	ctx := context.Background()

	t.Run("missing architecture doc", func(t *testing.T) {
		srv := NewServer(&mockStore{}, &mockEmbedder{}, docsDir)
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{}

		result, err := srv.handleGetArchitecture(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for missing architecture doc")
		}
	})

	t.Run("existing architecture doc", func(t *testing.T) {
		content := "# Architecture\n\nOverview of the system.\n"
		if err := os.WriteFile(filepath.Join(docsDir, "architecture.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		srv := NewServer(&mockStore{}, &mockEmbedder{}, docsDir)
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{}

		result, err := srv.handleGetArchitecture(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}
	})
}

func TestHandleGetDiagram(t *testing.T) {
	docsDir := t.TempDir()
	ctx := context.Background()

	// Create diagrams directory with a test diagram.
	diagramDir := filepath.Join(docsDir, "diagrams")
	if err := os.MkdirAll(diagramDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(diagramDir, "architecture.mmd"), []byte("graph TD\n  A-->B\n"), 0644); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(&mockStore{}, &mockEmbedder{}, docsDir)

	t.Run("existing diagram", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"diagram_type": "architecture",
		}

		result, err := srv.handleGetDiagram(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}
	})

	t.Run("missing diagram", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"diagram_type": "sequence",
		}

		result, err := srv.handleGetDiagram(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for missing diagram")
		}
	})

	t.Run("missing diagram_type param", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{}

		result, err := srv.handleGetDiagram(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for missing diagram_type")
		}
	})
}

func TestFormatSearchResults(t *testing.T) {
	t.Run("empty results", func(t *testing.T) {
		result := formatSearchResults(nil)
		if result != "Found 0 result(s):\n" {
			t.Errorf("unexpected output for empty results: %q", result)
		}
	})

	t.Run("single result", func(t *testing.T) {
		results := []vectordb.SearchResult{
			{
				Document: vectordb.Document{
					ID:      "1",
					Content: "Main entry point",
					Metadata: vectordb.DocumentMetadata{
						FilePath:  "main.go",
						LineStart: 1,
						LineEnd:   10,
						Type:      vectordb.DocTypeFile,
						Language:  "go",
					},
				},
				Similarity: 0.9523,
			},
		}
		result := formatSearchResults(results)
		if result == "" {
			t.Error("expected non-empty result")
		}
		// Should contain key information.
		for _, want := range []string{"main.go:1-10", "file", "go", "95.2%", "Main entry point"} {
			if !contains(result, want) {
				t.Errorf("result missing %q:\n%s", want, result)
			}
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
