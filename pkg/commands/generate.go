// Package commands provides public access to pixie-cli commands for embedding in other CLIs.
package commands

import (
	"github.com/spf13/cobra"

	"github.com/pixie-sh/pixie-cli/internal/cli/pixie/generate_cmd"
)

// GenerateCmd returns the generate command group with all subcommands.
// This includes:
//   - openapi-spec: Generate OpenAPI 3.0 specification from controllers
//   - extract-endpoints: Extract HTTP endpoints from controller files
//   - microservice: Scaffold a new microservice
//   - domain: Scaffold a new domain
//   - entity: Scaffold a new entity
//   - service: Scaffold a new service
//   - repository: Scaffold a new repository
func GenerateCmd() *cobra.Command {
	return generate_cmd.GenerateCmd()
}
