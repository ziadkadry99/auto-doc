package indexer

import "time"

// FileAnalysis holds the complete analysis result for a single source file.
type FileAnalysis struct {
	FilePath     string        `json:"file_path"`
	Language     string        `json:"language"`
	Summary      string        `json:"summary"`
	Purpose      string        `json:"purpose"`
	Functions    []FunctionDoc `json:"functions,omitempty"`
	Classes      []ClassDoc    `json:"classes,omitempty"`
	Dependencies []Dependency  `json:"dependencies,omitempty"`
	KeyLogic     []string      `json:"key_logic,omitempty"`
	ContentHash  string        `json:"content_hash"`
}

// FunctionDoc describes a single function or method found in a file.
type FunctionDoc struct {
	Name       string     `json:"name"`
	Signature  string     `json:"signature"`
	Summary    string     `json:"summary"`
	Parameters []ParamDoc `json:"parameters,omitempty"`
	Returns    string     `json:"returns,omitempty"`
	LineStart  int        `json:"line_start,omitempty"`
	LineEnd    int        `json:"line_end,omitempty"`
}

// ParamDoc describes a function parameter.
type ParamDoc struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ClassDoc describes a class, struct, or interface found in a file.
type ClassDoc struct {
	Name      string        `json:"name"`
	Summary   string        `json:"summary"`
	Methods   []FunctionDoc `json:"methods,omitempty"`
	Fields    []FieldDoc    `json:"fields,omitempty"`
	LineStart int           `json:"line_start,omitempty"`
	LineEnd   int           `json:"line_end,omitempty"`
}

// FieldDoc describes a field within a class or struct.
type FieldDoc struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// Dependency represents an external dependency used by a file.
type Dependency struct {
	Name string `json:"name"`
	Type string `json:"type"` // import, api_call, database, event, etc.
}

// PipelineResult summarizes the outcome of a full indexing run.
type PipelineResult struct {
	FilesProcessed    int
	FilesSkipped      int
	FilesFailed       int
	TotalInputTokens  int
	TotalOutputTokens int
	EstimatedCost     float64
	Duration          time.Duration
	Errors            []error
	Analyses          map[string]FileAnalysis
}

// CostEstimate provides a cost breakdown without making API calls.
type CostEstimate struct {
	TotalFiles          int
	TotalTokensEstimate int
	EstimatedCost       float64
	CostBreakdown       map[string]float64 // per operation: analysis, embeddings, architecture
}

// ProgressFunc is called during batch processing to report progress.
type ProgressFunc func(processed int, total int, currentFile string)
