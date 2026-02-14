package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ziadkadry99/auto-doc/internal/config"
	"github.com/ziadkadry99/auto-doc/internal/indexer"
	"github.com/ziadkadry99/auto-doc/internal/llm"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
	"github.com/ziadkadry99/auto-doc/internal/walker"
)

var costCmd = &cobra.Command{
	Use:   "cost",
	Short: "Estimate API costs for generating documentation",
	Long:  `Performs a dry run that counts files, estimates tokens, and calculates the expected API cost without making any calls.`,
	RunE:  runCost,
}

func init() {
	rootCmd.AddCommand(costCmd)
}

func runCost(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load config.
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Get working directory.
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Walk codebase.
	files, err := walker.Walk(walker.WalkerConfig{
		RootDir:     rootDir,
		Include:     cfg.Include,
		Exclude:     cfg.Exclude,
		MaxFileSize: 0,
	})
	if err != nil {
		return fmt.Errorf("walking codebase: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No files found to document.")
		return nil
	}

	// We need an LLM provider and embedder to create the pipeline (for DryRun),
	// but DryRun doesn't actually make API calls.
	llmProvider, err := llm.NewProvider(string(cfg.Provider), cfg.Model)
	if err != nil {
		return fmt.Errorf("creating LLM provider: %w", err)
	}

	embedder, err := createEmbedderFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating embedder: %w", err)
	}

	store, err := vectordb.NewChromemStore(embedder)
	if err != nil {
		return fmt.Errorf("creating vector store: %w", err)
	}

	pipeline := indexer.NewPipeline(llmProvider, embedder, store, cfg, rootDir)

	estimate, err := pipeline.DryRun(ctx, files)
	if err != nil {
		return fmt.Errorf("cost estimation failed: %w", err)
	}

	// Print cost breakdown.
	fmt.Println("Cost Estimate")
	fmt.Println("=============")
	fmt.Printf("  Total files found:   %d\n", len(files))
	fmt.Printf("  Files to process:    %d (changed since last run)\n", estimate.TotalFiles)
	fmt.Printf("  Estimated tokens:    %d\n", estimate.TotalTokensEstimate)
	fmt.Println()

	fmt.Println("  Cost Breakdown:")
	for op, cost := range estimate.CostBreakdown {
		fmt.Printf("    %-20s $%.4f\n", op, cost)
	}
	fmt.Printf("    %-20s --------\n", "")
	fmt.Printf("    %-20s $%.4f\n", "Total", estimate.EstimatedCost)
	fmt.Println()

	// Show tier comparison.
	fmt.Println("  Tier Comparison:")
	fmt.Println("  ────────────────────────────────────────")
	for _, tier := range []config.QualityTier{config.QualityLite, config.QualityNormal, config.QualityMax} {
		tierCfg := *cfg
		tierCfg.Quality = tier
		preset := config.GetPreset(cfg.Provider, tier)
		tierCfg.Model = preset.Model

		tierPipeline := indexer.NewPipeline(llmProvider, embedder, store, &tierCfg, rootDir)
		tierEstimate, err := tierPipeline.DryRun(ctx, files)
		if err != nil {
			continue
		}

		marker := " "
		if tier == cfg.Quality {
			marker = "*"
		}
		fmt.Printf("  %s %-8s  ~$%.4f  (model: %s)\n", marker, tier, tierEstimate.EstimatedCost, preset.Model)
	}
	fmt.Println()
	fmt.Println("  * = current configuration")
	fmt.Println()
	fmt.Printf("  Provider: %s\n", cfg.Provider)
	fmt.Printf("  Model:    %s\n", cfg.Model)
	fmt.Printf("  Quality:  %s\n", cfg.Quality)

	return nil
}
