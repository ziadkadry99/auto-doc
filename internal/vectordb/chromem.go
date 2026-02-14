package vectordb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	chromem "github.com/philippgille/chromem-go"

	"github.com/ziadkadry99/auto-doc/internal/embeddings"
)

const collectionName = "codebase"

// ChromemStore implements VectorStore using chromem-go.
type ChromemStore struct {
	db         *chromem.DB
	collection *chromem.Collection
	embedder   embeddings.Embedder
	embedFunc  chromem.EmbeddingFunc
}

// NewChromemStore creates a new in-memory ChromemStore.
func NewChromemStore(embedder embeddings.Embedder) (*ChromemStore, error) {
	db := chromem.NewDB()
	ef := embeddings.ToChromemFunc(embedder)

	col, err := db.GetOrCreateCollection(collectionName, nil, ef)
	if err != nil {
		return nil, fmt.Errorf("create collection: %w", err)
	}

	return &ChromemStore{
		db:         db,
		collection: col,
		embedder:   embedder,
		embedFunc:  ef,
	}, nil
}

func (s *ChromemStore) AddDocuments(ctx context.Context, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}

	chromDocs := make([]chromem.Document, len(docs))
	for i, doc := range docs {
		chromDocs[i] = chromem.Document{
			ID:       doc.ID,
			Content:  doc.Content,
			Metadata: metadataToMap(doc.Metadata),
		}
	}

	return s.collection.AddDocuments(ctx, chromDocs, 1)
}

func (s *ChromemStore) Search(ctx context.Context, query string, limit int, filter *SearchFilter) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	// chromem-go requires nResults <= collection size.
	if count := s.collection.Count(); limit > count && count > 0 {
		limit = count
	} else if count == 0 {
		return nil, nil
	}

	where := buildWhereClause(filter)

	results, err := s.collection.Query(ctx, query, limit, where, nil)
	if err != nil {
		return nil, fmt.Errorf("chromem query: %w", err)
	}

	searchResults := make([]SearchResult, len(results))
	for i, r := range results {
		searchResults[i] = SearchResult{
			Document: Document{
				ID:       r.ID,
				Content:  r.Content,
				Metadata: mapToMetadata(r.Metadata),
			},
			Similarity: r.Similarity,
		}
	}

	return searchResults, nil
}

func (s *ChromemStore) GetByFilePath(ctx context.Context, filePath string) ([]Document, error) {
	count := s.collection.Count()
	if count == 0 {
		return nil, nil
	}

	where := map[string]string{"file_path": filePath}

	// Use filePath as the query text with count as limit to get all matching documents.
	results, err := s.collection.Query(ctx, filePath, count, where, nil)
	if err != nil {
		return nil, fmt.Errorf("chromem query by file path: %w", err)
	}

	docs := make([]Document, len(results))
	for i, r := range results {
		docs[i] = Document{
			ID:       r.ID,
			Content:  r.Content,
			Metadata: mapToMetadata(r.Metadata),
		}
	}

	return docs, nil
}

func (s *ChromemStore) DeleteByFilePath(ctx context.Context, filePath string) error {
	where := map[string]string{"file_path": filePath}
	return s.collection.Delete(ctx, where, nil)
}

func (s *ChromemStore) Persist(ctx context.Context, dir string) error {
	return s.db.ExportToFile(dir+"/chromem.gob.gz", true, "")
}

func (s *ChromemStore) Load(ctx context.Context, dir string) error {
	err := s.db.ImportFromFile(dir+"/chromem.gob.gz", "")
	if err != nil {
		return fmt.Errorf("import from file: %w", err)
	}

	// Re-acquire collection reference after import.
	col := s.db.GetCollection(collectionName, s.embedFunc)
	if col == nil {
		return fmt.Errorf("collection %q not found after import", collectionName)
	}
	s.collection = col
	return nil
}

func (s *ChromemStore) Count() int {
	return s.collection.Count()
}

// metadataToMap converts DocumentMetadata to a flat map[string]string for chromem.
func metadataToMap(m DocumentMetadata) map[string]string {
	md := map[string]string{
		"file_path":    m.FilePath,
		"line_start":   strconv.Itoa(m.LineStart),
		"line_end":     strconv.Itoa(m.LineEnd),
		"content_hash": m.ContentHash,
		"type":         string(m.Type),
		"language":     m.Language,
		"symbol":       m.Symbol,
		"repo_id":      m.RepoID,
		"last_updated": m.LastUpdated.Format(time.RFC3339),
	}
	return md
}

// mapToMetadata converts a flat map[string]string back to DocumentMetadata.
func mapToMetadata(m map[string]string) DocumentMetadata {
	lineStart, _ := strconv.Atoi(m["line_start"])
	lineEnd, _ := strconv.Atoi(m["line_end"])
	lastUpdated, _ := time.Parse(time.RFC3339, m["last_updated"])

	return DocumentMetadata{
		FilePath:    m["file_path"],
		LineStart:   lineStart,
		LineEnd:     lineEnd,
		ContentHash: m["content_hash"],
		Type:        DocumentType(m["type"]),
		Language:    m["language"],
		Symbol:      m["symbol"],
		RepoID:      m["repo_id"],
		LastUpdated: lastUpdated,
	}
}

// buildWhereClause converts a SearchFilter to a chromem where clause.
func buildWhereClause(filter *SearchFilter) map[string]string {
	if filter == nil {
		return nil
	}

	where := make(map[string]string)
	if filter.Type != nil {
		where["type"] = string(*filter.Type)
	}
	if filter.FilePath != nil {
		where["file_path"] = *filter.FilePath
	}
	if filter.Language != nil {
		where["language"] = *filter.Language
	}

	if len(where) == 0 {
		return nil
	}
	return where
}
