package config

// QualityTier controls the model selection and trade-off between speed/cost and quality.
type QualityTier string

const (
	QualityLite   QualityTier = "lite"
	QualityNormal QualityTier = "normal"
	QualityMax    QualityTier = "max"
)

// ProviderType identifies an LLM provider.
type ProviderType string

const (
	ProviderAnthropic ProviderType = "anthropic"
	ProviderOpenAI    ProviderType = "openai"
	ProviderGoogle    ProviderType = "google"
	ProviderOllama    ProviderType = "ollama"
	ProviderMiniMax   ProviderType = "minimax"
)

// Config is the top-level autodoc configuration, corresponding to .autodoc.yml.
type Config struct {
	Provider          ProviderType `yaml:"provider" koanf:"provider"`
	Model             string       `yaml:"model" koanf:"model"`
	EmbeddingProvider ProviderType `yaml:"embedding_provider" koanf:"embedding_provider"`
	EmbeddingModel    string       `yaml:"embedding_model" koanf:"embedding_model"`
	Quality           QualityTier  `yaml:"quality" koanf:"quality"`
	OutputDir         string       `yaml:"output_dir" koanf:"output_dir"`
	Logo              string       `yaml:"logo" koanf:"logo"`
	Include           []string     `yaml:"include" koanf:"include"`
	Exclude           []string     `yaml:"exclude" koanf:"exclude"`
	ContextFile       string       `yaml:"context_file" koanf:"context_file"`
	CI                CIConfig     `yaml:"ci" koanf:"ci"`
	MaxConcurrency    int          `yaml:"max_concurrency" koanf:"max_concurrency"`
	MaxCostUSD        float64      `yaml:"max_cost_usd" koanf:"max_cost_usd"`
}

// CIConfig holds CI-specific settings.
type CIConfig struct {
	AutoCommit  bool `yaml:"auto_commit" koanf:"auto_commit"`
	FailOnError bool `yaml:"fail_on_error" koanf:"fail_on_error"`
}
