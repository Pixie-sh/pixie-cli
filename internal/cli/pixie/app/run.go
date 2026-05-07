package app

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pixie-sh/pixie-cli/internal/cli/pixie/db_shell_cmd"
	"github.com/pixie-sh/pixie-cli/internal/cli/pixie/generate_cmd"
	"github.com/pixie-sh/pixie-cli/internal/cli/pixie/init_cmd"
	"github.com/pixie-sh/pixie-cli/internal/version"
)

func Run() {
	rootCmd := NewRootCmd()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "pixie",
		Short:   "Pixie CLI - Multi-Stack Project Generator",
		Long:    "Pixie CLI generates complete projects for Go backend, Angular frontend, and Expo mobile applications.",
		Version: version.Info(),
	}

	rootCmd.SetVersionTemplate("pixie version {{.Version}}\n")
	rootCmd.PersistentFlags().String("config", "", "Path to configuration file")
	rootCmd.PersistentFlags().String("env", "", "Path to environment file")
	rootCmd.AddCommand(init_cmd.InitCmd())
	rootCmd.AddCommand(generate_cmd.GenerateCmd())
	rootCmd.AddCommand(db_shell_cmd.Cmd())
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("pixie version %s\n", version.Info())
		},
	})

	return rootCmd
}
