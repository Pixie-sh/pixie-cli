package db_shell_cmd

import (
	"os/signal"

	"github.com/spf13/cobra"
)

// Cmd returns the db-shell command.
func Cmd() *cobra.Command {
	var opts Options

	cmd := &cobra.Command{
		Use:   "db-shell",
		Short: "Open an interactive SQL shell through Pixie runtime abstractions",
		Long: `Open an interactive SQL shell through Pixie runtime abstractions.

Configuration precedence:
  1. Command flags
  2. Explicit env file values (--env), then process environment variables
  3. Project config in .pixie.yaml or pixie.yaml under the db: section
  4. Built-in defaults

Runtime paths:
  - Helper-backed PostgreSQL is the primary runtime path
  - SQLite remains available for local fallback and tests

Supported built-ins:
  .help  Show available shell commands
  .exit  Close the session
  .quit  Close the session

Examples:
  pixie db-shell --driver sqlite --dsn "file:pixie-shell.db"
  pixie --env .env db-shell --name app_db --user postgres
  pixie --config .pixie.yaml db-shell --driver postgres`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.InheritedFlags().GetString("config")
			envPath, _ := cmd.InheritedFlags().GetString("env")

			resolvedConfig, err := ResolveConfig(opts, resolveConfigPath(configPath), envPath, nil)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), interruptSignals...)
			defer stop()

			executor, err := OpenExecutor(ctx, resolvedConfig)
			if err != nil {
				return err
			}

			shell := Shell{
				Executor: executor,
				In:       cmd.InOrStdin(),
				Out:      cmd.OutOrStdout(),
				ErrOut:   cmd.ErrOrStderr(),
				Prompt:   "pixie-sql> ",
			}

			return shell.Run(ctx)
		},
	}

	cmd.Flags().StringVar(&opts.Driver, "driver", defaultPostgresDriver, "Database driver (postgres primary path; sqlite fallback/test-only)")
	cmd.Flags().StringVar(&opts.DSN, "dsn", "", "Raw DSN/connection string (overrides host/user/name fields)")
	cmd.Flags().StringVar(&opts.Host, "host", "", "Database host for helper-backed PostgreSQL connections")
	cmd.Flags().IntVar(&opts.Port, "port", 0, "Database port for helper-backed PostgreSQL connections")
	cmd.Flags().StringVar(&opts.Name, "name", "", "Database name for helper-backed PostgreSQL connections")
	cmd.Flags().StringVar(&opts.User, "user", "", "Database user for helper-backed PostgreSQL connections")
	cmd.Flags().StringVar(&opts.Password, "password", "", "Database password for helper-backed PostgreSQL connections")
	cmd.Flags().StringVar(&opts.SSLMode, "sslmode", "", "SSL mode for helper-backed PostgreSQL connections")

	return cmd
}
