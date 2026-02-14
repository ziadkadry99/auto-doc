package vectordb

import (
	"fmt"
	"strings"
)

// FormatResults renders search results as human-readable text.
func FormatResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No results found."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d result(s):\n\n", len(results)))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("--- Result %d (similarity: %.4f) ---\n", i+1, r.Similarity))

		if r.Document.Metadata.FilePath != "" {
			location := r.Document.Metadata.FilePath
			if r.Document.Metadata.LineStart > 0 {
				location += fmt.Sprintf(":%d", r.Document.Metadata.LineStart)
				if r.Document.Metadata.LineEnd > r.Document.Metadata.LineStart {
					location += fmt.Sprintf("-%d", r.Document.Metadata.LineEnd)
				}
			}
			sb.WriteString(fmt.Sprintf("File: %s\n", location))
		}

		if r.Document.Metadata.Type != "" {
			sb.WriteString(fmt.Sprintf("Type: %s\n", r.Document.Metadata.Type))
		}
		if r.Document.Metadata.Symbol != "" {
			sb.WriteString(fmt.Sprintf("Symbol: %s\n", r.Document.Metadata.Symbol))
		}
		if r.Document.Metadata.Language != "" {
			sb.WriteString(fmt.Sprintf("Language: %s\n", r.Document.Metadata.Language))
		}

		sb.WriteString("\n")
		sb.WriteString(r.Document.Content)
		sb.WriteString("\n\n")
	}

	return sb.String()
}
