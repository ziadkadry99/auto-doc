package vectordb

import (
	"context"
	"math"
	"os"
	"testing"
	"time"
)

// mockEmbedder returns deterministic embeddings based on text content.
// It produces a simple hash-based vector for reproducible tests.
type mockEmbedder struct {
	dims int
}

func newMockEmbedder(dims int) *mockEmbedder {
	return &mockEmbedder{dims: dims}
}

func (m *mockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		results[i] = m.deterministicVector(text)
	}
	return results, nil
}

func (m *mockEmbedder) Dimensions() int { return m.dims }
func (m *mockEmbedder) Name() string    { return "mock" }

// deterministicVector produces a normalized vector from text.
// Similar texts will produce similar vectors because shared characters contribute
// to the same positions in the vector.
func (m *mockEmbedder) deterministicVector(text string) []float32 {
	vec := make([]float32, m.dims)
	for i, ch := range text {
		idx := (int(ch) + i) % m.dims
		vec[idx] += 1.0
	}
	// Normalize
	var norm float64
	for _, v := range vec {
		norm += float64(v * v)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] = float32(float64(vec[i]) / norm)
		}
	}
	return vec
}

func TestChromemStore_AddAndSearch(t *testing.T) {
	ctx := context.Background()
	embedder := newMockEmbedder(64)

	store, err := NewChromemStore(embedder)
	if err != nil {
		t.Fatalf("NewChromemStore: %v", err)
	}

	docs := []Document{
		{
			ID:      "doc1",
			Content: "The authentication module handles user login and session management",
			Metadata: DocumentMetadata{
				FilePath:    "internal/auth/login.go",
				LineStart:   1,
				LineEnd:     50,
				ContentHash: "abc123",
				Type:        DocTypeFunction,
				Language:    "go",
				Symbol:      "HandleLogin",
				LastUpdated: time.Now(),
			},
		},
		{
			ID:      "doc2",
			Content: "Database connection pool configuration and initialization",
			Metadata: DocumentMetadata{
				FilePath:    "internal/db/pool.go",
				LineStart:   1,
				LineEnd:     30,
				ContentHash: "def456",
				Type:        DocTypeFile,
				Language:    "go",
				Symbol:      "",
				LastUpdated: time.Now(),
			},
		},
		{
			ID:      "doc3",
			Content: "HTTP router setup and middleware chain for the REST API",
			Metadata: DocumentMetadata{
				FilePath:    "internal/api/router.go",
				LineStart:   10,
				LineEnd:     80,
				ContentHash: "ghi789",
				Type:        DocTypeModule,
				Language:    "go",
				Symbol:      "SetupRouter",
				LastUpdated: time.Now(),
			},
		},
	}

	if err := store.AddDocuments(ctx, docs); err != nil {
		t.Fatalf("AddDocuments: %v", err)
	}

	if count := store.Count(); count != 3 {
		t.Errorf("Count: got %d, want 3", count)
	}

	// Search for auth-related content
	results, err := store.Search(ctx, "user authentication login", 2, nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search returned no results")
	}
	if len(results) > 2 {
		t.Errorf("Search returned %d results, expected at most 2", len(results))
	}

	// Verify results have similarity scores
	for _, r := range results {
		if r.Similarity == 0 {
			t.Error("result has zero similarity")
		}
	}
}

func TestChromemStore_SearchWithFilter(t *testing.T) {
	ctx := context.Background()
	embedder := newMockEmbedder(64)

	store, err := NewChromemStore(embedder)
	if err != nil {
		t.Fatalf("NewChromemStore: %v", err)
	}

	docs := []Document{
		{
			ID:      "f1",
			Content: "Go function that processes data",
			Metadata: DocumentMetadata{
				FilePath: "main.go",
				Type:     DocTypeFunction,
				Language: "go",
			},
		},
		{
			ID:      "f2",
			Content: "Python function that processes data",
			Metadata: DocumentMetadata{
				FilePath: "main.py",
				Type:     DocTypeFunction,
				Language: "python",
			},
		},
	}

	if err := store.AddDocuments(ctx, docs); err != nil {
		t.Fatalf("AddDocuments: %v", err)
	}

	// Filter by language
	lang := "python"
	results, err := store.Search(ctx, "process data", 10, &SearchFilter{Language: &lang})
	if err != nil {
		t.Fatalf("Search with filter: %v", err)
	}

	for _, r := range results {
		if r.Document.Metadata.Language != "python" {
			t.Errorf("expected language python, got %s", r.Document.Metadata.Language)
		}
	}
}

func TestChromemStore_DeleteByFilePath(t *testing.T) {
	ctx := context.Background()
	embedder := newMockEmbedder(64)

	store, err := NewChromemStore(embedder)
	if err != nil {
		t.Fatalf("NewChromemStore: %v", err)
	}

	docs := []Document{
		{
			ID:      "d1",
			Content: "first document content",
			Metadata: DocumentMetadata{
				FilePath: "file_a.go",
				Type:     DocTypeFile,
				Language: "go",
			},
		},
		{
			ID:      "d2",
			Content: "second document content",
			Metadata: DocumentMetadata{
				FilePath: "file_b.go",
				Type:     DocTypeFile,
				Language: "go",
			},
		},
	}

	if err := store.AddDocuments(ctx, docs); err != nil {
		t.Fatalf("AddDocuments: %v", err)
	}

	if count := store.Count(); count != 2 {
		t.Fatalf("Count before delete: got %d, want 2", count)
	}

	if err := store.DeleteByFilePath(ctx, "file_a.go"); err != nil {
		t.Fatalf("DeleteByFilePath: %v", err)
	}

	if count := store.Count(); count != 1 {
		t.Errorf("Count after delete: got %d, want 1", count)
	}
}

func TestChromemStore_PersistAndLoad(t *testing.T) {
	ctx := context.Background()
	embedder := newMockEmbedder(64)

	store, err := NewChromemStore(embedder)
	if err != nil {
		t.Fatalf("NewChromemStore: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	docs := []Document{
		{
			ID:      "persist1",
			Content: "persistent document about authentication",
			Metadata: DocumentMetadata{
				FilePath:    "auth.go",
				LineStart:   5,
				LineEnd:     25,
				ContentHash: "hash1",
				Type:        DocTypeFunction,
				Language:    "go",
				Symbol:      "Authenticate",
				RepoID:      "repo-123",
				LastUpdated: now,
			},
		},
		{
			ID:      "persist2",
			Content: "persistent document about database queries",
			Metadata: DocumentMetadata{
				FilePath:    "db.go",
				LineStart:   10,
				LineEnd:     40,
				ContentHash: "hash2",
				Type:        DocTypeFile,
				Language:    "go",
				Symbol:      "",
				RepoID:      "repo-123",
				LastUpdated: now,
			},
		},
	}

	if err := store.AddDocuments(ctx, docs); err != nil {
		t.Fatalf("AddDocuments: %v", err)
	}

	// Persist to temp dir
	tmpDir, err := os.MkdirTemp("", "chromem-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := store.Persist(ctx, tmpDir); err != nil {
		t.Fatalf("Persist: %v", err)
	}

	// Create new store and load
	store2, err := NewChromemStore(embedder)
	if err != nil {
		t.Fatalf("NewChromemStore for load: %v", err)
	}

	if err := store2.Load(ctx, tmpDir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if count := store2.Count(); count != 2 {
		t.Errorf("Count after load: got %d, want 2", count)
	}

	// Search in loaded store - verify documents are retrievable and metadata preserved
	results, err := store2.Search(ctx, "authentication database", 2, nil)
	if err != nil {
		t.Fatalf("Search after load: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Search after load returned %d results, want 2", len(results))
	}

	// Check that both documents are present with correct metadata
	foundAuth, foundDB := false, false
	for _, r := range results {
		switch r.Document.Metadata.FilePath {
		case "auth.go":
			foundAuth = true
			if r.Document.Metadata.RepoID != "repo-123" {
				t.Errorf("auth.go: expected repo_id repo-123, got %s", r.Document.Metadata.RepoID)
			}
			if r.Document.Metadata.Type != DocTypeFunction {
				t.Errorf("auth.go: expected type function, got %s", r.Document.Metadata.Type)
			}
			if r.Document.Metadata.Symbol != "Authenticate" {
				t.Errorf("auth.go: expected symbol Authenticate, got %s", r.Document.Metadata.Symbol)
			}
		case "db.go":
			foundDB = true
			if r.Document.Metadata.LineStart != 10 {
				t.Errorf("db.go: expected line_start 10, got %d", r.Document.Metadata.LineStart)
			}
		}
	}
	if !foundAuth {
		t.Error("auth.go document not found after load")
	}
	if !foundDB {
		t.Error("db.go document not found after load")
	}
}

func TestFormatResults(t *testing.T) {
	results := []SearchResult{
		{
			Document: Document{
				ID:      "r1",
				Content: "func main() { ... }",
				Metadata: DocumentMetadata{
					FilePath:  "main.go",
					LineStart: 10,
					LineEnd:   20,
					Type:      DocTypeFunction,
					Symbol:    "main",
					Language:  "go",
				},
			},
			Similarity: 0.9512,
		},
	}

	output := FormatResults(results)
	if output == "" {
		t.Error("FormatResults returned empty string")
	}
	if !contains(output, "main.go:10-20") {
		t.Errorf("expected file location in output, got: %s", output)
	}
	if !contains(output, "0.9512") {
		t.Errorf("expected similarity score in output, got: %s", output)
	}
}

func TestFormatResults_Empty(t *testing.T) {
	output := FormatResults(nil)
	if output != "No results found." {
		t.Errorf("expected 'No results found.', got: %s", output)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
