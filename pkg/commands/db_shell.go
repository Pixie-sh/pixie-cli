// Package commands provides public access to pixie-cli commands for embedding in other CLIs.
package commands

import (
	"github.com/spf13/cobra"

	"github.com/pixie-sh/pixie-cli/internal/cli/pixie/db_shell_cmd"
)

// DBShellCmd returns the interactive database shell command.
func DBShellCmd() *cobra.Command {
	return db_shell_cmd.Cmd()
}
