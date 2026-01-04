package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pixie-sh/pixie-cli/internal/cli/pixie/init_cmd"
	"github.com/pixie-sh/pixie-cli/internal/version"
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "pixie",
		Short:   "Pixie CLI - Multi-Stack Project Generator",
		Long:    "Pixie CLI generates complete projects for Go backend, Angular frontend, and Expo mobile applications.",
		Version: version.Info(),
	}

	// Custom version template
	rootCmd.SetVersionTemplate("pixie version {{.Version}}\n")

	// Add flags for config and env
	rootCmd.PersistentFlags().String("config", "", "Path to configuration file")
	rootCmd.PersistentFlags().String("env", "", "Path to environment file")

	// Register commands
	rootCmd.AddCommand(init_cmd.InitCmd()) // Multi-stack init command

	// Add version command for explicit version info
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("pixie version %s\n", version.Info())
		},
	})

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
