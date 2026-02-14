package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	yamlv3 "gopkg.in/yaml.v3"
)

// Load reads configuration from the given YAML file, then overlays
// environment variable overrides (AUTODOC_*).
func Load(path string) (*Config, error) {
	k := koanf.New(".")

	// Start from defaults.
	cfg := DefaultConfig()

	// Load YAML file if it exists.
	if _, err := os.Stat(path); err == nil {
		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("reading config %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("accessing config %s: %w", path, err)
	}

	// Overlay environment variables: AUTODOC_PROVIDER -> provider, etc.
	if err := k.Load(env.Provider("AUTODOC_", ".", func(s string) string {
		return strings.ToLower(strings.TrimPrefix(s, "AUTODOC_"))
	}), nil); err != nil {
		return nil, fmt.Errorf("loading env overrides: %w", err)
	}

	if err := k.Unmarshal("", cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	return cfg, nil
}

// Save writes the configuration to the given YAML file path.
func (c *Config) Save(path string) error {
	data, err := yamlv3.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config to %s: %w", path, err)
	}
	return nil
}

// validProviders is the set of recognized provider values.
var validProviders = map[ProviderType]bool{
	ProviderAnthropic: true,
	ProviderOpenAI:    true,
	ProviderGoogle:    true,
	ProviderOllama:    true,
}

// validQualityTiers is the set of recognized quality tier values.
var validQualityTiers = map[QualityTier]bool{
	QualityLite:   true,
	QualityNormal: true,
	QualityMax:    true,
}

// Validate checks that the configuration contains valid values.
func (c *Config) Validate() error {
	if c.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if !validProviders[c.Provider] {
		return fmt.Errorf("invalid provider %q: must be one of anthropic, openai, google, ollama", c.Provider)
	}

	if c.Model == "" {
		return fmt.Errorf("model is required")
	}

	if c.EmbeddingProvider != "" && !validProviders[c.EmbeddingProvider] {
		return fmt.Errorf("invalid embedding_provider %q", c.EmbeddingProvider)
	}

	if c.Quality != "" && !validQualityTiers[c.Quality] {
		return fmt.Errorf("invalid quality %q: must be one of lite, normal, max", c.Quality)
	}

	if c.OutputDir == "" {
		return fmt.Errorf("output_dir is required")
	}

	if c.MaxConcurrency < 0 {
		return fmt.Errorf("max_concurrency must be non-negative")
	}

	if c.MaxCostUSD < 0 {
		return fmt.Errorf("max_cost_usd must be non-negative")
	}

	return nil
}

// APIKeyEnvVar returns the conventional environment variable name for
// the API key of the given provider.
func APIKeyEnvVar(provider ProviderType) string {
	switch provider {
	case ProviderAnthropic:
		return "ANTHROPIC_API_KEY"
	case ProviderOpenAI:
		return "OPENAI_API_KEY"
	case ProviderGoogle:
		return "GOOGLE_API_KEY"
	default:
		return ""
	}
}
