package expo

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pixie-sh/errors-go"
	"github.com/spf13/cobra"

	"github.com/pixie-sh/pixie-cli/internal/cli/pixie/init_cmd/shared"
)

// Options holds all options for the expo init command
type Options struct {
	Name   string // Project name
	Output string // Output directory
	Force  bool   // Overwrite existing files
}

// Cmd returns the expo init subcommand
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "expo",
		Short: "Initialize an Expo/React Native mobile project",
		Long: `Initialize a complete Expo/React Native mobile project.

This command generates a project structure with:
  - Expo SDK 52+
  - React Navigation setup
  - GitHub Actions CI/CD workflows
  - ESLint and Prettier configuration
  - Environment configuration

Note: For full Expo scaffolding, use npx create-expo-app.
This command adds pixie-specific configurations and GitHub Actions.

Examples:
  # Initialize a new Expo mobile project
  pixie init expo --name my-mobile-app

  # Initialize with custom output directory
  pixie init expo --name my-mobile-app --output /path/to/output

  # Force overwrite existing files
  pixie init expo --name my-mobile-app --force
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

// Run executes the expo init command
func Run(opts Options) error {
	// Validate inputs
	if opts.Name == "" {
		return errors.New("project name is required")
	}

	fmt.Printf("Initializing Expo mobile project: %s\n", opts.Name)
	fmt.Printf("   Output: %s\n\n", opts.Output)

	// Generate GitHub Actions workflows
	fmt.Println("Generating GitHub Actions workflows...")
	if err := generateGitHubActions(opts); err != nil {
		return errors.Wrap(err, "failed to generate GitHub Actions")
	}

	// Generate Expo project structure
	fmt.Println("Generating Expo project structure...")
	if err := generateProjectStructure(opts); err != nil {
		return errors.Wrap(err, "failed to generate project structure")
	}

	fmt.Printf("\nExpo mobile project initialized successfully!\n\n")
	printNextSteps(opts)

	return nil
}

// generateGitHubActions generates GitHub Actions workflow files for Expo
func generateGitHubActions(opts Options) error {
	workflowsDir := filepath.Join(opts.Output, ".github", "workflows")

	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create workflows directory")
	}

	// Expo-specific tests workflow
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
        run: npm test

      - name: Type check
        run: npx tsc --noEmit
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

	// Expo-specific build workflow
	buildContent := `name: Build
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

      - name: Setup Expo
        uses: expo/expo-github-action@v8
        with:
          expo-version: latest
          eas-version: latest
          token: ${{ secrets.EXPO_TOKEN }}

      - name: Install dependencies
        run: npm ci

      - name: Prepare version
        id: prepare_version
        run: |
          VERSION=$(echo $GITHUB_REF | sed 's/refs\/tags\///')
          echo "VERSION=$VERSION" >> $GITHUB_ENV
          echo "version=$VERSION" >> $GITHUB_OUTPUT

      - name: Build for web
        run: npx expo export --platform web

      # For native builds, use EAS Build:
      # - name: Build iOS
      #   run: eas build --platform ios --non-interactive
      # - name: Build Android
      #   run: eas build --platform android --non-interactive
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

// generateProjectStructure generates Expo-specific project files
func generateProjectStructure(opts Options) error {
	// Generate README
	readmeContent := fmt.Sprintf(`# %s

Expo/React Native mobile application generated by Pixie CLI.

## Prerequisites

- Node.js 20+
- npm 10+
- Expo CLI
- For iOS: macOS with Xcode
- For Android: Android Studio with SDK

## Getting Started

1. Install dependencies:
   `+"`"+`npm install`+"`"+`

2. Start development server:
   `+"`"+`npx expo start`+"`"+`

3. Run on device/simulator:
   - Press `+"`"+`i`+"`"+` for iOS simulator
   - Press `+"`"+`a`+"`"+` for Android emulator
   - Scan QR code with Expo Go app

## Available Scripts

- `+"`"+`npm start`+"`"+` - Start Expo development server
- `+"`"+`npm test`+"`"+` - Run tests
- `+"`"+`npm run lint`+"`"+` - Run linting
- `+"`"+`npx expo export`+"`"+` - Export for web

## Project Structure

`+"`"+``+"`"+``+"`"+`
app/
├── (tabs)/           # Tab navigation screens
├── _layout.tsx       # Root layout
└── index.tsx         # Entry screen

components/           # Reusable components
hooks/               # Custom React hooks
services/            # API services
constants/           # App constants
assets/              # Images, fonts, etc.
`+"`"+``+"`"+``+"`"+`

## Building for Production

### Using EAS Build

1. Install EAS CLI: `+"`"+`npm install -g eas-cli`+"`"+`
2. Configure: `+"`"+`eas build:configure`+"`"+`
3. Build iOS: `+"`"+`eas build --platform ios`+"`"+`
4. Build Android: `+"`"+`eas build --platform android`+"`"+`

## Learn More

- [Expo Documentation](https://docs.expo.dev)
- [React Native Documentation](https://reactnative.dev)
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
	fmt.Printf("2. Create Expo application (if not exists):\n")
	fmt.Printf("   npx create-expo-app@latest %s --template blank-typescript\n\n", opts.Name)
	fmt.Printf("3. Install dependencies:\n")
	fmt.Printf("   npm install\n\n")
	fmt.Printf("4. Start development server:\n")
	fmt.Printf("   npx expo start\n\n")
	fmt.Printf("5. Run on device/simulator:\n")
	fmt.Printf("   - Press 'i' for iOS simulator\n")
	fmt.Printf("   - Press 'a' for Android emulator\n\n")
	fmt.Printf("For more information, see README.md\n")
}
