package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const googleEmbedEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/%s:embedContent?key=%s"

// GoogleModel represents a supported Google embedding model.
type GoogleModel string

const (
	ModelGeminiEmbedding001 GoogleModel = "gemini-embedding-001"
)

func (m GoogleModel) dimensions() int {
	switch m {
	case ModelGeminiEmbedding001:
		return 3072
	default:
		return 3072
	}
}

// GoogleEmbedder generates embeddings using Google's Generative AI API.
type GoogleEmbedder struct {
	apiKey     string
	model      GoogleModel
	httpClient *http.Client
}

// NewGoogleEmbedder creates a new Google embedder.
func NewGoogleEmbedder(apiKey string, model GoogleModel) *GoogleEmbedder {
	return &GoogleEmbedder{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{},
	}
}

func (e *GoogleEmbedder) Name() string {
	return string(e.model)
}

func (e *GoogleEmbedder) Dimensions() int {
	return e.model.dimensions()
}

type googleEmbedRequest struct {
	Content googleContent `json:"content"`
}

type googleContent struct {
	Parts []googlePart `json:"parts"`
}

type googlePart struct {
	Text string `json:"text"`
}

type googleEmbedResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}

func (e *GoogleEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
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

func (e *GoogleEmbedder) embedSingle(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(googleEmbedRequest{
		Content: googleContent{
			Parts: []googlePart{{Text: text}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal google embed request: %w", err)
	}

	url := fmt.Sprintf(googleEmbedEndpoint, e.model, e.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create google embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google embed request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google embed API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result googleEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode google embed response: %w", err)
	}

	if len(result.Embedding.Values) == 0 {
		return nil, fmt.Errorf("google returned empty embedding")
	}

	return result.Embedding.Values, nil
}
