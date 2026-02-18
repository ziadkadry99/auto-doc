package llm

import (
	"context"

	openai "github.com/sashabaranov/go-openai"
)

// MinimaxProvider implements Provider using the MiniMax API (OpenAI-compatible).
type MinimaxProvider struct {
	client *openai.Client
	model  string
}

// NewMinimaxProvider creates a new MiniMax provider.
func NewMinimaxProvider(apiKey string, model string) *MinimaxProvider {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = "https://api.minimax.io/v1"
	client := openai.NewClientWithConfig(cfg)
	return &MinimaxProvider{
		client: client,
		model:  model,
	}
}

func (p *MinimaxProvider) Name() string {
	return "minimax"
}

func (p *MinimaxProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// MiniMax requires temperature in (0.0, 1.0].
	temp := req.Temperature
	if temp <= 0 {
		temp = 0.01
	} else if temp > 1.0 {
		temp = 1.0
	}

	var messages []openai.ChatCompletionMessage
	for _, msg := range req.Messages {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	apiReq := openai.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: float32(temp),
	}

	if req.JSONMode {
		apiReq.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	resp, err := p.client.CreateChatCompletion(ctx, apiReq)
	if err != nil {
		return nil, err
	}

	var content string
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	var finishReason string
	if len(resp.Choices) > 0 {
		finishReason = string(resp.Choices[0].FinishReason)
	}

	return &CompletionResponse{
		Content:      content,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
		Model:        resp.Model,
		FinishReason: finishReason,
	}, nil
}
