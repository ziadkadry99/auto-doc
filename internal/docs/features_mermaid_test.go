package docs

import (
	"strings"
	"testing"
)

func TestSanitizeMermaidFixesMalformedNodeLine(t *testing.T) {
	input := `graph TD
subgraph Integration Layer
MCPServer[Multi-Agent Protocol (MCP) Server]    en
end`

	got := sanitizeMermaid(input)

	if strings.Contains(got, "Server]    en") {
		t.Fatalf("expected trailing garbage to be removed, got:\n%s", got)
	}
	if !strings.Contains(got, `MCPServer["Multi-Agent Protocol #lpar;MCP#rpar; Server"]`) {
		t.Fatalf("expected normalized quoted label, got:\n%s", got)
	}
	if !isMermaidDiagramValid(got) {
		t.Fatalf("expected sanitized diagram to be valid, got:\n%s", got)
	}
}

func TestSanitizeMermaidDropsInvalidFreeText(t *testing.T) {
	input := `MCPServer[Node]
this is not valid mermaid`

	got := sanitizeMermaid(input)

	if !strings.HasPrefix(got, "graph TD\n") {
		t.Fatalf("expected graph header to be inserted, got:\n%s", got)
	}
	if strings.Contains(got, "this is not valid mermaid") {
		t.Fatalf("expected invalid line to be dropped, got:\n%s", got)
	}
	if !isMermaidDiagramValid(got) {
		t.Fatalf("expected sanitized diagram to be valid, got:\n%s", got)
	}
}
