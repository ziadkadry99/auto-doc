package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultOllamaBaseURL = "http://localhost:11434"

// OllamaEmbedder generates embeddings using a local Ollama instance.
type OllamaEmbedder struct {
	baseURL    string
	model      string
	dimensions int
	httpClient *http.Client
}

// NewOllamaEmbedder creates a new Ollama embedder.
// model is the Ollama model name (e.g. "nomic-embed-text").
// dimensions is the output dimension count for the model.
// baseURL defaults to http://localhost:11434 if empty.
func NewOllamaEmbedder(model string, dimensions int, baseURL string) *OllamaEmbedder {
	if baseURL == "" {
		baseURL = defaultOllamaBaseURL
	}
	return &OllamaEmbedder{
		baseURL:    baseURL,
		model:      model,
		dimensions: dimensions,
		httpClient: &http.Client{},
	}
}

func (e *OllamaEmbedder) Name() string {
	return "ollama/" + e.model
}

func (e *OllamaEmbedder) Dimensions() int {
	return e.dimensions
}

type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func (e *OllamaEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	results := make([][]float32, 0, len(texts))
	for _, text := range texts {
		emb, err := e.embedSingle(ctx, text)
		if err != nil {
			return nil, err
		}
		results = append(results, emb)
	}
	return results, nil
}

func (e *OllamaEmbedder) embedSingle(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(ollamaEmbedRequest{
		Model: e.model,
		Input: text,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}

	if len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("ollama returned no embeddings")
	}

	return result.Embeddings[0], nil
}
