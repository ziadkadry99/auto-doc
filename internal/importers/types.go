package importers

import "time"

// SourceType identifies the kind of external source.
type SourceType string

const (
	SourceConfluence SourceType = "confluence"
	SourceReadme     SourceType = "readme"
	SourceADR        SourceType = "adr"
	SourceOpenAPI    SourceType = "openapi"
	SourceAsyncAPI   SourceType = "asyncapi"
)

// ImportSource represents a configured external documentation source.
type ImportSource struct {
	ID           string     `json:"id"`
	Type         SourceType `json:"type"`
	Name         string     `json:"name"`
	Config       string     `json:"config"` // JSON config specific to source type
	LastImported *time.Time `json:"last_imported,omitempty"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
}

// ImportResult contains the results of an import operation.
type ImportResult struct {
	SourceID       string           `json:"source_id"`
	SourceType     SourceType       `json:"source_type"`
	ItemsFound     int              `json:"items_found"`
	ItemsImported  int              `json:"items_imported"`
	ItemsStale     int              `json:"items_stale"`
	Items          []ImportedItem   `json:"items"`
	Errors         []string         `json:"errors,omitempty"`
}

// ImportedItem is a single piece of content extracted from an external source.
type ImportedItem struct {
	Title        string    `json:"title"`
	Content      string    `json:"content"`
	SourceURL    string    `json:"source_url,omitempty"`
	LastModified time.Time `json:"last_modified"`
	Stale        bool      `json:"stale"`
	StaleReason  string    `json:"stale_reason,omitempty"`
}

// ReadmeSection represents a parsed section from a README file.
type ReadmeSection struct {
	Heading string `json:"heading"`
	Content string `json:"content"`
	Level   int    `json:"level"` // heading level (1-6)
}

// ADR represents a parsed Architecture Decision Record.
type ADR struct {
	Number       int    `json:"number"`
	Title        string `json:"title"`
	Status       string `json:"status"` // "accepted", "deprecated", "superseded"
	Context      string `json:"context"`
	Decision     string `json:"decision"`
	Consequences string `json:"consequences"`
	Date         string `json:"date,omitempty"`
	FilePath     string `json:"file_path"`
}

// OpenAPIEndpoint represents a parsed endpoint from an OpenAPI spec.
type OpenAPIEndpoint struct {
	Path        string            `json:"path"`
	Method      string            `json:"method"`
	Summary     string            `json:"summary"`
	Description string            `json:"description"`
	Parameters  []string          `json:"parameters,omitempty"`
	RequestBody string            `json:"request_body,omitempty"`
	Responses   map[string]string `json:"responses,omitempty"`
}
