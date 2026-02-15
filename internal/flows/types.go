package flows

import "time"

// Flow represents a cross-service flow narrative.
type Flow struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Narrative      string    `json:"narrative"`
	MermaidDiagram string    `json:"mermaid_diagram"`
	Services       []string  `json:"services"`
	EntryPoint     string    `json:"entry_point"`
	ExitPoint      string    `json:"exit_point"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// FlowStep represents a single step within a flow.
type FlowStep struct {
	Order       int    `json:"order"`
	Service     string `json:"service"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

// CrossServiceCall represents a detected cross-service communication pattern.
type CrossServiceCall struct {
	Type     string `json:"type"`
	Target   string `json:"target"`
	Method   string `json:"method"`
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
}
