package config

// QualityPreset describes the models to use for a given quality tier.
type QualityPreset struct {
	Model          string
	EmbeddingModel string
}

// qualityPresets maps each provider+quality combination to its model choices.
var qualityPresets = map[ProviderType]map[QualityTier]QualityPreset{
	ProviderAnthropic: {
		QualityLite:   {Model: "claude-haiku-4-5-20251001", EmbeddingModel: "text-embedding-3-small"},
		QualityNormal: {Model: "claude-sonnet-4-5-20250929", EmbeddingModel: "text-embedding-3-small"},
		QualityMax:    {Model: "claude-opus-4-6", EmbeddingModel: "text-embedding-3-large"},
	},
	ProviderOpenAI: {
		QualityLite:   {Model: "gpt-4o-mini", EmbeddingModel: "text-embedding-3-small"},
		QualityNormal: {Model: "gpt-4o", EmbeddingModel: "text-embedding-3-small"},
		QualityMax:    {Model: "gpt-4", EmbeddingModel: "text-embedding-3-large"},
	},
	ProviderGoogle: {
		QualityLite:   {Model: "gemini-3-flash-preview", EmbeddingModel: "text-embedding-004"},
		QualityNormal: {Model: "gemini-3-pro-preview", EmbeddingModel: "text-embedding-004"},
		QualityMax:    {Model: "gemini-3-pro-preview", EmbeddingModel: "text-embedding-004"},
	},
	ProviderOllama: {
		QualityLite:   {Model: "llama3", EmbeddingModel: "nomic-embed-text"},
		QualityNormal: {Model: "llama3", EmbeddingModel: "nomic-embed-text"},
		QualityMax:    {Model: "llama3:70b", EmbeddingModel: "nomic-embed-text"},
	},
	ProviderMiniMax: {
		QualityLite:   {Model: "MiniMax-M2.5-highspeed", EmbeddingModel: "text-embedding-3-small"},
		QualityNormal: {Model: "MiniMax-M2.5", EmbeddingModel: "text-embedding-3-small"},
		QualityMax:    {Model: "MiniMax-M2.5", EmbeddingModel: "text-embedding-3-large"},
	},
	ProviderOpenRouter: {
		QualityLite:   {Model: "minimax/minimax-m2.5", EmbeddingModel: "text-embedding-3-small"},
		QualityNormal: {Model: "minimax/minimax-m2.5", EmbeddingModel: "text-embedding-3-small"},
		QualityMax:    {Model: "minimax/minimax-m2.5", EmbeddingModel: "text-embedding-3-large"},
	},
}

// DefaultExcludes are glob patterns excluded from analysis by default.
var DefaultExcludes = []string{
	"vendor/**",
	"node_modules/**",
	".git/**",
	"dist/**",
	"build/**",
	"*.min.js",
	"*.min.css",
	"*.lock",
	"go.sum",
	"package-lock.json",
	"yarn.lock",
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Provider:          ProviderAnthropic,
		Model:             "claude-sonnet-4-5-20250929",
		EmbeddingProvider: ProviderOpenAI,
		EmbeddingModel:    "text-embedding-3-small",
		Quality:           QualityNormal,
		OutputDir:         "docs",
		Include:           []string{"**"},
		Exclude:           DefaultExcludes,
		MaxConcurrency:    5,
		MaxCostUSD:        10.0,
		CI: CIConfig{
			AutoCommit:  false,
			FailOnError: true,
		},
	}
}

// GetPreset returns the quality preset for the given provider and tier.
// Returns the Normal Anthropic preset if the combination is not found.
func GetPreset(provider ProviderType, tier QualityTier) QualityPreset {
	if tiers, ok := qualityPresets[provider]; ok {
		if preset, ok := tiers[tier]; ok {
			return preset
		}
	}
	return qualityPresets[ProviderAnthropic][QualityNormal]
}
