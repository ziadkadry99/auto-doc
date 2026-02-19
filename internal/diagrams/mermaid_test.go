package diagrams

import (
	"encoding/json"
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

	var data DiagramData
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("result is not valid JSON: %v\ngot: %s", err, result)
	}
	if len(data.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(data.Nodes))
	}
	if len(data.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(data.Edges))
	}
	// Check node labels.
	labels := make(map[string]bool)
	for _, n := range data.Nodes {
		labels[n.Label] = true
	}
	for _, want := range []string{"CLI", "Config", "Indexer"} {
		if !labels[want] {
			t.Errorf("missing node label %q", want)
		}
	}
	// Check edge with label.
	if data.Edges[0].Label != "loads" {
		t.Errorf("expected first edge label 'loads', got %q", data.Edges[0].Label)
	}
}

func TestDependencyDiagram(t *testing.T) {
	deps := map[string][]string{
		"main.go": {"fmt", "os"},
	}

	result := DependencyDiagram(deps)

	var data DiagramData
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("result is not valid JSON: %v\ngot: %s", err, result)
	}
	if len(data.Nodes) != 3 {
		t.Errorf("expected 3 nodes (main.go, fmt, os), got %d", len(data.Nodes))
	}
	if len(data.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(data.Edges))
	}
	// Check that main_go node exists.
	found := false
	for _, n := range data.Nodes {
		if n.ID == "main_go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected node with ID 'main_go'")
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

	got = escapeMermaid("Factory (pattern) support")
	if strings.Contains(got, "(") || strings.Contains(got, ")") {
		t.Errorf("expected escaped parens, got: %s", got)
	}
	if !strings.Contains(got, "#lpar;") || !strings.Contains(got, "#rpar;") {
		t.Errorf("expected #lpar; and #rpar;, got: %s", got)
	}

	got = escapeMermaid("map[string]bool")
	if strings.Contains(got, "[") || strings.Contains(got, "]") {
		t.Errorf("expected escaped brackets, got: %s", got)
	}
}
