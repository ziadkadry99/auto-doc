package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ziadkadry99/auto-doc/internal/indexer"
)

func sampleAnalyses() []indexer.FileAnalysis {
	return []indexer.FileAnalysis{
		{
			FilePath: "cmd/main.go",
			Language: "go",
			Summary:  "Entry point for the application.",
			Purpose:  "Initializes and starts the CLI.",
			Functions: []indexer.FunctionDoc{
				{
					Name:      "main",
					Signature: "func main()",
					Summary:   "Application entry point.",
					LineStart: 10,
					LineEnd:   25,
				},
				{
					Name:      "run",
					Signature: "func run(cfg Config) error",
					Summary:   "Runs the main logic.",
					Parameters: []indexer.ParamDoc{
						{Name: "cfg", Type: "Config", Description: "Application configuration"},
					},
					Returns:   "error if execution fails",
					LineStart: 27,
					LineEnd:   50,
				},
			},
			Classes: []indexer.ClassDoc{
				{
					Name:    "App",
					Summary: "Holds application state.",
					Fields: []indexer.FieldDoc{
						{Name: "Name", Type: "string", Description: "Application name"},
					},
					Methods: []indexer.FunctionDoc{
						{
							Name:      "Start",
							Signature: "func (a *App) Start() error",
							Summary:   "Starts the application.",
						},
					},
					LineStart: 5,
					LineEnd:   8,
				},
			},
			Dependencies: []indexer.Dependency{
				{Name: "cobra", Type: "import"},
				{Name: "fmt", Type: "import"},
			},
			KeyLogic: []string{
				"Parses CLI flags before initializing config.",
			},
		},
		{
			FilePath: "internal/config/config.go",
			Language: "go",
			Summary:  "Configuration loading and validation.",
			Purpose:  "Loads config from file and env.",
		},
	}
}

func TestGenerateFileDocs(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewDocGenerator(tmpDir)

	analyses := sampleAnalyses()
	if err := gen.GenerateFileDocs(analyses); err != nil {
		t.Fatalf("GenerateFileDocs failed: %v", err)
	}

	// Check that the file was created with the right path.
	outPath := filepath.Join(tmpDir, "docs", "cmd", "main.go.md")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("expected file %s to exist: %v", outPath, err)
	}

	content := string(data)

	// Verify key sections are present.
	checks := []string{
		"# cmd/main.go",
		"## Summary",
		"Entry point for the application.",
		"## Purpose",
		"## Functions",
		"### main",
		"func main()",
		"### run",
		"**Parameters:**",
		"| cfg |",
		"**Returns:**",
		"## Types",
		"### App",
		"## Dependencies",
		"| cobra | import |",
		"## Key Business Logic",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("file doc missing expected content: %q", check)
		}
	}

	// Verify the second file was created too.
	outPath2 := filepath.Join(tmpDir, "docs", "internal", "config", "config.go.md")
	if _, err := os.Stat(outPath2); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", outPath2)
	}
}

func TestGenerateIndex(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewDocGenerator(tmpDir)

	analyses := sampleAnalyses()
	if err := gen.GenerateIndex(analyses); err != nil {
		t.Fatalf("GenerateIndex failed: %v", err)
	}

	outPath := filepath.Join(tmpDir, "docs", "index.md")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("expected index.md to exist: %v", err)
	}

	content := string(data)

	checks := []string{
		"Documentation",
		"## Files",
		"cmd/main.go",
		"internal/config/config.go",
		"Architecture",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("index.md missing expected content: %q", check)
		}
	}
}

func TestParseArchResponse(t *testing.T) {
	input := `===OVERVIEW===
This project is a CLI tool.

It generates documentation automatically.

===COMPONENTS===
CLI: Command-line interface entry point
Config: Configuration management
Indexer: Source code analysis

===DATAFLOW===
Files are read, analyzed by LLM, and output as markdown.

===PATTERNS===
- Factory pattern
- Strategy pattern`

	data := parseArchResponse(input)

	if !strings.Contains(data.Overview, "CLI tool") {
		t.Errorf("overview missing expected content, got: %s", data.Overview)
	}
	if len(data.Components) != 3 {
		t.Errorf("expected 3 components, got %d", len(data.Components))
	}
	if data.Components[0].Name != "CLI" {
		t.Errorf("expected first component name 'CLI', got %q", data.Components[0].Name)
	}
	if !strings.Contains(data.DataFlow, "markdown") {
		t.Errorf("data flow missing expected content, got: %s", data.DataFlow)
	}
	if len(data.DesignPatterns) != 2 {
		t.Errorf("expected 2 patterns, got %d", len(data.DesignPatterns))
	}
}

func TestAnchorize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"MyFunction", "myfunction"},
		{"Hello World", "hello-world"},
		{"run()", "run"},
	}
	for _, tt := range tests {
		got := anchorize(tt.input)
		if got != tt.want {
			t.Errorf("anchorize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
