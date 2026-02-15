package diagrams

import (
	"fmt"
	"strings"
)

// Component represents a node in an architecture diagram.
type Component struct {
	Name        string
	Description string
}

// Relationship represents an edge between two components.
type Relationship struct {
	From  string
	To    string
	Label string
}

// ArchitectureDiagram generates a mermaid graph TD diagram from components and relationships.
func ArchitectureDiagram(components []Component, relationships []Relationship) string {
	var b strings.Builder
	b.WriteString("graph TD\n")

	for _, c := range components {
		id := sanitizeID(c.Name)
		if c.Description != "" {
			b.WriteString(fmt.Sprintf("    %s[\"%s<br/>%s\"]\n", id, escapeMermaid(c.Name), escapeMermaid(c.Description)))
		} else {
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, escapeMermaid(c.Name)))
		}
	}

	for _, r := range relationships {
		fromID := sanitizeID(r.From)
		toID := sanitizeID(r.To)
		if r.Label != "" {
			b.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", fromID, escapeMermaid(r.Label), toID))
		} else {
			b.WriteString(fmt.Sprintf("    %s --> %s\n", fromID, toID))
		}
	}

	return b.String()
}

// DependencyDiagram generates a mermaid dependency graph from a map of file to dependencies.
func DependencyDiagram(deps map[string][]string) string {
	var b strings.Builder
	b.WriteString("graph LR\n")

	for file, fileDeps := range deps {
		fromID := sanitizeID(file)
		b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", fromID, escapeMermaid(file)))
		for _, dep := range fileDeps {
			depID := sanitizeID(dep)
			b.WriteString(fmt.Sprintf("    %s --> %s[\"%s\"]\n", fromID, depID, escapeMermaid(dep)))
		}
	}

	return b.String()
}

// sanitizeID converts a string into a safe mermaid node ID.
func sanitizeID(s string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		".", "_",
		"-", "_",
		" ", "_",
		"(", "_",
		")", "_",
		"[", "_",
		"]", "_",
		"{", "_",
		"}", "_",
		":", "_",
	)
	return replacer.Replace(s)
}

// escapeMermaid escapes characters that have special meaning in mermaid labels.
func escapeMermaid(s string) string {
	s = strings.ReplaceAll(s, "\"", "#quot;")
	s = strings.ReplaceAll(s, "(", "#lpar;")
	s = strings.ReplaceAll(s, ")", "#rpar;")
	s = strings.ReplaceAll(s, "[", "#lsqb;")
	s = strings.ReplaceAll(s, "]", "#rsqb;")
	s = strings.ReplaceAll(s, "{", "#lbrace;")
	s = strings.ReplaceAll(s, "}", "#rbrace;")
	s = strings.ReplaceAll(s, "<", "#lt;")
	s = strings.ReplaceAll(s, ">", "#gt;")
	return s
}
