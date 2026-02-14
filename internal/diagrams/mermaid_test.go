package diagrams

import (
	"strings"
	"testing"
)

func TestArchitectureDiagram(t *testing.T) {
	components := []Component{
		{Name: "CLI", Description: "Command entry point"},
		{Name: "Config", Description: "Configuration loader"},
		{Name: "Indexer", Description: "Code analyzer"},
	}
	relationships := []Relationship{
		{From: "CLI", To: "Config", Label: "loads"},
		{From: "CLI", To: "Indexer"},
	}

	result := ArchitectureDiagram(components, relationships)

	if !strings.HasPrefix(result, "graph TD\n") {
		t.Error("diagram should start with 'graph TD'")
	}
	checks := []string{
		`CLI["CLI`,
		`Config["Config`,
		`Indexer["Indexer`,
		"CLI -->|loads| Config",
		"CLI --> Indexer",
	}
	for _, c := range checks {
		if !strings.Contains(result, c) {
			t.Errorf("diagram missing: %q\ngot:\n%s", c, result)
		}
	}
}

func TestDependencyDiagram(t *testing.T) {
	deps := map[string][]string{
		"main.go": {"fmt", "os"},
	}

	result := DependencyDiagram(deps)

	if !strings.HasPrefix(result, "graph LR\n") {
		t.Error("diagram should start with 'graph LR'")
	}
	if !strings.Contains(result, "main_go") {
		t.Errorf("diagram missing sanitized node ID 'main_go', got:\n%s", result)
	}
	if !strings.Contains(result, `"fmt"`) {
		t.Errorf("diagram missing fmt dependency, got:\n%s", result)
	}
}

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"main.go", "main_go"},
		{"src/auth/handler.go", "src_auth_handler_go"},
		{"my-pkg", "my_pkg"},
	}
	for _, tt := range tests {
		got := sanitizeID(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEscapeMermaid(t *testing.T) {
	got := escapeMermaid(`say "hello"`)
	if !strings.Contains(got, "#quot;") {
		t.Errorf("expected escaped quotes, got: %s", got)
	}
}
