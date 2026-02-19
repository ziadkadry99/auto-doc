package diagrams

import (
	"encoding/json"
	"strings"
)

// DiagramData is the JSON-serializable structure for architecture/dependency diagrams.
type DiagramData struct {
	Nodes []DiagramNode `json:"nodes"`
	Edges []DiagramEdge `json:"edges"`
}

// DiagramNode represents a box in the rendered diagram.
type DiagramNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Desc  string `json:"desc,omitempty"`
	Group string `json:"group,omitempty"`
}

// DiagramEdge represents an arrow between two nodes.
type DiagramEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

// Component represents a node in an architecture diagram.
type Component struct {
	Name        string
	Description string
	Group       string // Optional layer/group name for visual grouping.
}

// Relationship represents an edge between two components.
type Relationship struct {
	From  string
	To    string
	Label string
}

// ArchitectureDiagram returns a JSON string encoding a DiagramData structure
// from the given components and relationships. The JSON is rendered to SVG
// by the site's JavaScript renderer.
func ArchitectureDiagram(components []Component, relationships []Relationship) string {
	data := DiagramData{}
	for _, c := range components {
		data.Nodes = append(data.Nodes, DiagramNode{
			ID:    sanitizeID(c.Name),
			Label: c.Name,
			Desc:  c.Description,
			Group: c.Group,
		})
	}
	for _, r := range relationships {
		data.Edges = append(data.Edges, DiagramEdge{
			From:  sanitizeID(r.From),
			To:    sanitizeID(r.To),
			Label: r.Label,
		})
	}
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(b)
}

// DependencyDiagram returns a JSON string encoding a DiagramData structure
// from the given file-to-dependencies map.
func DependencyDiagram(deps map[string][]string) string {
	data := DiagramData{}
	seen := make(map[string]bool)
	for file, fileDeps := range deps {
		fileID := sanitizeID(file)
		if !seen[fileID] {
			data.Nodes = append(data.Nodes, DiagramNode{ID: fileID, Label: file})
			seen[fileID] = true
		}
		for _, dep := range fileDeps {
			depID := sanitizeID(dep)
			if !seen[depID] {
				data.Nodes = append(data.Nodes, DiagramNode{ID: depID, Label: dep})
				seen[depID] = true
			}
			data.Edges = append(data.Edges, DiagramEdge{From: fileID, To: depID})
		}
	}
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(b)
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
