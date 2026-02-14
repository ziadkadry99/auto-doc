package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Provider != ProviderAnthropic {
		t.Errorf("expected default provider %q, got %q", ProviderAnthropic, cfg.Provider)
	}
	if cfg.Quality != QualityNormal {
		t.Errorf("expected default quality %q, got %q", QualityNormal, cfg.Quality)
	}
	if cfg.OutputDir != "docs" {
		t.Errorf("expected default output_dir %q, got %q", "docs", cfg.OutputDir)
	}
	if cfg.MaxConcurrency != 5 {
		t.Errorf("expected default max_concurrency 5, got %d", cfg.MaxConcurrency)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.autodoc.yml")

	original := DefaultConfig()
	original.Provider = ProviderOpenAI
	original.Model = "gpt-4o"
	original.Quality = QualityMax
	original.Include = []string{"**/*.go", "**/*.py"}
	original.OutputDir = "output"
	original.MaxCostUSD = 25.5

	// Save.
	if err := original.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load back.
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify round-trip.
	if loaded.Provider != original.Provider {
		t.Errorf("provider: got %q, want %q", loaded.Provider, original.Provider)
	}
	if loaded.Model != original.Model {
		t.Errorf("model: got %q, want %q", loaded.Model, original.Model)
	}
	if loaded.Quality != original.Quality {
		t.Errorf("quality: got %q, want %q", loaded.Quality, original.Quality)
	}
	if loaded.OutputDir != original.OutputDir {
		t.Errorf("output_dir: got %q, want %q", loaded.OutputDir, original.OutputDir)
	}
	if loaded.MaxCostUSD != original.MaxCostUSD {
		t.Errorf("max_cost_usd: got %f, want %f", loaded.MaxCostUSD, original.MaxCostUSD)
	}
	if len(loaded.Include) != len(original.Include) {
		t.Errorf("include length: got %d, want %d", len(loaded.Include), len(original.Include))
	}
	for i, v := range loaded.Include {
		if v != original.Include[i] {
			t.Errorf("include[%d]: got %q, want %q", i, v, original.Include[i])
		}
	}
}

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.yml")

	// Loading a missing file should return defaults, not an error.
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load should not fail for missing file: %v", err)
	}
	if cfg.Provider != ProviderAnthropic {
		t.Errorf("expected default provider, got %q", cfg.Provider)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yml")

	cfg := DefaultConfig()
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Override provider via env var.
	os.Setenv("AUTODOC_PROVIDER", "openai")
	defer os.Unsetenv("AUTODOC_PROVIDER")

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Provider != ProviderOpenAI {
		t.Errorf("env override failed: got %q, want %q", loaded.Provider, ProviderOpenAI)
	}
}

func TestValidateValid(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("DefaultConfig should be valid, got: %v", err)
	}
}

func TestValidateInvalidProvider(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for invalid provider")
	}
}

func TestValidateEmptyProvider(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for empty provider")
	}
}

func TestValidateEmptyModel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Model = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for empty model")
	}
}

func TestValidateInvalidQuality(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Quality = "ultra"
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for invalid quality")
	}
}

func TestValidateEmptyOutputDir(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OutputDir = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for empty output_dir")
	}
}

func TestValidateNegativeConcurrency(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConcurrency = -1
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for negative max_concurrency")
	}
}

func TestValidateNegativeCost(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxCostUSD = -5.0
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for negative max_cost_usd")
	}
}

func TestGetPreset(t *testing.T) {
	p := GetPreset(ProviderAnthropic, QualityLite)
	if p.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("expected haiku model, got %q", p.Model)
	}

	p = GetPreset(ProviderOpenAI, QualityMax)
	if p.Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %q", p.Model)
	}

	// Unknown combination falls back.
	p = GetPreset("unknown", QualityLite)
	if p.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("expected fallback to sonnet, got %q", p.Model)
	}
}

func TestAPIKeyEnvVar(t *testing.T) {
	tests := []struct {
		provider ProviderType
		want     string
	}{
		{ProviderAnthropic, "ANTHROPIC_API_KEY"},
		{ProviderOpenAI, "OPENAI_API_KEY"},
		{ProviderGoogle, "GOOGLE_API_KEY"},
		{ProviderOllama, ""},
	}
	for _, tt := range tests {
		got := APIKeyEnvVar(tt.provider)
		if got != tt.want {
			t.Errorf("APIKeyEnvVar(%q) = %q, want %q", tt.provider, got, tt.want)
		}
	}
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"**/*.go", []string{"**/*.go"}},
		{"", nil},
		{"  ,  , ", nil},
	}
	for _, tt := range tests {
		got := splitAndTrim(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitAndTrim(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
			continue
		}
		for i, v := range got {
			if v != tt.want[i] {
				t.Errorf("splitAndTrim(%q)[%d] = %q, want %q", tt.input, i, v, tt.want[i])
			}
		}
	}
}
