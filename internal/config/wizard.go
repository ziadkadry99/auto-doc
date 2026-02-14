package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/manifoldco/promptui"
)

// projectTypePatterns maps marker files to human-readable project types
// and a recommended include glob.
var projectTypePatterns = map[string]struct {
	Name    string
	Include string
}{
	"go.mod":           {Name: "Go", Include: "**/*.go"},
	"package.json":     {Name: "Node.js/TypeScript", Include: "**/*.{js,ts,jsx,tsx}"},
	"requirements.txt": {Name: "Python", Include: "**/*.py"},
	"pyproject.toml":   {Name: "Python", Include: "**/*.py"},
	"Cargo.toml":       {Name: "Rust", Include: "**/*.rs"},
	"pom.xml":          {Name: "Java", Include: "**/*.java"},
	"build.gradle":     {Name: "Java/Kotlin", Include: "**/*.{java,kt}"},
	"Gemfile":          {Name: "Ruby", Include: "**/*.rb"},
	"composer.json":    {Name: "PHP", Include: "**/*.php"},
	"*.csproj":         {Name: ".NET", Include: "**/*.cs"},
}

// detectProjectType checks the current directory for well-known project markers.
func detectProjectType() (name string, include string) {
	for marker, info := range projectTypePatterns {
		matches, _ := filepath.Glob(marker)
		if len(matches) > 0 {
			return info.Name, info.Include
		}
	}
	return "", "**"
}

// RunWizard runs an interactive configuration wizard and returns the
// resulting Config. It also saves the config to .autodoc.yml.
func RunWizard() (*Config, error) {
	fmt.Println("Welcome to autodoc! Let's configure your project.")
	fmt.Println()

	// Detect project type.
	projType, defaultInclude := detectProjectType()
	if projType != "" {
		fmt.Printf("Detected project type: %s\n\n", projType)
	}

	// 1. Provider selection.
	providerPrompt := promptui.Select{
		Label: "Select LLM provider",
		Items: []string{"anthropic", "openai", "google", "ollama"},
	}
	_, providerStr, err := providerPrompt.Run()
	if err != nil {
		return nil, fmt.Errorf("provider selection: %w", err)
	}
	provider := ProviderType(providerStr)

	// 2. Quality tier.
	qualityPrompt := promptui.Select{
		Label: "Select quality tier",
		Items: []string{
			"lite   — fast & cheap (haiku / gpt-4o-mini)",
			"normal — balanced (sonnet / gpt-4o)",
			"max    — highest quality (opus / gpt-4)",
		},
	}
	qualityIdx, _, err := qualityPrompt.Run()
	if err != nil {
		return nil, fmt.Errorf("quality selection: %w", err)
	}
	tiers := []QualityTier{QualityLite, QualityNormal, QualityMax}
	quality := tiers[qualityIdx]

	preset := GetPreset(provider, quality)

	// 3. Output directory.
	outputPrompt := promptui.Prompt{
		Label:   "Output directory for generated docs",
		Default: "docs",
	}
	outputDir, err := outputPrompt.Run()
	if err != nil {
		return nil, fmt.Errorf("output dir: %w", err)
	}

	// 4. Include patterns.
	includePrompt := promptui.Prompt{
		Label:   "Include patterns (comma-separated globs)",
		Default: defaultInclude,
	}
	includeStr, err := includePrompt.Run()
	if err != nil {
		return nil, fmt.Errorf("include patterns: %w", err)
	}
	include := splitAndTrim(includeStr)

	// 5. Extra exclude patterns.
	excludePrompt := promptui.Prompt{
		Label:   "Extra exclude patterns (comma-separated, leave blank for defaults)",
		Default: "",
	}
	excludeStr, err := excludePrompt.Run()
	if err != nil {
		return nil, fmt.Errorf("exclude patterns: %w", err)
	}
	exclude := DefaultExcludes
	if excludeStr != "" {
		exclude = append(exclude, splitAndTrim(excludeStr)...)
	}

	// Build the config.
	cfg := &Config{
		Provider:          provider,
		Model:             preset.Model,
		EmbeddingProvider: embeddingProviderFor(provider),
		EmbeddingModel:    preset.EmbeddingModel,
		Quality:           quality,
		OutputDir:         outputDir,
		Include:           include,
		Exclude:           exclude,
		MaxConcurrency:    5,
		MaxCostUSD:        10.0,
		CI: CIConfig{
			AutoCommit:  false,
			FailOnError: true,
		},
	}

	// Check for API key.
	envVar := APIKeyEnvVar(provider)
	if envVar != "" {
		if os.Getenv(envVar) == "" {
			fmt.Printf("\nNote: Set %s in your environment before running autodoc generate.\n", envVar)
		}
	}

	// Save to .autodoc.yml.
	configPath := ".autodoc.yml"
	if err := cfg.Save(configPath); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\nConfiguration saved to %s\n", configPath)
	return cfg, nil
}

// embeddingProviderFor returns the default embedding provider for a given
// LLM provider. OpenAI embeddings are used for all cloud providers.
func embeddingProviderFor(p ProviderType) ProviderType {
	if p == ProviderOllama {
		return ProviderOllama
	}
	return ProviderOpenAI
}

// splitAndTrim splits a comma-separated string and trims whitespace.
func splitAndTrim(s string) []string {
	var result []string
	for _, part := range filepath.SplitList(s) {
		// filepath.SplitList uses OS path list separator; we want comma.
		result = append(result, part)
	}
	// Actually, split by comma.
	result = nil
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			token := trimSpace(s[start:i])
			if token != "" {
				result = append(result, token)
			}
			start = i + 1
		}
	}
	return result
}

func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}
