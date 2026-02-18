package vectordb

import "context"

// VectorStore defines the interface for storing and searching documents by embeddings.
type VectorStore interface {
	// AddDocuments adds or updates documents in the store.
	AddDocuments(ctx context.Context, docs []Document) error

	// Search performs a semantic search using the query text.
	Search(ctx context.Context, query string, limit int, filter *SearchFilter) ([]SearchResult, error)

	// GetByFilePath retrieves all documents associated with the given file path.
	GetByFilePath(ctx context.Context, filePath string) ([]Document, error)

	// DeleteByFilePath removes all documents associated with the given file path.
	DeleteByFilePath(ctx context.Context, filePath string) error

	// DeleteByRepoID removes all documents associated with the given repository ID.
	DeleteByRepoID(ctx context.Context, repoID string) error

	// Persist saves the store's data to the given directory.
	Persist(ctx context.Context, dir string) error

	// Load restores the store's data from the given directory.
	Load(ctx context.Context, dir string) error

	// Count returns the total number of documents in the store.
	Count() int
}
