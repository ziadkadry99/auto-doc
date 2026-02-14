package vectordb

import "time"

// DocumentType categorizes the kind of document stored in the vector DB.
type DocumentType string

const (
	DocTypeFile         DocumentType = "file"
	DocTypeFunction     DocumentType = "function"
	DocTypeClass        DocumentType = "class"
	DocTypeModule       DocumentType = "module"
	DocTypeArchitecture DocumentType = "architecture"
)

// Document represents a piece of content to be stored and searched.
type Document struct {
	ID       string
	Content  string
	Metadata DocumentMetadata
}

// DocumentMetadata holds structured information about a document.
type DocumentMetadata struct {
	FilePath    string
	LineStart   int
	LineEnd     int
	ContentHash string
	Type        DocumentType
	Language    string
	Symbol      string
	RepoID      string // For future Phase 4 central server support.
	LastUpdated time.Time
}

// SearchResult pairs a document with its similarity score.
type SearchResult struct {
	Document   Document
	Similarity float32
}

// SearchFilter allows narrowing search results by metadata fields.
type SearchFilter struct {
	Type     *DocumentType
	FilePath *string
	Language *string
}
