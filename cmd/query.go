package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ziadkadry99/auto-doc/internal/vectordb"
)

var queryCmd = &cobra.Command{
	Use:   "query [question]",
	Short: "Semantically search the codebase documentation",
	Long:  `Searches the vector database using a natural language query and returns relevant files, functions, and context.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runQuery,
}

func init() {
	queryCmd.Flags().Int("limit", 10, "maximum number of results")
	queryCmd.Flags().String("type", "", "filter by type: file, function, class, architecture")
	queryCmd.Flags().Bool("json", false, "output results as JSON")
	rootCmd.AddCommand(queryCmd)
}

func runQuery(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	queryText := args[0]

	limit, _ := cmd.Flags().GetInt("limit")
	typeFilter, _ := cmd.Flags().GetString("type")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// Load config.
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Create embedder for query embedding.
	embedder, err := createEmbedderFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating embedder: %w", err)
	}

	// Create and load vector store.
	store, err := vectordb.NewChromemStore(embedder)
	if err != nil {
		return fmt.Errorf("creating vector store: %w", err)
	}

	vectorDir := filepath.Join(cfg.OutputDir, "vectordb")
	if err := store.Load(ctx, vectorDir); err != nil {
		return fmt.Errorf("loading vector store from %s: %w\nRun `autodoc generate` first to build the index", vectorDir, err)
	}

	if store.Count() == 0 {
		fmt.Println("Vector store is empty. Run `autodoc generate` first.")
		return nil
	}

	// Build search filter.
	var filter *vectordb.SearchFilter
	if typeFilter != "" {
		docType := vectordb.DocumentType(typeFilter)
		filter = &vectordb.SearchFilter{Type: &docType}
	}

	// Search.
	results, err := store.Search(ctx, queryText, limit, filter)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	// Output results.
	if jsonOutput {
		return printQueryResultsJSON(results)
	}

	printQueryResultsTable(results)
	return nil
}

type queryResultJSON struct {
	Rank       int     `json:"rank"`
	Similarity float64 `json:"similarity"`
	FilePath   string  `json:"file_path"`
	LineStart  int     `json:"line_start,omitempty"`
	Type       string  `json:"type"`
	Symbol     string  `json:"symbol,omitempty"`
	Summary    string  `json:"summary"`
}

func printQueryResultsJSON(results []vectordb.SearchResult) error {
	var out []queryResultJSON
	for i, r := range results {
		out = append(out, queryResultJSON{
			Rank:       i + 1,
			Similarity: float64(r.Similarity),
			FilePath:   r.Document.Metadata.FilePath,
			LineStart:  r.Document.Metadata.LineStart,
			Type:       string(r.Document.Metadata.Type),
			Symbol:     r.Document.Metadata.Symbol,
			Summary:    truncate(r.Document.Content, 200),
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func printQueryResultsTable(results []vectordb.SearchResult) {
	fmt.Printf("Found %d results:\n\n", len(results))
	for i, r := range results {
		location := r.Document.Metadata.FilePath
		if r.Document.Metadata.LineStart > 0 {
			location = fmt.Sprintf("%s:%d", location, r.Document.Metadata.LineStart)
		}

		symbol := ""
		if r.Document.Metadata.Symbol != "" {
			symbol = fmt.Sprintf(" (%s)", r.Document.Metadata.Symbol)
		}

		fmt.Printf("  %d. [%.1f%%] %s%s\n", i+1, r.Similarity*100, location, symbol)
		fmt.Printf("     Type: %s\n", r.Document.Metadata.Type)
		fmt.Printf("     %s\n\n", truncate(r.Document.Content, 120))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
