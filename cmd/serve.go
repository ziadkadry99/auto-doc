package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ziadkadry99/auto-doc/internal/config"
	mcpserver "github.com/ziadkadry99/auto-doc/internal/mcp"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server for AI agent integration",
	Long:  `Starts a Model Context Protocol (MCP) server on stdio, exposing codebase search tools for AI agents like Claude Code.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Create embedder for query embedding during search.
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
		if err := store.Load(context.Background(), vectorDir); err != nil {
			// Log warning but continue — store may be empty if generate hasn't run yet.
			fmt.Fprintf(os.Stderr, "Warning: could not load vector store from %s: %v\n", vectorDir, err)
			fmt.Fprintf(os.Stderr, "Search results will be empty. Run `autodoc generate` first.\n")
		}

		docsDir := cfg.OutputDir

		// Set version from the cmd package variable.
		mcpserver.Version = Version

		fmt.Fprintf(os.Stderr, "autodoc MCP server started on stdio (docs=%s, documents=%d)\n", docsDir, store.Count())

		srv := mcpserver.NewServer(store, embedder, docsDir)
		return srv.Serve()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

