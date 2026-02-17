package cmd

import (
	"fmt"
	"os"

	"github.com/ziadkadry99/auto-doc/internal/auth"
	"github.com/ziadkadry99/auto-doc/internal/config"
	"github.com/ziadkadry99/auto-doc/internal/embeddings"
	"github.com/ziadkadry99/auto-doc/internal/llm"
)

// createEmbedderFromConfig creates an embeddings.Embedder based on config.
// This is the shared version used by generate, query, cost, and serve commands.
func createEmbedderFromConfig(cfg *config.Config) (embeddings.Embedder, error) {
	provider := cfg.EmbeddingProvider
	if provider == "" {
		provider = cfg.Provider
	}
	model := cfg.EmbeddingModel
	if model == "" {
		preset := config.GetPreset(provider, cfg.Quality)
		model = preset.EmbeddingModel
	}

	switch provider {
	case config.ProviderOpenAI:
		apiKey := auth.GetAPIKey("openai")
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI API key not found.\nRun `autodoc auth openai` or set OPENAI_API_KEY")
		}
		return embeddings.NewOpenAIEmbedder(apiKey, embeddings.OpenAIModel(model)), nil
	case config.ProviderGoogle:
		apiKey := os.Getenv(config.APIKeyEnvVar(config.ProviderGoogle))
		if apiKey != "" {
			return embeddings.NewGoogleEmbedder(apiKey, embeddings.GoogleModel(model)), nil
		}
		// Try OAuth2 credentials.
		creds, err := auth.Load()
		if err == nil && creds.Google != nil && creds.Google.RefreshToken != "" {
			ts := auth.NewGoogleTokenSource(creds.Google)
			return embeddings.NewGoogleEmbedderWithTokenSource(ts, embeddings.GoogleModel(model)), nil
		}
		return nil, fmt.Errorf("Google API credentials not found.\nRun `autodoc auth google` or set GOOGLE_API_KEY")
	case config.ProviderOllama:
		return embeddings.NewOllamaEmbedder(model, 768, ""), nil
	default:
		// For providers without native embeddings, fall back to OpenAI.
		apiKey := auth.GetAPIKey("openai")
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI API key not found (used for embeddings when provider is %s).\nRun `autodoc auth openai` or set OPENAI_API_KEY", provider)
		}
		return embeddings.NewOpenAIEmbedder(apiKey, embeddings.OpenAIModel(model)), nil
	}
}

// createLLMProviderFromConfig creates an LLM provider based on config settings.
func createLLMProviderFromConfig(cfg *config.Config) (llm.Provider, error) {
	return llm.NewProvider(string(cfg.Provider), cfg.Model)
}

// loadConfig loads and validates the config, providing a user-friendly error.
func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w\nRun `autodoc init` to create a config file", err)
	}
	return cfg, nil
}
