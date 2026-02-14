package llm

import (
	"fmt"
	"os"
)

// NewProvider creates a new LLM provider based on the given provider type and model.
// Supported provider types: "anthropic", "openai", "google", "ollama".
func NewProvider(providerType string, model string) (Provider, error) {
	switch providerType {
	case "anthropic":
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable is not set")
		}
		return NewAnthropicProvider(apiKey, model), nil

	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY environment variable is not set")
		}
		return NewOpenAIProvider(apiKey, model), nil

	case "google":
		apiKey := os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("GOOGLE_API_KEY environment variable is not set")
		}
		return NewGoogleProvider(apiKey, model), nil

	case "ollama":
		host := os.Getenv("OLLAMA_HOST")
		if host == "" {
			host = "http://localhost:11434"
		}
		return NewOllamaProvider(host, model), nil

	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}
