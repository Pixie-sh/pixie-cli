package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/pixie-sh/pixie-cli/internal/cli/core/bootstrap_cmd"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "cli_core",
		Short: "CLI Core - Backend Project Generator",
		Long:  "CLI Core generates complete Go backend projects with authentication, notifications, and custom microservices.",
	}

	// Add flags for config and env
	rootCmd.PersistentFlags().String("config", "", "Path to configuration file")
	rootCmd.PersistentFlags().String("env", "", "Path to environment file")

	// Register commands
	rootCmd.AddCommand(bootstrap_cmd.BootstrapCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
