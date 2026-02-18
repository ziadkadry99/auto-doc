package llm

import (
	"context"

	openai "github.com/sashabaranov/go-openai"
)

// OpenRouterProvider implements Provider using the OpenRouter API (OpenAI-compatible).
type OpenRouterProvider struct {
	client *openai.Client
	model  string
}

// NewOpenRouterProvider creates a new OpenRouter provider.
func NewOpenRouterProvider(apiKey string, model string) *OpenRouterProvider {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = "https://openrouter.ai/api/v1"
	client := openai.NewClientWithConfig(cfg)
	return &OpenRouterProvider{
		client: client,
		model:  model,
	}
}

func (p *OpenRouterProvider) Name() string {
	return "openrouter"
}

func (p *OpenRouterProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
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
		Temperature: float32(req.Temperature),
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
