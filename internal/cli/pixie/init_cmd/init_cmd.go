package init_cmd

import (
	"github.com/spf13/cobra"

	"github.com/pixie-sh/pixie-cli/internal/cli/pixie/init_cmd/angular"
	"github.com/pixie-sh/pixie-cli/internal/cli/pixie/init_cmd/expo"
	"github.com/pixie-sh/pixie-cli/internal/cli/pixie/init_cmd/golang"
)

// InitCmd returns the init parent command with all stack subcommands
func InitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new project for a specific technology stack",
		Long: `Initialize a new project with complete scaffolding for your chosen technology stack.

Available stacks:
  golang   - Go backend with microservices architecture
  angular  - Angular frontend with standalone components
  expo     - Expo/React Native mobile application

Each stack generates:
  - Complete project structure following best practices
  - GitHub Actions CI/CD workflows
  - Configuration files and environment templates
  - README with setup instructions

Examples:
  # Initialize a Go backend project
  pixie init golang --name my-backend --module github.com/company/my-backend

  # Initialize an Angular frontend project
  pixie init angular --name my-frontend

  # Initialize an Expo mobile project
  pixie init expo --name my-mobile-app
`,
	}

	// Register stack subcommands
	cmd.AddCommand(golang.Cmd())
	cmd.AddCommand(angular.Cmd())
	cmd.AddCommand(expo.Cmd())

	return cmd
}
