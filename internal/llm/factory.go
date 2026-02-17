package llm

import (
	"fmt"
	"os"

	"github.com/ziadkadry99/auto-doc/internal/auth"
	"golang.org/x/oauth2"
)

// NewProvider creates a new LLM provider based on the given provider type and model.
// Supported provider types: "anthropic", "openai", "google", "ollama".
// Credential lookup order: env var → stored credentials → error.
func NewProvider(providerType string, model string) (Provider, error) {
	switch providerType {
	case "anthropic":
		apiKey := auth.GetAPIKey("anthropic")
		if apiKey == "" {
			return nil, fmt.Errorf("Anthropic API key not found.\nRun `autodoc auth anthropic` or set ANTHROPIC_API_KEY")
		}
		return NewAnthropicProvider(apiKey, model), nil

	case "openai":
		apiKey := auth.GetAPIKey("openai")
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI API key not found.\nRun `autodoc auth openai` or set OPENAI_API_KEY")
		}
		return NewOpenAIProvider(apiKey, model), nil

	case "google":
		apiKey := os.Getenv("GOOGLE_API_KEY")
		if apiKey != "" {
			return NewGoogleProvider(apiKey, model), nil
		}
		// Try OAuth2 credentials.
		ts, err := googleTokenSource()
		if err == nil && ts != nil {
			return NewGoogleProviderWithTokenSource(ts, model), nil
		}
		return nil, fmt.Errorf("Google API credentials not found.\nRun `autodoc auth google` or set GOOGLE_API_KEY")

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

// googleTokenSource loads stored Google OAuth credentials and returns a token source.
func googleTokenSource() (oauth2.TokenSource, error) {
	creds, err := auth.Load()
	if err != nil {
		return nil, err
	}
	if creds.Google == nil || creds.Google.RefreshToken == "" {
		return nil, fmt.Errorf("no Google OAuth credentials stored")
	}
	return auth.NewGoogleTokenSource(creds.Google), nil
}
