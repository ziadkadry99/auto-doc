package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "autodoc",
	Short: "AI-powered codebase documentation and semantic indexing",
	Long: `Auto Doc reads your entire codebase using AI to generate comprehensive
documentation and builds a semantic vector database for intelligent
code navigation. It integrates with AI agents via MCP for instant
codebase understanding.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", ".autodoc.yml", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

func exitOnError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
