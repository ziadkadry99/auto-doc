package embeddings

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

const maxBatchSize = 100

// OpenAIModel represents a supported OpenAI embedding model.
type OpenAIModel string

const (
	ModelTextEmbedding3Small OpenAIModel = "text-embedding-3-small"
	ModelTextEmbedding3Large OpenAIModel = "text-embedding-3-large"
)

func (m OpenAIModel) dimensions() int {
	switch m {
	case ModelTextEmbedding3Small:
		return 1536
	case ModelTextEmbedding3Large:
		return 3072
	default:
		return 1536
	}
}

// OpenAIEmbedder generates embeddings using OpenAI's API.
type OpenAIEmbedder struct {
	client *openai.Client
	model  OpenAIModel
}

// NewOpenAIEmbedder creates a new OpenAI embedder with the given API key and model.
func NewOpenAIEmbedder(apiKey string, model OpenAIModel) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		client: openai.NewClient(apiKey),
		model:  model,
	}
}

func (e *OpenAIEmbedder) Name() string {
	return string(e.model)
}

func (e *OpenAIEmbedder) Dimensions() int {
	return e.model.dimensions()
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	allEmbeddings := make([][]float32, 0, len(texts))

	// Batch up to maxBatchSize texts per API call
	for i := 0; i < len(texts); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		resp, err := e.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
			Input: batch,
			Model: openai.EmbeddingModel(e.model),
		})
		if err != nil {
			return nil, fmt.Errorf("openai embedding request failed: %w", err)
		}

		if len(resp.Data) != len(batch) {
			return nil, fmt.Errorf("openai returned %d embeddings, expected %d", len(resp.Data), len(batch))
		}

		for _, emb := range resp.Data {
			allEmbeddings = append(allEmbeddings, emb.Embedding)
		}
	}

	return allEmbeddings, nil
}
