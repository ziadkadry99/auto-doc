package cmd

import (
	"fmt"
	"os"

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
		apiKey := os.Getenv(config.APIKeyEnvVar(config.ProviderOpenAI))
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required for OpenAI embeddings")
		}
		return embeddings.NewOpenAIEmbedder(apiKey, embeddings.OpenAIModel(model)), nil
	case config.ProviderOllama:
		return embeddings.NewOllamaEmbedder(model, 768, ""), nil
	default:
		// For providers without native embeddings, fall back to OpenAI.
		apiKey := os.Getenv(config.APIKeyEnvVar(config.ProviderOpenAI))
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY is required (used for embeddings when provider is %s)", provider)
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
