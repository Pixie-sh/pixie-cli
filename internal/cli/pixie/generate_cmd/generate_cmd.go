package generate_cmd

import (
	"github.com/spf13/cobra"

	"github.com/pixie-sh/pixie-cli/internal/cli/pixie/generate_cmd/openapi"
	"github.com/pixie-sh/pixie-cli/internal/cli/pixie/generate_cmd/scaffold"
)

// GenerateCmd returns the generate parent command with all subcommands
func GenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Code generation and analysis tools",
		Long: `Tools for generating boilerplate code and analyzing the codebase.

Available subcommands:

Scaffold commands:
  microservice  - Generate a new microservice with full structure
  domain        - Generate a new domain within a microservice
  entity        - Generate a new entity with migration and model
  service       - Generate a new service in a domain
  repository    - Generate a new repository in a domain

OpenAPI commands:
  openapi-spec       - Generate OpenAPI 3.0 specification from controllers
  extract-endpoints  - Extract HTTP endpoints from controller files

Configuration:
  Directory conventions and naming patterns can be customized via
  .pixie.yaml or pixie.yaml in the project root. Run with defaults
  if no config file is present.

Examples:
  # Generate a new microservice
  pixie generate microservice --name payments

  # Generate a domain with entity features
  pixie generate domain --ms payments --name billing --features entity,migration

  # Generate OpenAPI spec
  pixie generate openapi-spec --output api-spec.yaml

  # Extract endpoints from specific microservices
  pixie generate extract-endpoints --ms payments --ms notifications
`,
	}

	// Scaffold subcommands
	cmd.AddCommand(scaffold.MicroserviceCmd())
	cmd.AddCommand(scaffold.DomainCmd())
	cmd.AddCommand(scaffold.EntityCmd())
	cmd.AddCommand(scaffold.ServiceCmd())
	cmd.AddCommand(scaffold.RepositoryCmd())

	// OpenAPI subcommands
	cmd.AddCommand(openapi.OpenAPISpecCmd())
	cmd.AddCommand(openapi.ExtractEndpointsCmd())

	return cmd
}
