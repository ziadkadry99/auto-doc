package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ziadkadry99/auto-doc/internal/config"
	bizctx "github.com/ziadkadry99/auto-doc/internal/context"
	"github.com/ziadkadry99/auto-doc/internal/docs"
	"github.com/ziadkadry99/auto-doc/internal/indexer"
	"github.com/ziadkadry99/auto-doc/internal/llm"
	"github.com/ziadkadry99/auto-doc/internal/progress"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
	"github.com/ziadkadry99/auto-doc/internal/walker"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate documentation and vector index for the codebase",
	Long:  `Scans the entire codebase, generates AI-powered documentation, and builds a semantic vector database.`,
	RunE:  runGenerate,
}

func init() {
	generateCmd.Flags().Bool("dry-run", false, "estimate costs without making API calls")
	generateCmd.Flags().Int("concurrency", 0, "max parallel LLM calls (overrides config)")
	generateCmd.Flags().Bool("interactive", false, "collect business context interactively")
	generateCmd.Flags().String("context-file", "", "path to a business context JSON file")
	rootCmd.AddCommand(generateCmd)
}

func runGenerate(cmd *cobra.Command, args []string) error {
	start := time.Now()
	ctx := context.Background()

	// Load config.
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Override concurrency from flag if provided.
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	if concurrency > 0 {
		cfg.MaxConcurrency = concurrency
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	interactive, _ := cmd.Flags().GetBool("interactive")
	contextFile, _ := cmd.Flags().GetString("context-file")

	// Collect or load business context.
	var businessCtx *bizctx.BusinessContext

	if interactive {
		collected, err := bizctx.CollectInteractive()
		if err != nil {
			return fmt.Errorf("collecting business context: %w", err)
		}
		if collected != nil && !collected.IsEmpty() {
			businessCtx = collected
			savePath := filepath.Join(".autodoc", "context.json")
			if err := businessCtx.Save(savePath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save context: %v\n", err)
			} else if verbose {
				fmt.Fprintf(os.Stderr, "Business context saved to %s\n", savePath)
			}
		}
	}

	// Load from explicit flag or auto-load from default path.
	if businessCtx == nil {
		loadPath := contextFile
		if loadPath == "" {
			loadPath = filepath.Join(".autodoc", "context.json")
		}
		loaded, err := bizctx.Load(loadPath)
		if err != nil {
			// Only error if the user explicitly provided a path.
			if contextFile != "" {
				return fmt.Errorf("loading context file: %w", err)
			}
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: could not load context: %v\n", err)
			}
		}
		if loaded != nil {
			businessCtx = loaded
			if verbose {
				fmt.Fprintf(os.Stderr, "Loaded business context from %s\n", loadPath)
			}
		}
	}

	// Get the working directory as root for walking.
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Walk codebase.
	if verbose {
		fmt.Fprintf(os.Stderr, "Scanning files in %s...\n", rootDir)
	}

	files, err := walker.Walk(walker.WalkerConfig{
		RootDir:     rootDir,
		Include:     cfg.Include,
		Exclude:     cfg.Exclude,
		MaxFileSize: 0, // Use default.
	})
	if err != nil {
		return fmt.Errorf("walking codebase: %w", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Found %d files to process\n", len(files))
	}

	if len(files) == 0 {
		fmt.Println("No files found to document.")
		return nil
	}

	// Initialize LLM provider.
	llmProvider, err := llm.NewProvider(string(cfg.Provider), cfg.Model)
	if err != nil {
		return fmt.Errorf("creating LLM provider: %w", err)
	}

	// Initialize embedder.
	embedder, err := createEmbedderFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating embedder: %w", err)
	}

	// Initialize vector store.
	store, err := vectordb.NewChromemStore(embedder)
	if err != nil {
		return fmt.Errorf("creating vector store: %w", err)
	}

	// Try to load existing vector store (ignore error for fresh generate).
	vectorDir := filepath.Join(cfg.OutputDir, "vectordb")
	if err := store.Load(ctx, vectorDir); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "No existing vector store found (fresh generate): %v\n", err)
		}
	}

	// Create pipeline.
	pipeline := indexer.NewPipeline(llmProvider, embedder, store, cfg, rootDir)

	// Handle dry-run mode.
	if dryRun {
		estimate, err := pipeline.DryRun(ctx, files)
		if err != nil {
			return fmt.Errorf("dry run failed: %w", err)
		}
		printCostEstimate(estimate, cfg)
		return nil
	}

	// Set up progress reporting.
	reporter := progress.NewReporter()
	reporter.Start(len(files))
	pipeline.SetProgressFunc(func(processed int, total int, currentFile string) {
		reporter.Update(processed, currentFile)
	})

	// Run the pipeline.
	result, err := pipeline.Run(ctx, files)
	reporter.Finish()
	if err != nil {
		return fmt.Errorf("pipeline failed: %w", err)
	}

	// Save analyses for dependency-aware incremental updates.
	if len(result.Analyses) > 0 {
		if err := indexer.SaveAnalyses(rootDir, result.Analyses); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save analyses cache: %v\n", err)
		} else if verbose {
			fmt.Fprintf(os.Stderr, "Saved %d analyses to .autodoc/analyses.json\n", len(result.Analyses))
		}
	}

	// Generate markdown documentation.
	docGen := docs.NewDocGenerator(cfg.OutputDir)
	docGen.BusinessContext = businessCtx

	// Collect analyses from the pipeline results for doc generation.
	// We need to re-walk files to collect analyses that were already stored.
	// The pipeline stores documents in the vector store, but for doc generation
	// we need the original FileAnalysis objects. For now we re-use what was processed.
	// The pipeline result doesn't carry analyses directly, so we generate docs
	// from the vector store content.
	if verbose {
		fmt.Fprintf(os.Stderr, "Generating markdown documentation...\n")
	}

	// Generate documentation for all tiers.
	allDocs, err := getAllFileAnalyses(ctx, store, files)
	if err == nil && len(allDocs) > 0 {
		if err := docGen.GenerateFileDocs(allDocs); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate file docs: %v\n", err)
		}

		// Enhanced index with LLM-generated overview and features (all tiers).
		if verbose {
			fmt.Fprintf(os.Stderr, "Generating enhanced home page...\n")
		}
		if err := docGen.GenerateEnhancedIndex(ctx, allDocs, llmProvider, cfg.Model); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: enhanced index generation failed, falling back to basic index: %v\n", err)
			if err := docGen.GenerateIndex(allDocs); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to generate index: %v\n", err)
			}
		}

		// Architecture overview for Normal and Max tiers only.
		if cfg.Quality != config.QualityLite {
			if verbose {
				fmt.Fprintf(os.Stderr, "Generating architecture overview...\n")
			}
			if err := docGen.GenerateArchitecture(ctx, allDocs, llmProvider, cfg.Model); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: architecture generation failed: %v\n", err)
			} else {
				// Index the architecture doc into the vector store.
				archPath := filepath.Join(cfg.OutputDir, "docs", "architecture.md")
				if archDocs, archErr := indexArchitecture(archPath); archErr == nil && len(archDocs) > 0 {
					if err := store.AddDocuments(ctx, archDocs); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to index architecture doc: %v\n", err)
					} else if verbose {
						fmt.Fprintf(os.Stderr, "Indexed %d architecture sections into vector store\n", len(archDocs))
					}
				}
			}
		}
	}

	// Persist the vector store (after architecture indexing).
	if err := store.Persist(ctx, vectorDir); err != nil {
		return fmt.Errorf("persisting vector store: %w", err)
	}

	// Print summary.
	duration := time.Since(start)
	fmt.Println()
	fmt.Println("Documentation generation complete!")
	fmt.Printf("  Files processed: %d\n", result.FilesProcessed)
	fmt.Printf("  Files skipped:   %d (unchanged)\n", result.FilesSkipped)
	fmt.Printf("  Files failed:    %d\n", result.FilesFailed)
	fmt.Printf("  Tokens used:     %d input, %d output\n", result.TotalInputTokens, result.TotalOutputTokens)

	cost := llm.EstimateCost(cfg.Model, result.TotalInputTokens, result.TotalOutputTokens)
	if cost > 0 {
		fmt.Printf("  Estimated cost:  $%.4f\n", cost)
	}
	fmt.Printf("  Duration:        %s\n", duration.Round(time.Millisecond))
	fmt.Printf("  Output:          %s\n", cfg.OutputDir)

	if len(result.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "\nWarnings (%d):\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  - %v\n", e)
		}
	}

	return nil
}

// getAllFileAnalyses reconstructs minimal FileAnalysis objects from the vector store.
func getAllFileAnalyses(ctx context.Context, store vectordb.VectorStore, files []walker.FileInfo) ([]indexer.FileAnalysis, error) {
	var analyses []indexer.FileAnalysis
	for _, f := range files {
		docs, err := store.GetByFilePath(ctx, f.RelPath)
		if err != nil || len(docs) == 0 {
			continue
		}

		analysis := indexer.FileAnalysis{
			FilePath:    f.RelPath,
			Language:    f.Language,
			ContentHash: f.ContentHash,
		}

		// Extract summary from the file-level document.
		for _, doc := range docs {
			if doc.Metadata.Type == vectordb.DocTypeFile {
				analysis.Summary = doc.Content
			}
		}

		analyses = append(analyses, analysis)
	}
	return analyses, nil
}

// indexArchitecture reads the architecture.md file and creates vector DB documents from its sections.
func indexArchitecture(archPath string) ([]vectordb.Document, error) {
	content, err := os.ReadFile(archPath)
	if err != nil {
		return nil, fmt.Errorf("reading architecture doc: %w", err)
	}

	fullContent := string(content)
	now := time.Now()
	var docs []vectordb.Document

	// Create a full-overview document (first 2000 chars).
	overview := fullContent
	if len(overview) > 2000 {
		overview = overview[:2000]
	}
	docs = append(docs, vectordb.Document{
		ID:      "arch:overview",
		Content: overview,
		Metadata: vectordb.DocumentMetadata{
			FilePath:    "architecture.md",
			Type:        vectordb.DocTypeArchitecture,
			LastUpdated: now,
		},
	})

	// Split by ## headers into sections and create one document per section.
	sections := splitMarkdownSections(fullContent)
	for _, sec := range sections {
		if strings.TrimSpace(sec.content) == "" {
			continue
		}
		id := fmt.Sprintf("arch:%s", strings.ToLower(strings.ReplaceAll(strings.TrimSpace(sec.title), " ", "-")))
		docs = append(docs, vectordb.Document{
			ID:      id,
			Content: fmt.Sprintf("## %s\n\n%s", sec.title, sec.content),
			Metadata: vectordb.DocumentMetadata{
				FilePath:    "architecture.md",
				Symbol:      sec.title,
				Type:        vectordb.DocTypeArchitecture,
				LastUpdated: now,
			},
		})
	}

	return docs, nil
}

type markdownSection struct {
	title   string
	content string
}

// splitMarkdownSections splits markdown content by ## headers.
func splitMarkdownSections(content string) []markdownSection {
	lines := strings.Split(content, "\n")
	var sections []markdownSection
	var currentTitle string
	var currentLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// Save previous section if any.
			if currentTitle != "" {
				sections = append(sections, markdownSection{
					title:   currentTitle,
					content: strings.Join(currentLines, "\n"),
				})
			}
			currentTitle = strings.TrimPrefix(line, "## ")
			currentLines = nil
		} else if currentTitle != "" {
			currentLines = append(currentLines, line)
		}
	}
	// Save last section.
	if currentTitle != "" {
		sections = append(sections, markdownSection{
			title:   currentTitle,
			content: strings.Join(currentLines, "\n"),
		})
	}

	return sections
}

// printCostEstimate displays cost estimate results.
func printCostEstimate(estimate *indexer.CostEstimate, cfg *config.Config) {
	fmt.Println("Cost Estimate (dry run)")
	fmt.Println("=======================")
	fmt.Printf("  Files to process:    %d\n", estimate.TotalFiles)
	fmt.Printf("  Estimated tokens:    %d\n", estimate.TotalTokensEstimate)
	fmt.Printf("  Estimated total:     $%.4f\n", estimate.EstimatedCost)
	fmt.Println()
	fmt.Println("  Breakdown:")
	for op, cost := range estimate.CostBreakdown {
		fmt.Printf("    %-20s $%.4f\n", op, cost)
	}
	fmt.Println()
	fmt.Printf("  Quality tier:        %s\n", cfg.Quality)
	fmt.Printf("  Model:               %s\n", cfg.Model)
}
