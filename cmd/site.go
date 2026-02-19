package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ziadkadry99/auto-doc/internal/config"
	"github.com/ziadkadry99/auto-doc/internal/flows"
	"github.com/ziadkadry99/auto-doc/internal/indexer"
	"github.com/ziadkadry99/auto-doc/internal/llm"
	"github.com/ziadkadry99/auto-doc/internal/registry"
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
	siteCmd.Flags().Bool("central", false, "generate a combined multi-repo site from all registered repositories")
	rootCmd.AddCommand(siteCmd)
}

func runSite(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	central, _ := cmd.Flags().GetBool("central")

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

	var pageCount int

	if central {
		pageCount, err = runCentralSite(cfg, outputDir, projectName)
	} else {
		// Verify that docs have been generated.
		docsDir := filepath.Join(cfg.OutputDir, "docs")
		if _, err := os.Stat(docsDir); os.IsNotExist(err) {
			return fmt.Errorf("docs directory not found at %s\nRun `autodoc generate` first to create documentation", docsDir)
		}

		generator := site.NewSiteGenerator(docsDir, outputDir, projectName)
		generator.LogoPath = cfg.Logo
		pageCount, err = generator.Generate()
	}
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

// runCentralSite generates a combined multi-repo site from all registered repositories.
func runCentralSite(cfg *config.Config, outputDir, projectName string) (int, error) {
	ctx := context.Background()

	// Open the central database.
	database, err := openCentralDB(cfg)
	if err != nil {
		return 0, fmt.Errorf("opening central database: %w\nHave you registered any repos with `autodoc repo add`?", err)
	}
	defer database.Close()

	// Load repos.
	repoStore := registry.NewStore(database)
	repos, err := repoStore.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing repos: %w", err)
	}
	if len(repos) == 0 {
		return 0, fmt.Errorf("no repositories registered\nUse `autodoc repo add <name> --path <path>` to register repos first")
	}

	// Convert repos to site RepoInfo.
	siteRepos := make([]site.RepoInfo, len(repos))
	for i, r := range repos {
		docsDir := filepath.Join(r.LocalPath, ".autodoc", "docs")
		if _, statErr := os.Stat(docsDir); os.IsNotExist(statErr) {
			docsDir = "" // No docs available for this repo.
		}
		// Detect primary language from analyses.
		lang := detectRepoLanguage(r.LocalPath)

		siteRepos[i] = site.RepoInfo{
			Name:          r.Name,
			DisplayName:   r.DisplayName,
			Summary:       r.Summary,
			Status:        r.Status,
			FileCount:     r.FileCount,
			SourceType:    r.SourceType,
			Language:      lang,
			LastCommitSHA: r.LastCommitSHA,
			DocsDir:       docsDir,
		}
	}

	// Load cross-service links.
	links, err := repoStore.GetLinks(ctx, "")
	if err != nil {
		return 0, fmt.Errorf("loading links: %w", err)
	}
	siteLinks := make([]site.LinkInfo, len(links))
	for i, l := range links {
		siteLinks[i] = site.LinkInfo{
			FromRepo:  l.FromRepo,
			ToRepo:    l.ToRepo,
			LinkType:  l.LinkType,
			Reason:    l.Reason,
			Endpoints: l.Endpoints,
		}
	}

	// Load flows.
	flowStore := flows.NewStore(database)
	allFlows, _ := flowStore.ListFlows(ctx)
	siteFlows := make([]site.FlowInfo, len(allFlows))
	for i, f := range allFlows {
		siteFlows[i] = site.FlowInfo{
			Name:        f.Name,
			Description: f.Description,
			Narrative:   f.Narrative,
			Diagram:     f.MermaidDiagram,
			Services:    f.Services,
		}
	}

	// Generate the combined site.
	gen := &site.CentralSiteGenerator{
		OutputDir:   outputDir,
		ProjectName: projectName + " System",
		Repos:       siteRepos,
		Links:       siteLinks,
		Flows:       siteFlows,
		LogoPath:    cfg.Logo,
	}

	fmt.Printf("Generating central site for %d repositories...\n", len(repos))
	return gen.Generate()
}

// detectRepoLanguage determines the primary programming language of a repo from its analyses.
func detectRepoLanguage(repoPath string) string {
	analyses, err := indexer.LoadAnalyses(repoPath)
	if err != nil || len(analyses) == 0 {
		return ""
	}
	// Skip non-programming "languages" when counting.
	skip := map[string]bool{
		"unknown": true, "": true, "YAML": true, "Docker": true,
		"JSON": true, "XML": true, "Markdown": true, "Text": true,
		"TOML": true, "INI": true, "Properties": true, "Shell": true,
	}
	langCount := make(map[string]int)
	for _, a := range analyses {
		if a.Language != "" && !skip[a.Language] {
			langCount[a.Language]++
		}
	}
	topLang := ""
	topCount := 0
	for lang, count := range langCount {
		if count > topCount {
			topLang = lang
			topCount = count
		}
	}
	return topLang
}
