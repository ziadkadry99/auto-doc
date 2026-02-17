package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
)

const googleAPIBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

// GoogleProvider implements Provider using the Google Gemini API via direct HTTP.
type GoogleProvider struct {
	apiKey      string
	tokenSource oauth2.TokenSource
	model       string
	client      *http.Client
}

// NewGoogleProvider creates a new Google Gemini provider using an API key.
func NewGoogleProvider(apiKey string, model string) *GoogleProvider {
	return &GoogleProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{},
	}
}

// NewGoogleProviderWithTokenSource creates a Google provider using OAuth2 Bearer tokens.
func NewGoogleProviderWithTokenSource(ts oauth2.TokenSource, model string) *GoogleProvider {
	return &GoogleProvider{
		tokenSource: ts,
		model:       model,
		client:      &http.Client{},
	}
}

func (p *GoogleProvider) Name() string {
	return "google"
}

type geminiRequest struct {
	Contents          []geminiContent         `json:"contents"`
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	MaxOutputTokens  int     `json:"maxOutputTokens,omitempty"`
	Temperature      float64 `json:"temperature"`
	ResponseMIMEType string  `json:"responseMimeType,omitempty"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate    `json:"candidates"`
	UsageMetadata *geminiUsageMetadata `json:"usageMetadata"`
	Error         *geminiError         `json:"error,omitempty"`
}

type geminiCandidate struct {
	Content      *geminiContent `json:"content"`
	FinishReason string         `json:"finishReason"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type geminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

func (p *GoogleProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	// Build system instruction and conversation contents.
	var systemParts []geminiPart
	var contents []geminiContent

	for _, msg := range req.Messages {
		switch msg.Role {
		case RoleSystem:
			systemParts = append(systemParts, geminiPart{Text: msg.Content})
		case RoleUser:
			contents = append(contents, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: msg.Content}},
			})
		case RoleAssistant:
			contents = append(contents, geminiContent{
				Role:  "model",
				Parts: []geminiPart{{Text: msg.Content}},
			})
		}
	}

	// Ensure there's at least one content entry.
	if len(contents) == 0 {
		contents = append(contents, geminiContent{
			Role:  "user",
			Parts: []geminiPart{{Text: ""}},
		})
	}

	apiReq := geminiRequest{
		Contents: contents,
		GenerationConfig: &geminiGenerationConfig{
			Temperature: req.Temperature,
		},
	}

	if len(systemParts) > 0 {
		apiReq.SystemInstruction = &geminiContent{
			Parts: systemParts,
		}
	}

	if req.MaxTokens > 0 {
		apiReq.GenerationConfig.MaxOutputTokens = req.MaxTokens
	}

	if req.JSONMode {
		apiReq.GenerationConfig.ResponseMIMEType = "application/json"
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gemini request: %w", err)
	}

	var url string
	if p.tokenSource != nil {
		url = fmt.Sprintf("%s/%s:generateContent", googleAPIBaseURL, model)
	} else {
		url = fmt.Sprintf("%s/%s:generateContent?key=%s", googleAPIBaseURL, model, p.apiKey)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if p.tokenSource != nil {
		token, err := p.tokenSource.Token()
		if err != nil {
			return nil, fmt.Errorf("getting OAuth2 token: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	}

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read gemini response: %w", err)
	}

	var apiResp geminiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal gemini response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("gemini API error (%s): %s", apiResp.Error.Status, apiResp.Error.Message)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var content string
	if len(apiResp.Candidates) > 0 && apiResp.Candidates[0].Content != nil {
		for _, part := range apiResp.Candidates[0].Content.Parts {
			content += part.Text
		}
	}

	var finishReason string
	if len(apiResp.Candidates) > 0 {
		finishReason = apiResp.Candidates[0].FinishReason
	}

	var inputTokens, outputTokens int
	if apiResp.UsageMetadata != nil {
		inputTokens = apiResp.UsageMetadata.PromptTokenCount
		outputTokens = apiResp.UsageMetadata.CandidatesTokenCount
	}

	return &CompletionResponse{
		Content:      content,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Model:        model,
		FinishReason: finishReason,
	}, nil
}
