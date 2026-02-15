package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ziadkadry99/auto-doc/internal/llm"
	"github.com/ziadkadry99/auto-doc/internal/site"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
)

var siteCmd = &cobra.Command{
	Use:   "site",
	Short: "Generate a static documentation website",
	Long:  `Generates a self-contained static HTML site from the generated documentation, with search and navigation.`,
	RunE:  runSite,
}

func init() {
	siteCmd.Flags().Bool("serve", false, "start a local HTTP server after generating")
	siteCmd.Flags().Int("port", 8080, "port for the local dev server")
	siteCmd.Flags().Bool("open", false, "open browser automatically when serving")
	siteCmd.Flags().String("output", "", "override output directory (defaults to {outputDir}/site)")
	rootCmd.AddCommand(siteCmd)
}

func runSite(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Verify that docs have been generated.
	docsDir := filepath.Join(cfg.OutputDir, "docs")
	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
		return fmt.Errorf("docs directory not found at %s\nRun `autodoc generate` first to create documentation", docsDir)
	}

	// Determine output directory.
	outputDir, _ := cmd.Flags().GetString("output")
	if outputDir == "" {
		outputDir = filepath.Join(cfg.OutputDir, "site")
	}

	// Derive project name from the working directory.
	projectName := "Documentation"
	if wd, wdErr := os.Getwd(); wdErr == nil {
		projectName = filepath.Base(wd)
	}
	if projectName == "." || projectName == "" {
		projectName = "Documentation"
	}
	generator := site.NewSiteGenerator(docsDir, outputDir, projectName)
	generator.LogoPath = cfg.Logo
	pageCount, err := generator.Generate()
	if err != nil {
		return fmt.Errorf("generating site: %w", err)
	}

	fmt.Printf("Static site generated: %s (%d pages)\n", outputDir, pageCount)

	// Optionally serve the site.
	serve, _ := cmd.Flags().GetBool("serve")
	if serve {
		port, _ := cmd.Flags().GetInt("port")
		openBrowser, _ := cmd.Flags().GetBool("open")

		// Try to load the vector store for AI-powered search.
		var store vectordb.VectorStore
		vectorDir := filepath.Join(cfg.OutputDir, "vectordb")
		if _, statErr := os.Stat(vectorDir); statErr == nil {
			embedder, embErr := createEmbedderFromConfig(cfg)
			if embErr == nil {
				chromemStore, storeErr := vectordb.NewChromemStore(embedder)
				if storeErr == nil {
					if loadErr := chromemStore.Load(context.Background(), vectorDir); loadErr == nil && chromemStore.Count() > 0 {
						store = chromemStore
						fmt.Printf("AI search enabled (%d documents indexed)\n", chromemStore.Count())
					}
				}
			}
			if store == nil {
				fmt.Println("AI search unavailable (vector DB could not be loaded — file search still works)")
			}
		} else {
			fmt.Println("AI search unavailable (no vector DB found — run `autodoc generate` first)")
		}

		// Try to create an LLM provider for search answer synthesis.
		var llmProvider llm.Provider
		if p, llmErr := createLLMProviderFromConfig(cfg); llmErr == nil {
			llmProvider = p
			fmt.Println("LLM-powered search answers enabled")
		}

		fmt.Printf("Serving at http://localhost:%d — press Ctrl+C to stop\n", port)
		if err := site.Serve(outputDir, port, openBrowser, store, llmProvider, cfg.Model); err != nil {
			return fmt.Errorf("serving site: %w", err)
		}
	}

	return nil
}
