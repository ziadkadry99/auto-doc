package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OllamaProvider implements Provider using direct HTTP calls to the Ollama API.
type OllamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(baseURL string, model string) *OllamaProvider {
	return &OllamaProvider{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
	}
}

func (p *OllamaProvider) Name() string {
	return "ollama"
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  ollamaOptions   `json:"options,omitempty"`
	Format   string          `json:"format,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaChatResponse struct {
	Message            ollamaMessage `json:"message"`
	Model              string        `json:"model"`
	Done               bool          `json:"done"`
	DoneReason         string        `json:"done_reason"`
	PromptEvalCount    int           `json:"prompt_eval_count"`
	EvalCount          int           `json:"eval_count"`
}

func (p *OllamaProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	var messages []ollamaMessage
	for _, msg := range req.Messages {
		messages = append(messages, ollamaMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	ollamaReq := ollamaChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
		Options: ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		},
	}

	if req.JSONMode {
		ollamaReq.Format = "json"
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ollama request: %w", err)
	}

	url := fmt.Sprintf("%s/api/chat", p.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ollama response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaChatResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ollama response: %w", err)
	}

	return &CompletionResponse{
		Content:      ollamaResp.Message.Content,
		InputTokens:  ollamaResp.PromptEvalCount,
		OutputTokens: ollamaResp.EvalCount,
		Model:        ollamaResp.Model,
		FinishReason: ollamaResp.DoneReason,
	}, nil
}
