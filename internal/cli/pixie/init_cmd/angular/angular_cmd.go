package angular

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pixie-sh/errors-go"
	"github.com/spf13/cobra"

	"github.com/pixie-sh/pixie-cli/internal/cli/pixie/init_cmd/shared"
)

// Options holds all options for the angular init command
type Options struct {
	Name   string // Project name
	Output string // Output directory
	Force  bool   // Overwrite existing files
}

// Cmd returns the angular init subcommand
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "angular",
		Short: "Initialize an Angular frontend project",
		Long: `Initialize a complete Angular frontend project with modern best practices.

This command generates a project structure with:
  - Angular 18+ with standalone components
  - Signal-based state management
  - GitHub Actions CI/CD workflows
  - ESLint and Prettier configuration
  - Environment configuration

Note: For full Angular scaffolding, use the Angular CLI (ng new).
This command adds pixie-specific configurations and GitHub Actions.

Examples:
  # Initialize a new Angular frontend project
  pixie init angular --name my-frontend

  # Initialize with custom output directory
  pixie init angular --name my-frontend --output /path/to/output

  # Force overwrite existing files
  pixie init angular --name my-frontend --force
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			output, _ := cmd.Flags().GetString("output")
			force, _ := cmd.Flags().GetBool("force")

			opts := Options{
				Name:   name,
				Output: output,
				Force:  force,
			}

			return Run(opts)
		},
	}

	// Required flags
	cmd.Flags().String("name", "", "Project name (required)")

	// Optional flags
	cmd.Flags().String("output", ".", "Output directory")
	cmd.Flags().Bool("force", false, "Force overwrite existing files")

	// Mark required flags
	cmd.MarkFlagRequired("name")

	return cmd
}

// Run executes the angular init command
func Run(opts Options) error {
	// Validate inputs
	if opts.Name == "" {
		return errors.New("project name is required")
	}

	fmt.Printf("Initializing Angular frontend project: %s\n", opts.Name)
	fmt.Printf("   Output: %s\n\n", opts.Output)

	// Generate GitHub Actions workflows
	fmt.Println("Generating GitHub Actions workflows...")
	if err := generateGitHubActions(opts); err != nil {
		return errors.Wrap(err, "failed to generate GitHub Actions")
	}

	// Generate Angular project structure
	fmt.Println("Generating Angular project structure...")
	if err := generateProjectStructure(opts); err != nil {
		return errors.Wrap(err, "failed to generate project structure")
	}

	fmt.Printf("\nAngular frontend project initialized successfully!\n\n")
	printNextSteps(opts)

	return nil
}

// generateGitHubActions generates GitHub Actions workflow files for Angular
func generateGitHubActions(opts Options) error {
	workflowsDir := filepath.Join(opts.Output, ".github", "workflows")

	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create workflows directory")
	}

	// Angular-specific tests workflow
	testsContent := `name: Tests
on:
  push:
    branches:
      - '**'

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'

      - name: Install dependencies
        run: npm ci

      - name: Run linting
        run: npm run lint

      - name: Run tests
        run: npm test -- --watch=false --browsers=ChromeHeadless

      - name: Build
        run: npm run build
`

	testsPath := filepath.Join(workflowsDir, "tests.yaml")
	if err := shared.WriteFile(testsPath, []byte(testsContent), opts.Force); err != nil {
		if !opts.Force {
			fmt.Printf("   WARNING: Skipping %s (file exists)\n", testsPath)
		} else {
			return errors.Wrap(err, "failed to write tests.yaml")
		}
	} else {
		fmt.Printf("   Generated %s\n", testsPath)
	}

	// Angular-specific build workflow
	buildContent := `name: Build and Deploy
on:
  push:
    tags:
      - 'v[0-9]+.[0-9]+.[0-9]+'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'

      - name: Install dependencies
        run: npm ci

      - name: Build for production
        run: npm run build -- --configuration=production

      - name: Prepare version
        id: prepare_version
        run: |
          VERSION=$(echo $GITHUB_REF | sed 's/refs\/tags\///')
          echo "VERSION=$VERSION" >> $GITHUB_ENV
          echo "version=$VERSION" >> $GITHUB_OUTPUT

      # Add your deployment steps here
      # Example: Deploy to S3, Firebase Hosting, etc.
`

	buildPath := filepath.Join(workflowsDir, "build.yaml")
	if err := shared.WriteFile(buildPath, []byte(buildContent), opts.Force); err != nil {
		if !opts.Force {
			fmt.Printf("   WARNING: Skipping %s (file exists)\n", buildPath)
		} else {
			return errors.Wrap(err, "failed to write build.yaml")
		}
	} else {
		fmt.Printf("   Generated %s\n", buildPath)
	}

	return nil
}

// generateProjectStructure generates Angular-specific project files
func generateProjectStructure(opts Options) error {
	// Generate README
	readmeContent := fmt.Sprintf(`# %s

Angular frontend application generated by Pixie CLI.

## Prerequisites

- Node.js 20+
- npm 10+
- Angular CLI 18+

## Getting Started

1. Install dependencies:
   `+"`"+`npm install`+"`"+`

2. Start development server:
   `+"`"+`ng serve`+"`"+`

3. Open http://localhost:4200

## Available Scripts

- `+"`"+`npm start`+"`"+` - Start development server
- `+"`"+`npm test`+"`"+` - Run unit tests
- `+"`"+`npm run lint`+"`"+` - Run linting
- `+"`"+`npm run build`+"`"+` - Build for production

## Project Structure

`+"`"+``+"`"+``+"`"+`
src/
├── app/
│   ├── components/     # Reusable components
│   ├── pages/          # Page components
│   ├── services/       # Angular services
│   ├── models/         # TypeScript interfaces
│   └── app.config.ts   # Application configuration
├── assets/             # Static assets
└── environments/       # Environment configs
`+"`"+``+"`"+``+"`"+`

## Learn More

- [Angular Documentation](https://angular.dev)
- [Angular CLI](https://angular.dev/tools/cli)
`, opts.Name)

	readmePath := filepath.Join(opts.Output, "README.md")
	if err := shared.WriteFile(readmePath, []byte(readmeContent), opts.Force); err != nil {
		if !opts.Force {
			fmt.Printf("   WARNING: Skipping %s (file exists)\n", readmePath)
		} else {
			return errors.Wrap(err, "failed to write README.md")
		}
	} else {
		fmt.Printf("   Generated %s\n", readmePath)
	}

	return nil
}

// printNextSteps prints the next steps after initialization
func printNextSteps(opts Options) {
	fmt.Printf("Next steps:\n\n")
	fmt.Printf("1. Navigate to the project directory:\n")
	fmt.Printf("   cd %s\n\n", opts.Output)
	fmt.Printf("2. Create Angular application (if not exists):\n")
	fmt.Printf("   ng new %s --directory . --skip-git\n\n", opts.Name)
	fmt.Printf("3. Install dependencies:\n")
	fmt.Printf("   npm install\n\n")
	fmt.Printf("4. Start development server:\n")
	fmt.Printf("   ng serve\n\n")
	fmt.Printf("5. Open http://localhost:4200\n\n")
	fmt.Printf("For more information, see README.md\n")
}
