package embeddings

import "context"

// Embedder defines the interface for generating text embeddings.
type Embedder interface {
	// Embed generates embeddings for one or more texts.
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the number of dimensions in the embedding vectors.
	Dimensions() int

	// Name returns the name/identifier of the embedding model.
	Name() string
}
