package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"

// AnthropicProvider implements Provider using the Anthropic Messages API via direct HTTP.
type AnthropicProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(apiKey string, model string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{},
	}
}

func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content    []anthropicContent `json:"content"`
	Model      string             `json:"model"`
	StopReason string             `json:"stop_reason"`
	Usage      anthropicUsage     `json:"usage"`
	Error      *anthropicError    `json:"error,omitempty"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (p *AnthropicProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Separate system messages from user/assistant messages.
	var systemPrompt string
	var messages []anthropicMessage
	for _, msg := range req.Messages {
		switch msg.Role {
		case RoleSystem:
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += msg.Content
		case RoleUser:
			messages = append(messages, anthropicMessage{Role: "user", Content: msg.Content})
		case RoleAssistant:
			messages = append(messages, anthropicMessage{Role: "assistant", Content: msg.Content})
		}
	}

	apiReq := anthropicRequest{
		Model:       model,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		System:      systemPrompt,
		Messages:    messages,
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal anthropic request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read anthropic response: %w", err)
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal anthropic response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("anthropic API error (%s): %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var content string
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return &CompletionResponse{
		Content:      content,
		InputTokens:  apiResp.Usage.InputTokens,
		OutputTokens: apiResp.Usage.OutputTokens,
		Model:        apiResp.Model,
		FinishReason: apiResp.StopReason,
	}, nil
}
