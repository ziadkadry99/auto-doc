package cmd

import (
	"github.com/spf13/cobra"
	"github.com/ziadkadry99/auto-doc/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize autodoc configuration with an interactive wizard",
	Long:  `Runs an interactive wizard to configure autodoc for your project and generates a .autodoc.yml file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := config.RunWizard()
		return err
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
