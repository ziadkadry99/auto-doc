package indexer

import (
	"fmt"
	"strings"
	"time"

	"github.com/ziadkadry99/auto-doc/internal/config"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
)

// ChunkAnalysis converts a FileAnalysis into a set of vector store documents.
func ChunkAnalysis(analysis *FileAnalysis, tier config.QualityTier) []vectordb.Document {
	var docs []vectordb.Document
	now := time.Now()

	// File-level summary document.
	var fileSummaryParts []string
	fileSummaryParts = append(fileSummaryParts, fmt.Sprintf("File: %s", analysis.FilePath))
	fileSummaryParts = append(fileSummaryParts, fmt.Sprintf("Language: %s", analysis.Language))
	fileSummaryParts = append(fileSummaryParts, fmt.Sprintf("Summary: %s", analysis.Summary))
	if analysis.Purpose != "" {
		fileSummaryParts = append(fileSummaryParts, fmt.Sprintf("Purpose: %s", analysis.Purpose))
	}
	if len(analysis.Dependencies) > 0 {
		var deps []string
		for _, d := range analysis.Dependencies {
			deps = append(deps, fmt.Sprintf("%s (%s)", d.Name, d.Type))
		}
		fileSummaryParts = append(fileSummaryParts, fmt.Sprintf("Dependencies: %s", strings.Join(deps, ", ")))
	}
	if len(analysis.KeyLogic) > 0 {
		fileSummaryParts = append(fileSummaryParts, fmt.Sprintf("Key Logic: %s", strings.Join(analysis.KeyLogic, "; ")))
	}

	docs = append(docs, vectordb.Document{
		ID:      fmt.Sprintf("file:%s", analysis.FilePath),
		Content: strings.Join(fileSummaryParts, "\n"),
		Metadata: vectordb.DocumentMetadata{
			FilePath:    analysis.FilePath,
			ContentHash: analysis.ContentHash,
			Type:        vectordb.DocTypeFile,
			Language:    analysis.Language,
			LastUpdated: now,
		},
	})

	// Dependency document: lists service-level dependencies for blast-radius queries.
	// Only created when the file has meaningful service-level deps (gRPC, API calls, etc.),
	// not for trivial imports or shell commands.
	if len(analysis.Dependencies) > 0 {
		var serviceDeps []Dependency
		for _, d := range analysis.Dependencies {
			if isServiceLevelDep(d) {
				serviceDeps = append(serviceDeps, d)
			}
		}
		if len(serviceDeps) > 0 {
			var depParts []string
			depParts = append(depParts, fmt.Sprintf("File: %s", analysis.FilePath))
			depParts = append(depParts, fmt.Sprintf("Language: %s", analysis.Language))
			depParts = append(depParts, "Service dependencies and blast radius:")
			for _, d := range serviceDeps {
				depParts = append(depParts, fmt.Sprintf("- Depends on %s (%s). Changes to %s may affect %s.", d.Name, d.Type, d.Name, analysis.FilePath))
			}
			docs = append(docs, vectordb.Document{
				ID:      fmt.Sprintf("deps:%s", analysis.FilePath),
				Content: strings.Join(depParts, "\n"),
				Metadata: vectordb.DocumentMetadata{
					FilePath:    analysis.FilePath,
					ContentHash: analysis.ContentHash,
					Type:        vectordb.DocTypeFile,
					Language:    analysis.Language,
					Symbol:      "dependencies",
					LastUpdated: now,
				},
			})
		}
	}

	// Function-level documents (Normal and Max tiers).
	if tier != config.QualityLite {
		for _, fn := range analysis.Functions {
			content := buildFunctionContent(analysis.FilePath, fn)
			docs = append(docs, vectordb.Document{
				ID:      fmt.Sprintf("func:%s:%s", analysis.FilePath, fn.Name),
				Content: content,
				Metadata: vectordb.DocumentMetadata{
					FilePath:    analysis.FilePath,
					LineStart:   fn.LineStart,
					LineEnd:     fn.LineEnd,
					ContentHash: analysis.ContentHash,
					Type:        vectordb.DocTypeFunction,
					Language:    analysis.Language,
					Symbol:      fn.Name,
					LastUpdated: now,
				},
			})
		}

		// Class-level documents.
		for _, cls := range analysis.Classes {
			content := buildClassContent(analysis.FilePath, cls)
			docs = append(docs, vectordb.Document{
				ID:      fmt.Sprintf("class:%s:%s", analysis.FilePath, cls.Name),
				Content: content,
				Metadata: vectordb.DocumentMetadata{
					FilePath:    analysis.FilePath,
					LineStart:   cls.LineStart,
					LineEnd:     cls.LineEnd,
					ContentHash: analysis.ContentHash,
					Type:        vectordb.DocTypeClass,
					Language:    analysis.Language,
					Symbol:      cls.Name,
					LastUpdated: now,
				},
			})
		}
	}

	return docs
}

func buildFunctionContent(filePath string, fn FunctionDoc) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Function: %s", fn.Name))
	if fn.Signature != "" {
		parts = append(parts, fmt.Sprintf("Signature: %s", fn.Signature))
	}
	parts = append(parts, fmt.Sprintf("Summary: %s", fn.Summary))
	if len(fn.Parameters) > 0 {
		var params []string
		for _, p := range fn.Parameters {
			params = append(params, fmt.Sprintf("%s (%s): %s", p.Name, p.Type, p.Description))
		}
		parts = append(parts, fmt.Sprintf("Parameters: %s", strings.Join(params, "; ")))
	}
	if fn.Returns != "" {
		parts = append(parts, fmt.Sprintf("Returns: %s", fn.Returns))
	}
	parts = append(parts, fmt.Sprintf("File: %s", filePath))
	return strings.Join(parts, "\n")
}

func buildClassContent(filePath string, cls ClassDoc) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Class/Type: %s", cls.Name))
	parts = append(parts, fmt.Sprintf("Summary: %s", cls.Summary))
	if len(cls.Fields) > 0 {
		var fields []string
		for _, f := range cls.Fields {
			fields = append(fields, fmt.Sprintf("%s (%s): %s", f.Name, f.Type, f.Description))
		}
		parts = append(parts, fmt.Sprintf("Fields: %s", strings.Join(fields, "; ")))
	}
	if len(cls.Methods) > 0 {
		var methods []string
		for _, m := range cls.Methods {
			methods = append(methods, fmt.Sprintf("%s: %s", m.Name, m.Summary))
		}
		parts = append(parts, fmt.Sprintf("Methods: %s", strings.Join(methods, "; ")))
	}
	parts = append(parts, fmt.Sprintf("File: %s", filePath))
	return strings.Join(parts, "\n")
}

// SplitLargeFile splits file content into chunks that fit within a token limit.
// It attempts to split at function/class boundaries by looking for common patterns.
func SplitLargeFile(content string, maxTokens int) []string {
	// Rough estimate: 1 token ~= 4 characters.
	maxChars := maxTokens * 4
	if len(content) <= maxChars {
		return []string{content}
	}

	lines := strings.Split(content, "\n")
	var chunks []string
	var current []string
	currentLen := 0

	for _, line := range lines {
		lineLen := len(line) + 1 // +1 for newline
		if currentLen+lineLen > maxChars && len(current) > 0 {
			chunks = append(chunks, strings.Join(current, "\n"))
			current = nil
			currentLen = 0
		}
		current = append(current, line)
		currentLen += lineLen
	}
	if len(current) > 0 {
		chunks = append(chunks, strings.Join(current, "\n"))
	}
	return chunks
}
