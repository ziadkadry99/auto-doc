package embeddings

import (
	"context"

	chromem "github.com/philippgille/chromem-go"
)

// ToChromemFunc converts an Embedder into a chromem.EmbeddingFunc.
// chromem-go expects a function that embeds a single text at a time.
func ToChromemFunc(e Embedder) chromem.EmbeddingFunc {
	return func(ctx context.Context, text string) ([]float32, error) {
		results, err := e.Embed(ctx, []string{text})
		if err != nil {
			return nil, err
		}
		if len(results) == 0 {
			return nil, nil
		}
		return results[0], nil
	}
}
