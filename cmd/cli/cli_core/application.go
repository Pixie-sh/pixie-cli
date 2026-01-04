package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pixie-sh/pixie-cli/internal/cli/core/bootstrap_cmd"
	"github.com/pixie-sh/pixie-cli/internal/version"
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "pixie",
		Short:   "Pixie CLI - Backend Project Generator",
		Long:    "Pixie CLI generates complete Go backend projects with authentication, notifications, and custom microservices.",
		Version: version.Info(),
	}

	// Custom version template
	rootCmd.SetVersionTemplate("pixie version {{.Version}}\n")

	// Add flags for config and env
	rootCmd.PersistentFlags().String("config", "", "Path to configuration file")
	rootCmd.PersistentFlags().String("env", "", "Path to environment file")

	// Register commands
	rootCmd.AddCommand(bootstrap_cmd.BootstrapCmd())

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
