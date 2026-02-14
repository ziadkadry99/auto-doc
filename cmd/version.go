package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set via ldflags at build time.
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of autodoc",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("autodoc %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
