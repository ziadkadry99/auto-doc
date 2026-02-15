package indexer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDepMatchesDir(t *testing.T) {
	tests := []struct {
		depName string
		dir     string
		want    bool
	}{
		// Exact match.
		{"internal/config", "internal/config", true},
		// Dep ends with dir.
		{"github.com/foo/internal/config", "internal/config", true},
		// Same base segment.
		{"config", "internal/config", true},
		// Contains as component.
		{"github.com/foo/internal/config/v2", "internal/config", true},
		// No match.
		{"github.com/foo/logging", "internal/config", false},
		// Empty inputs.
		{"", "internal/config", false},
		{"config", "", false},
		{"config", ".", false},
		// Partial base mismatch.
		{"myconfig", "internal/config", false},
	}

	for _, tt := range tests {
		t.Run(tt.depName+"_vs_"+tt.dir, func(t *testing.T) {
			got := depMatchesDir(tt.depName, tt.dir)
			if got != tt.want {
				t.Errorf("depMatchesDir(%q, %q) = %v, want %v", tt.depName, tt.dir, got, tt.want)
			}
		})
	}
}

func TestExpandChangedFiles_NoDeps(t *testing.T) {
	changed := []string{"cmd/main.go"}
	analyses := map[string]FileAnalysis{
		"cmd/main.go": {FilePath: "cmd/main.go"},
		"internal/config/config.go": {
			FilePath: "internal/config/config.go",
		},
	}

	expanded, depAffected := ExpandChangedFiles(changed, analyses)

	if len(expanded) != 1 {
		t.Errorf("expected 1 expanded file, got %d", len(expanded))
	}
	if len(depAffected) != 0 {
		t.Errorf("expected 0 dep-affected files, got %d", len(depAffected))
	}
}

func TestExpandChangedFiles_DirectDep(t *testing.T) {
	changed := []string{"internal/config/types.go"}
	analyses := map[string]FileAnalysis{
		"internal/config/types.go": {
			FilePath: "internal/config/types.go",
		},
		"cmd/generate.go": {
			FilePath: "cmd/generate.go",
			Dependencies: []Dependency{
				{Name: "github.com/foo/internal/config", Type: "import"},
			},
		},
		"internal/unrelated/foo.go": {
			FilePath: "internal/unrelated/foo.go",
			Dependencies: []Dependency{
				{Name: "fmt", Type: "import"},
			},
		},
	}

	expanded, depAffected := ExpandChangedFiles(changed, analyses)

	if len(expanded) != 2 {
		t.Errorf("expected 2 expanded files, got %d: %v", len(expanded), expanded)
	}
	if len(depAffected) != 1 {
		t.Errorf("expected 1 dep-affected file, got %d: %v", len(depAffected), depAffected)
	}
	if len(depAffected) > 0 && depAffected[0] != "cmd/generate.go" {
		t.Errorf("expected dep-affected to be cmd/generate.go, got %s", depAffected[0])
	}
}

func TestExpandChangedFiles_TransitiveDep(t *testing.T) {
	// C depends on B, B depends on A. Change A → both B and C should be affected.
	changed := []string{"pkg/a/types.go"}
	analyses := map[string]FileAnalysis{
		"pkg/a/types.go": {
			FilePath: "pkg/a/types.go",
		},
		"pkg/b/handler.go": {
			FilePath: "pkg/b/handler.go",
			Dependencies: []Dependency{
				{Name: "pkg/a", Type: "import"},
			},
		},
		"pkg/c/service.go": {
			FilePath: "pkg/c/service.go",
			Dependencies: []Dependency{
				{Name: "pkg/b", Type: "import"},
			},
		},
	}

	expanded, depAffected := ExpandChangedFiles(changed, analyses)

	if len(expanded) != 3 {
		t.Errorf("expected 3 expanded files, got %d: %v", len(expanded), expanded)
	}
	if len(depAffected) != 2 {
		t.Errorf("expected 2 dep-affected files, got %d: %v", len(depAffected), depAffected)
	}
}

func TestExpandChangedFiles_CircularDeps(t *testing.T) {
	// A depends on B, B depends on A. Should not loop forever.
	changed := []string{"pkg/a/main.go"}
	analyses := map[string]FileAnalysis{
		"pkg/a/main.go": {
			FilePath: "pkg/a/main.go",
			Dependencies: []Dependency{
				{Name: "pkg/b", Type: "import"},
			},
		},
		"pkg/b/main.go": {
			FilePath: "pkg/b/main.go",
			Dependencies: []Dependency{
				{Name: "pkg/a", Type: "import"},
			},
		},
	}

	expanded, depAffected := ExpandChangedFiles(changed, analyses)

	if len(expanded) != 2 {
		t.Errorf("expected 2 expanded files, got %d: %v", len(expanded), expanded)
	}
	if len(depAffected) != 1 {
		t.Errorf("expected 1 dep-affected file, got %d: %v", len(depAffected), depAffected)
	}
}

func TestExpandChangedFiles_EmptyAnalyses(t *testing.T) {
	changed := []string{"cmd/main.go", "internal/foo.go"}
	expanded, depAffected := ExpandChangedFiles(changed, map[string]FileAnalysis{})

	if len(expanded) != 2 {
		t.Errorf("expected 2 expanded files, got %d", len(expanded))
	}
	if len(depAffected) != 0 {
		t.Errorf("expected 0 dep-affected files, got %d", len(depAffected))
	}
}

func TestSaveAndLoadAnalyses_Roundtrip(t *testing.T) {
	dir := t.TempDir()

	analyses := map[string]FileAnalysis{
		"cmd/main.go": {
			FilePath: "cmd/main.go",
			Language: "go",
			Summary:  "Main entry point",
			Dependencies: []Dependency{
				{Name: "internal/config", Type: "import"},
			},
		},
		"internal/config/config.go": {
			FilePath: "internal/config/config.go",
			Language: "go",
			Summary:  "Configuration management",
		},
	}

	if err := SaveAnalyses(dir, analyses); err != nil {
		t.Fatalf("SaveAnalyses failed: %v", err)
	}

	// Verify file exists.
	path := filepath.Join(dir, ".autodoc", "analyses.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("analyses.json was not created")
	}

	loaded, err := LoadAnalyses(dir)
	if err != nil {
		t.Fatalf("LoadAnalyses failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Errorf("expected 2 analyses, got %d", len(loaded))
	}

	if loaded["cmd/main.go"].Summary != "Main entry point" {
		t.Errorf("unexpected summary: %s", loaded["cmd/main.go"].Summary)
	}

	if len(loaded["cmd/main.go"].Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(loaded["cmd/main.go"].Dependencies))
	}
}

func TestLoadAnalyses_MissingFile(t *testing.T) {
	dir := t.TempDir()

	loaded, err := LoadAnalyses(dir)
	if err != nil {
		t.Fatalf("LoadAnalyses should not error on missing file: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected empty map, got %d entries", len(loaded))
	}
}

func TestParseRegenerationAdvice_Valid(t *testing.T) {
	response := `PROJECT_OVERVIEW: YES
ARCHITECTURE: NO
FEATURE_PAGES: YES
COMPONENT_MAP: NO
REASONING: Only overview and features changed due to new API endpoints`

	advice := parseRegenerationAdvice(response)
	if advice == nil {
		t.Fatal("expected non-nil advice")
	}
	if !advice.ProjectOverview {
		t.Error("expected ProjectOverview=true")
	}
	if advice.Architecture {
		t.Error("expected Architecture=false")
	}
	if !advice.FeaturePages {
		t.Error("expected FeaturePages=true")
	}
	if advice.ComponentMap {
		t.Error("expected ComponentMap=false")
	}
	if advice.Reasoning != "Only overview and features changed due to new API endpoints" {
		t.Errorf("unexpected reasoning: %s", advice.Reasoning)
	}
}

func TestParseRegenerationAdvice_Invalid(t *testing.T) {
	advice := parseRegenerationAdvice("This is not a valid response at all")
	if advice != nil {
		t.Error("expected nil for unparseable response")
	}
}

func TestParseRegenerationAdvice_AllNo(t *testing.T) {
	response := `PROJECT_OVERVIEW: NO
ARCHITECTURE: NO
FEATURE_PAGES: NO
COMPONENT_MAP: NO
REASONING: Minor internal refactor with no user-facing changes`

	advice := parseRegenerationAdvice(response)
	if advice == nil {
		t.Fatal("expected non-nil advice")
	}
	if advice.ProjectOverview || advice.Architecture || advice.FeaturePages || advice.ComponentMap {
		t.Error("expected all fields to be false")
	}
}
