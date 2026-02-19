package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ziadkadry99/auto-doc/internal/config"
	"github.com/ziadkadry99/auto-doc/internal/contextengine"
	"github.com/ziadkadry99/auto-doc/internal/db"
	"github.com/ziadkadry99/auto-doc/internal/flows"
	"github.com/ziadkadry99/auto-doc/internal/registry"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage repositories in the central documentation server",
	Long:  `Add, list, remove, and sync repositories for multi-repo documentation.`,
}

var repoAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Register a repository",
	Long:  `Register a repository by local path or git URL. The repo must have been analyzed with 'autodoc generate' first.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoAdd,
}

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered repositories",
	RunE:  runRepoList,
}

var repoRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Unregister a repository",
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoRemove,
}

var repoSyncCmd = &cobra.Command{
	Use:   "sync <name>",
	Short: "Re-import a repository (git pull if remote)",
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoSync,
}

var repoSyncAllCmd = &cobra.Command{
	Use:   "sync-all",
	Short: "Sync all registered repositories",
	RunE:  runRepoSyncAll,
}

func init() {
	repoAddCmd.Flags().String("url", "", "Git URL to clone")
	repoAddCmd.Flags().String("path", "", "Local path to the repository")
	repoAddCmd.Flags().String("display-name", "", "Display name for the repository")

	repoCmd.AddCommand(repoAddCmd)
	repoCmd.AddCommand(repoListCmd)
	repoCmd.AddCommand(repoRemoveCmd)
	repoCmd.AddCommand(repoSyncCmd)
	repoCmd.AddCommand(repoSyncAllCmd)
	rootCmd.AddCommand(repoCmd)
}

func openCentralDB(cfg *config.Config) (*db.DB, error) {
	dbPath := filepath.Join(cfg.OutputDir, "autodoc.db")
	return db.Open(dbPath)
}

func createCentralVectorStore(cfg *config.Config) (vectordb.VectorStore, error) {
	embedder, err := createEmbedderFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating embedder: %w", err)
	}

	store, err := vectordb.NewChromemStore(embedder)
	if err != nil {
		return nil, fmt.Errorf("creating vector store: %w", err)
	}

	vectorDir := filepath.Join(cfg.OutputDir, "vectordb")
	if err := store.Load(context.Background(), vectorDir); err != nil {
		// Non-fatal: may be first run.
		fmt.Fprintf(os.Stderr, "Note: starting with empty vector store (%v)\n", err)
	}

	return store, nil
}

func runRepoAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	gitURL, _ := cmd.Flags().GetString("url")
	localPath, _ := cmd.Flags().GetString("path")
	displayName, _ := cmd.Flags().GetString("display-name")

	if gitURL == "" && localPath == "" {
		return fmt.Errorf("either --url or --path is required")
	}
	if gitURL != "" && localPath != "" {
		return fmt.Errorf("specify either --url or --path, not both")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	database, err := openCentralDB(cfg)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer database.Close()

	repoStore := registry.NewStore(database)

	// Check if repo already exists.
	existing, err := repoStore.Get(context.Background(), name)
	if err != nil {
		return fmt.Errorf("checking existing repo: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("repository %q already registered (use 'autodoc repo sync %s' to update)", name, name)
	}

	repo := &registry.Repository{
		Name:        name,
		DisplayName: displayName,
	}
	if displayName == "" {
		repo.DisplayName = name
	}

	if gitURL != "" {
		// Clone the repo.
		homeDir, _ := os.UserHomeDir()
		cloneDir := filepath.Join(homeDir, ".autodoc", "repos", name)
		if err := os.MkdirAll(filepath.Dir(cloneDir), 0o755); err != nil {
			return fmt.Errorf("creating repos directory: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Cloning %s to %s...\n", gitURL, cloneDir)
		if _, statErr := os.Stat(cloneDir); statErr == nil {
			// Directory exists — do a pull instead.
			pullCmd := exec.Command("git", "-C", cloneDir, "pull")
			pullCmd.Stdout = os.Stderr
			pullCmd.Stderr = os.Stderr
			if err := pullCmd.Run(); err != nil {
				return fmt.Errorf("git pull in %s: %w", cloneDir, err)
			}
		} else {
			cloneCmd := exec.Command("git", "clone", gitURL, cloneDir)
			cloneCmd.Stdout = os.Stderr
			cloneCmd.Stderr = os.Stderr
			if err := cloneCmd.Run(); err != nil {
				return fmt.Errorf("git clone %s: %w", gitURL, err)
			}
		}

		repo.SourceType = "git"
		repo.SourceURL = gitURL
		repo.LocalPath = cloneDir
	} else {
		// Local path.
		absPath, err := filepath.Abs(localPath)
		if err != nil {
			return fmt.Errorf("resolving path: %w", err)
		}
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", absPath)
		}
		repo.SourceType = "local"
		repo.LocalPath = absPath
	}

	// Check for .autodoc/analyses.json.
	analysesPath := filepath.Join(repo.LocalPath, ".autodoc", "analyses.json")
	if _, err := os.Stat(analysesPath); os.IsNotExist(err) {
		return fmt.Errorf("no .autodoc/analyses.json found in %s\nRun `autodoc generate` in that repository first", repo.LocalPath)
	}

	// Register the repo.
	if err := repoStore.Add(context.Background(), repo); err != nil {
		return fmt.Errorf("registering repository: %w", err)
	}

	// Import artifacts.
	vecStore, err := createCentralVectorStore(cfg)
	if err != nil {
		return fmt.Errorf("creating vector store: %w", err)
	}

	importer := registry.NewImporter(repoStore, vecStore, cfg.Quality)
	fmt.Fprintf(os.Stderr, "Importing %s...\n", name)
	if err := importer.ImportRepo(context.Background(), repo); err != nil {
		return fmt.Errorf("importing repository: %w", err)
	}

	// Persist vector store.
	vectorDir := filepath.Join(cfg.OutputDir, "vectordb")
	if err := os.MkdirAll(vectorDir, 0o755); err != nil {
		return fmt.Errorf("creating vector dir: %w", err)
	}
	if err := vecStore.Persist(context.Background(), vectorDir); err != nil {
		return fmt.Errorf("persisting vector store: %w", err)
	}

	// Discover cross-service links (requires LLM).
	llmProvider, llmErr := createLLMProviderFromConfig(cfg)
	if llmErr == nil {
		ctxStore := contextengine.NewStore(database)
		flowStore := flows.NewStore(database)
		linker := registry.NewLinker(repoStore, ctxStore, flowStore)
		fmt.Fprintf(os.Stderr, "Discovering cross-service links...\n")
		if linkErr := linker.DiscoverLinks(context.Background(), repo, llmProvider, cfg.Model); linkErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: link discovery failed: %v\n", linkErr)
		} else {
			links, _ := repoStore.GetLinks(context.Background(), name)
			if len(links) > 0 {
				fmt.Fprintf(os.Stderr, "  Discovered %d cross-service link(s)\n", len(links))
			}
		}
	}

	fmt.Printf("Repository %q registered successfully\n", name)
	fmt.Printf("  Status: %s\n", repo.Status)
	fmt.Printf("  Files: %d\n", repo.FileCount)
	fmt.Printf("  Path: %s\n", repo.LocalPath)
	if repo.Summary != "" {
		fmt.Printf("  Summary: %s\n", repo.Summary)
	}

	return nil
}

func runRepoList(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	database, err := openCentralDB(cfg)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer database.Close()

	repoStore := registry.NewStore(database)
	repos, err := repoStore.List(context.Background())
	if err != nil {
		return fmt.Errorf("listing repositories: %w", err)
	}

	if len(repos) == 0 {
		fmt.Println("No repositories registered. Use `autodoc repo add` to register one.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tFILES\tTYPE\tLAST INDEXED\tSUMMARY")
	for _, r := range repos {
		lastIndexed := r.LastIndexedAt
		if lastIndexed == "" {
			lastIndexed = "-"
		} else if len(lastIndexed) > 19 {
			lastIndexed = lastIndexed[:19]
		}
		summary := r.Summary
		if len(summary) > 60 {
			summary = summary[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n",
			r.Name, r.Status, r.FileCount, r.SourceType, lastIndexed, summary)
	}
	w.Flush()

	return nil
}

func runRepoRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	database, err := openCentralDB(cfg)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer database.Close()

	repoStore := registry.NewStore(database)

	// Check repo exists.
	repo, err := repoStore.Get(context.Background(), name)
	if err != nil {
		return fmt.Errorf("looking up repository: %w", err)
	}
	if repo == nil {
		return fmt.Errorf("repository %q not found", name)
	}

	// Clean up vector store entries.
	vecStore, err := createCentralVectorStore(cfg)
	if err == nil {
		vecStore.DeleteByRepoID(context.Background(), name)
		vectorDir := filepath.Join(cfg.OutputDir, "vectordb")
		vecStore.Persist(context.Background(), vectorDir)
	}

	// Remove from database.
	if err := repoStore.Remove(context.Background(), name); err != nil {
		return fmt.Errorf("removing repository: %w", err)
	}

	fmt.Printf("Repository %q removed\n", name)
	return nil
}

func runRepoSync(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	database, err := openCentralDB(cfg)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer database.Close()

	repoStore := registry.NewStore(database)

	repo, err := repoStore.Get(context.Background(), name)
	if err != nil {
		return fmt.Errorf("looking up repository: %w", err)
	}
	if repo == nil {
		return fmt.Errorf("repository %q not found", name)
	}

	// Git pull if it's a git repo.
	if repo.SourceType == "git" {
		fmt.Fprintf(os.Stderr, "Pulling latest changes for %s...\n", name)
		pullCmd := exec.Command("git", "-C", repo.LocalPath, "pull")
		pullCmd.Stdout = os.Stderr
		pullCmd.Stderr = os.Stderr
		if err := pullCmd.Run(); err != nil {
			return fmt.Errorf("git pull: %w", err)
		}
	}

	// Re-import.
	vecStore, err := createCentralVectorStore(cfg)
	if err != nil {
		return fmt.Errorf("creating vector store: %w", err)
	}

	importer := registry.NewImporter(repoStore, vecStore, cfg.Quality)
	fmt.Fprintf(os.Stderr, "Re-importing %s...\n", name)
	if err := importer.ImportRepo(context.Background(), repo); err != nil {
		return fmt.Errorf("importing repository: %w", err)
	}

	// Persist vector store.
	vectorDir := filepath.Join(cfg.OutputDir, "vectordb")
	if err := vecStore.Persist(context.Background(), vectorDir); err != nil {
		return fmt.Errorf("persisting vector store: %w", err)
	}

	// Re-discover cross-service links.
	llmProvider, llmErr := createLLMProviderFromConfig(cfg)
	if llmErr == nil {
		ctxStore := contextengine.NewStore(database)
		flowStore := flows.NewStore(database)
		linker := registry.NewLinker(repoStore, ctxStore, flowStore)
		fmt.Fprintf(os.Stderr, "Discovering cross-service links...\n")
		if linkErr := linker.DiscoverLinks(context.Background(), repo, llmProvider, cfg.Model); linkErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: link discovery failed: %v\n", linkErr)
		}
	}

	fmt.Printf("Repository %q synced successfully (%d files)\n", name, repo.FileCount)
	return nil
}

func runRepoSyncAll(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	database, err := openCentralDB(cfg)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer database.Close()

	repoStore := registry.NewStore(database)
	repos, err := repoStore.List(context.Background())
	if err != nil {
		return fmt.Errorf("listing repositories: %w", err)
	}

	if len(repos) == 0 {
		fmt.Println("No repositories registered.")
		return nil
	}

	vecStore, err := createCentralVectorStore(cfg)
	if err != nil {
		return fmt.Errorf("creating vector store: %w", err)
	}

	importer := registry.NewImporter(repoStore, vecStore, cfg.Quality)
	var errors []string

	for _, r := range repos {
		repo := r // copy
		fmt.Fprintf(os.Stderr, "Syncing %s...\n", repo.Name)

		// Git pull if needed.
		if repo.SourceType == "git" {
			pullCmd := exec.Command("git", "-C", repo.LocalPath, "pull")
			pullCmd.Stdout = os.Stderr
			pullCmd.Stderr = os.Stderr
			if err := pullCmd.Run(); err != nil {
				errors = append(errors, fmt.Sprintf("%s: git pull failed: %v", repo.Name, err))
				continue
			}
		}

		if err := importer.ImportRepo(context.Background(), &repo); err != nil {
			errors = append(errors, fmt.Sprintf("%s: import failed: %v", repo.Name, err))
			continue
		}

		fmt.Printf("  %s: synced (%d files)\n", repo.Name, repo.FileCount)
	}

	// Persist vector store.
	vectorDir := filepath.Join(cfg.OutputDir, "vectordb")
	if err := vecStore.Persist(context.Background(), vectorDir); err != nil {
		return fmt.Errorf("persisting vector store: %w", err)
	}

	// Re-discover cross-service links for all repos (needs full set for accuracy).
	llmProvider, llmErr := createLLMProviderFromConfig(cfg)
	if llmErr == nil {
		ctxStore := contextengine.NewStore(database)
		flowStore := flows.NewStore(database)
		linker := registry.NewLinker(repoStore, ctxStore, flowStore)
		fmt.Fprintf(os.Stderr, "\nDiscovering cross-service links...\n")
		for _, r := range repos {
			repo := r
			fmt.Fprintf(os.Stderr, "  Analyzing %s...\n", repo.Name)
			if linkErr := linker.DiscoverLinks(context.Background(), &repo, llmProvider, cfg.Model); linkErr != nil {
				fmt.Fprintf(os.Stderr, "  Warning: link discovery failed for %s: %v\n", repo.Name, linkErr)
			}
		}
		allLinks, _ := repoStore.GetLinks(context.Background(), "")
		fmt.Fprintf(os.Stderr, "  Total cross-service links: %d\n", len(allLinks))
	}

	if len(errors) > 0 {
		fmt.Fprintf(os.Stderr, "\nErrors:\n%s\n", strings.Join(errors, "\n"))
	}

	fmt.Printf("\nSynced %d/%d repositories\n", len(repos)-len(errors), len(repos))
	return nil
}
