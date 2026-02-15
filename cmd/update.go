package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/ziadkadry99/auto-doc/internal/config"
	"github.com/ziadkadry99/auto-doc/internal/docs"
	"github.com/ziadkadry99/auto-doc/internal/indexer"
	"github.com/ziadkadry99/auto-doc/internal/llm"
	"github.com/ziadkadry99/auto-doc/internal/progress"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
	"github.com/ziadkadry99/auto-doc/internal/walker"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Incrementally update documentation from git diff",
	Long:  `Detects changed files since the last index and updates only the affected documentation and vector entries.`,
	RunE:  runUpdate,
}

func init() {
	updateCmd.Flags().Bool("force", false, "skip git diff and re-process all files")
	updateCmd.Flags().Int("concurrency", 0, "max parallel LLM calls (overrides config)")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
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

	force, _ := cmd.Flags().GetBool("force")

	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Load existing state.
	state, err := indexer.LoadState(rootDir)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	// If no state exists (no LastCommitSHA and no file hashes), advise user to run generate first.
	if state.LastCommitSHA == "" && len(state.FileHashes) == 0 {
		fmt.Println("No existing index found. Run `autodoc generate` first to create the initial index.")
		return nil
	}

	// Load stored analyses for dependency expansion.
	storedAnalyses, err := indexer.LoadAnalyses(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load analyses cache: %v\n", err)
		storedAnalyses = make(map[string]indexer.FileAnalysis)
	}

	// Determine changed files.
	var modified, added, deleted []string
	if force {
		if verbose {
			fmt.Fprintf(os.Stderr, "Force mode: re-processing all files\n")
		}
	} else {
		modified, added, deleted, err = indexer.GetGitChangedFiles(rootDir, state.LastCommitSHA)
		if err != nil {
			return fmt.Errorf("detecting git changes: %w", err)
		}

		totalChanges := len(modified) + len(added) + len(deleted)
		if totalChanges == 0 {
			fmt.Println("No changes since last index.")
			return nil
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "Changes detected: %d modified, %d added, %d deleted\n",
				len(modified), len(added), len(deleted))
		}
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

	// Load existing vector store.
	vectorDir := filepath.Join(cfg.OutputDir, "vectordb")
	if err := store.Load(ctx, vectorDir); err != nil {
		return fmt.Errorf("loading vector store (run `autodoc generate` first): %w", err)
	}

	// Handle deleted files: remove docs, vector entries, and analyses.
	deletedCount := 0
	for _, filePath := range deleted {
		// Remove vector store entries.
		if err := store.DeleteByFilePath(ctx, filePath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete vector entries for %s: %v\n", filePath, err)
		}

		// Remove markdown doc file.
		docPath := filepath.Join(cfg.OutputDir, "docs", filePath+".md")
		if err := os.Remove(docPath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove doc %s: %v\n", docPath, err)
		}

		// Remove from state and analyses.
		delete(state.FileHashes, filePath)
		delete(storedAnalyses, filePath)
		deletedCount++
	}

	// Expand changed files via dependency graph (skip in force mode).
	var directlyChanged, depAffected []string
	var filesToProcess []walker.FileInfo

	if force {
		// Walk entire codebase for force mode.
		allFiles, err := walker.Walk(walker.WalkerConfig{
			RootDir:     rootDir,
			Include:     cfg.Include,
			Exclude:     cfg.Exclude,
			MaxFileSize: 0,
		})
		if err != nil {
			return fmt.Errorf("walking codebase: %w", err)
		}
		filesToProcess = allFiles
	} else {
		// Collect directly changed file paths.
		directlyChanged = append(directlyChanged, modified...)
		directlyChanged = append(directlyChanged, added...)

		// Expand via dependency graph.
		expandedPaths, affected := indexer.ExpandChangedFiles(directlyChanged, storedAnalyses)
		depAffected = affected

		if len(depAffected) > 0 {
			fmt.Printf("Dependency analysis: %d directly changed files affect %d additional files\n",
				len(directlyChanged), len(depAffected))
			if verbose {
				for _, f := range depAffected {
					fmt.Fprintf(os.Stderr, "  dep-affected: %s\n", f)
				}
			}
		}

		// Build set of expanded paths for filtering.
		expandedSet := make(map[string]bool)
		for _, f := range expandedPaths {
			expandedSet[f] = true
		}

		// Walk the codebase and filter to expanded files.
		allFiles, err := walker.Walk(walker.WalkerConfig{
			RootDir:     rootDir,
			Include:     cfg.Include,
			Exclude:     cfg.Exclude,
			MaxFileSize: 0,
		})
		if err != nil {
			return fmt.Errorf("walking codebase: %w", err)
		}

		for _, f := range allFiles {
			if expandedSet[f.RelPath] {
				filesToProcess = append(filesToProcess, f)
			}
		}
	}

	// Process changed files through the pipeline.
	updatedCount := 0
	var totalInputTokens, totalOutputTokens int
	var pipelineErrors []error

	if len(filesToProcess) > 0 {
		if verbose {
			fmt.Fprintf(os.Stderr, "Processing %d files...\n", len(filesToProcess))
		}

		pipelineConcurrency := cfg.MaxConcurrency
		if pipelineConcurrency < 1 {
			pipelineConcurrency = 4
		}
		analyzer := indexer.NewFileAnalyzer(llmProvider, cfg.Quality, cfg.Model)

		// Set up progress reporting.
		reporter := progress.NewReporter()
		reporter.Start(len(filesToProcess))
		batcher := indexer.NewBatcher(pipelineConcurrency, analyzer, func(processed int, total int, currentFile string) {
			reporter.Update(processed, currentFile)
		})

		batchResult := batcher.ProcessFiles(ctx, filesToProcess)
		reporter.Finish()

		pipelineErrors = append(pipelineErrors, batchResult.Errors...)
		totalInputTokens = batchResult.InputTokens
		totalOutputTokens = batchResult.OutputTokens

		// Chunk, embed, and store each analysis.
		for _, ar := range batchResult.Results {
			chunks := indexer.ChunkAnalysis(ar.Analysis, cfg.Quality)

			// Delete old documents for this file before adding new ones.
			if err := store.DeleteByFilePath(ctx, ar.Analysis.FilePath); err != nil {
				pipelineErrors = append(pipelineErrors, fmt.Errorf("delete old docs for %s: %w", ar.Analysis.FilePath, err))
				continue
			}

			if err := store.AddDocuments(ctx, chunks); err != nil {
				pipelineErrors = append(pipelineErrors, fmt.Errorf("store docs for %s: %w", ar.Analysis.FilePath, err))
				continue
			}

			state.FileHashes[ar.Analysis.FilePath] = ar.Analysis.ContentHash
			// Merge new analysis into stored analyses.
			storedAnalyses[ar.Analysis.FilePath] = *ar.Analysis
			updatedCount++
		}
	}

	// Save updated analyses.
	if err := indexer.SaveAnalyses(rootDir, storedAnalyses); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save analyses cache: %v\n", err)
	}

	// Persist the vector store.
	if err := store.Persist(ctx, vectorDir); err != nil {
		return fmt.Errorf("persisting vector store: %w", err)
	}

	// Determine which high-level docs to regenerate.
	var regenAdvice *indexer.RegenerationAdvice
	if !force && (updatedCount > 0 || deletedCount > 0) {
		if verbose {
			fmt.Fprintf(os.Stderr, "Asking LLM which docs need regeneration...\n")
		}
		regenAdvice, err = indexer.DecideRegeneration(ctx, llmProvider, cfg.Model, directlyChanged, depAffected, storedAnalyses)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: regeneration decision failed, regenerating all: %v\n", err)
			regenAdvice = nil // will fall through to regenerate everything
		}
	}

	// Walk all files for doc regeneration.
	allFiles, err := walker.Walk(walker.WalkerConfig{
		RootDir:     rootDir,
		Include:     cfg.Include,
		Exclude:     cfg.Exclude,
		MaxFileSize: 0,
	})
	if err != nil {
		return fmt.Errorf("walking codebase for doc regen: %w", err)
	}

	docGen := docs.NewDocGenerator(cfg.OutputDir)

	allDocs, err := getAllFileAnalyses(ctx, store, allFiles)
	if err == nil && len(allDocs) > 0 {
		// Regenerate file docs for updated files.
		if updatedCount > 0 || deletedCount > 0 {
			if err := docGen.GenerateFileDocs(allDocs); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to generate file docs: %v\n", err)
			}
		}

		// Conditionally regenerate high-level docs based on LLM advice.
		shouldRegenOverview := force || regenAdvice == nil || regenAdvice.ProjectOverview
		shouldRegenArch := force || regenAdvice == nil || regenAdvice.Architecture
		// FeaturePages and ComponentMap are currently part of the enhanced index generation.
		// We use overview as a proxy since they're generated together.

		if shouldRegenOverview {
			fmt.Println("Regenerating project overview...")
			if err := docGen.GenerateEnhancedIndex(ctx, allDocs, llmProvider, cfg.Model); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: enhanced index regeneration failed: %v\n", err)
				if err := docGen.GenerateIndex(allDocs); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to generate index: %v\n", err)
				}
			}
		} else {
			fmt.Println("Skipping project overview (no change needed)")
		}

		// Architecture overview for Normal and Max tiers.
		if cfg.Quality != config.QualityLite && shouldRegenArch {
			fmt.Println("Regenerating architecture overview...")
			if err := docGen.GenerateArchitecture(ctx, allDocs, llmProvider, cfg.Model); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: architecture regeneration failed: %v\n", err)
			}
		} else if cfg.Quality != config.QualityLite {
			fmt.Println("Skipping architecture overview (no change needed)")
		}
	}

	// Update and save state.
	state.LastCommitSHA = indexer.GetGitCommitSHA(rootDir)
	if err := state.SaveState(rootDir); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	// Calculate unchanged count.
	unchangedCount := len(allFiles) - updatedCount - deletedCount

	// Print summary.
	duration := time.Since(start)
	fmt.Println()
	fmt.Println("Incremental update complete!")
	fmt.Printf("  Files updated:     %d\n", updatedCount)
	if len(depAffected) > 0 {
		fmt.Printf("  Dep-affected:      %d\n", len(depAffected))
	}
	fmt.Printf("  Files deleted:     %d\n", deletedCount)
	fmt.Printf("  Files unchanged:   %d\n", unchangedCount)

	if totalInputTokens > 0 || totalOutputTokens > 0 {
		fmt.Printf("  Tokens used:       %d input, %d output\n", totalInputTokens, totalOutputTokens)
		cost := llm.EstimateCost(cfg.Model, totalInputTokens, totalOutputTokens)
		if cost > 0 {
			fmt.Printf("  Estimated cost:    $%.4f\n", cost)
		}
	}

	fmt.Printf("  Duration:          %s\n", duration.Round(time.Millisecond))
	fmt.Printf("  Output:            %s\n", cfg.OutputDir)

	if regenAdvice != nil && regenAdvice.Reasoning != "" {
		fmt.Printf("  Regen decision:    %s\n", regenAdvice.Reasoning)
	}

	if len(pipelineErrors) > 0 {
		fmt.Fprintf(os.Stderr, "\nWarnings (%d):\n", len(pipelineErrors))
		for _, e := range pipelineErrors {
			fmt.Fprintf(os.Stderr, "  - %v\n", e)
		}
	}

	return nil
}
