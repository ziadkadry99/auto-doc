package llm

import "context"

// Provider defines the interface for LLM providers.
type Provider interface {
	// Complete sends a completion request and returns the response.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	// Name returns the name of this provider.
	Name() string
}
