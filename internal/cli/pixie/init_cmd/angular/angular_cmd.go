package angular

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/pixie-sh/errors-go"
	"github.com/spf13/cobra"

	"github.com/pixie-sh/pixie-cli/internal/cli/pixie/init_cmd/shared"
)

// Options holds all options for the angular init command
type Options struct {
	Name      string // Project name
	Output    string // Output directory
	Prefix    string // Angular component selector prefix
	ApiUrl    string // API base URL
	Force     bool   // Overwrite existing files
	SkipAuth  bool   // Skip authentication module
	SkipPwa   bool   // Skip PWA configuration
	SkipState bool   // Skip NGXS state management
	SkipI18n  bool   // Skip internationalization
}

// TemplateData holds the data passed to templates
type TemplateData struct {
	ProjectName string
	Prefix      string
	ApiUrl      string
	Features    map[string]bool
	Timestamp   string
}

// Cmd returns the angular init subcommand
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "angular",
		Short: "Initialize an Angular frontend project",
		Long: `Initialize a complete Angular frontend project with production-ready architecture.

This command generates a full Angular 19 project structure with:
  - Standalone components architecture
  - Authentication flows (sign-in, sign-up, password reset)
  - Base API service layer with CRUD operations
  - JWT token management with refresh capability
  - NGXS state management with storage plugins
  - SCSS design system with CSS custom properties
  - Multiple environment configurations
  - PWA capabilities
  - Internationalization support

Examples:
  # Initialize a new Angular frontend project
  pixie init angular --name my-frontend

  # Initialize with custom output directory
  pixie init angular --name my-frontend --output /path/to/output

  # Initialize with custom prefix and API URL
  pixie init angular --name my-frontend --prefix=myapp --api-url=https://api.example.com

  # Skip optional features
  pixie init angular --name my-frontend --skip-auth --skip-pwa

  # Minimal project (skip all optional features)
  pixie init angular --name my-frontend --skip-auth --skip-pwa --skip-state --skip-i18n

  # Force overwrite existing files
  pixie init angular --name my-frontend --force
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			output, _ := cmd.Flags().GetString("output")
			prefix, _ := cmd.Flags().GetString("prefix")
			apiUrl, _ := cmd.Flags().GetString("api-url")
			force, _ := cmd.Flags().GetBool("force")
			skipAuth, _ := cmd.Flags().GetBool("skip-auth")
			skipPwa, _ := cmd.Flags().GetBool("skip-pwa")
			skipState, _ := cmd.Flags().GetBool("skip-state")
			skipI18n, _ := cmd.Flags().GetBool("skip-i18n")

			opts := Options{
				Name:      name,
				Output:    output,
				Prefix:    prefix,
				ApiUrl:    apiUrl,
				Force:     force,
				SkipAuth:  skipAuth,
				SkipPwa:   skipPwa,
				SkipState: skipState,
				SkipI18n:  skipI18n,
			}

			return Run(opts)
		},
	}

	// Required flags
	cmd.Flags().String("name", "", "Project name (required)")

	// Optional flags
	cmd.Flags().String("output", ".", "Output directory")
	cmd.Flags().String("prefix", "app", "Angular component selector prefix (2-5 lowercase letters)")
	cmd.Flags().String("api-url", "http://localhost:3000", "Default API base URL")
	cmd.Flags().Bool("force", false, "Force overwrite existing files")
	cmd.Flags().Bool("skip-auth", false, "Skip authentication module")
	cmd.Flags().Bool("skip-pwa", false, "Skip PWA configuration")
	cmd.Flags().Bool("skip-state", false, "Skip NGXS state management")
	cmd.Flags().Bool("skip-i18n", false, "Skip internationalization")

	// Mark required flags
	cmd.MarkFlagRequired("name")

	return cmd
}

// Run executes the angular init command
func Run(opts Options) error {
	// Validate inputs
	if err := validateOptions(opts); err != nil {
		return errors.Wrap(err, "validation failed")
	}

	fmt.Printf("Initializing Angular frontend project: %s\n", opts.Name)
	fmt.Printf("   Output: %s\n", opts.Output)
	fmt.Printf("   Prefix: %s\n", opts.Prefix)
	fmt.Printf("   API URL: %s\n", opts.ApiUrl)
	fmt.Printf("   Features:\n")
	fmt.Printf("      Auth: %v\n", !opts.SkipAuth)
	fmt.Printf("      PWA: %v\n", !opts.SkipPwa)
	fmt.Printf("      State (NGXS): %v\n", !opts.SkipState)
	fmt.Printf("      i18n: %v\n\n", !opts.SkipI18n)

	// Generate GitHub Actions workflows
	fmt.Println("Generating GitHub Actions workflows...")
	if err := generateGitHubActions(opts); err != nil {
		return errors.Wrap(err, "failed to generate GitHub Actions")
	}

	// Generate Angular project structure
	fmt.Println("\nGenerating Angular project structure...")
	if err := generateProjectStructure(opts); err != nil {
		return errors.Wrap(err, "failed to generate project structure")
	}

	fmt.Printf("\nAngular frontend project initialized successfully!\n\n")
	printNextSteps(opts)

	return nil
}

// validateOptions validates the init options
func validateOptions(opts Options) error {
	if opts.Name == "" {
		return errors.New("project name is required")
	}

	// Validate project name (lowercase, hyphens allowed, must start with letter)
	projectNameRegex := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	if !projectNameRegex.MatchString(opts.Name) {
		return errors.New("project name must be lowercase, start with a letter, and contain only letters, numbers, and hyphens")
	}

	// Validate prefix (2-5 lowercase letters)
	prefixRegex := regexp.MustCompile(`^[a-z]{2,5}$`)
	if !prefixRegex.MatchString(opts.Prefix) {
		return errors.New("prefix must be 2-5 lowercase letters only")
	}

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

// getTemplateData creates the template data from options
func getTemplateData(opts Options) TemplateData {
	return TemplateData{
		ProjectName: opts.Name,
		Prefix:      opts.Prefix,
		ApiUrl:      opts.ApiUrl,
		Features: map[string]bool{
			"auth":  !opts.SkipAuth,
			"pwa":   !opts.SkipPwa,
			"state": !opts.SkipState,
			"i18n":  !opts.SkipI18n,
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// generateProjectStructure generates the Angular project files from templates
func generateProjectStructure(opts Options) error {
	data := getTemplateData(opts)

	// Define template mappings with conditions
	templateMappings := getTemplateMappings(opts)

	for _, mapping := range templateMappings {
		// Check condition
		if mapping.condition != nil && !mapping.condition() {
			continue
		}

		outputPath := filepath.Join(opts.Output, mapping.outputPath)

		// Check if file exists
		if !opts.Force && shared.FileExists(outputPath) {
			fmt.Printf("   WARNING: Skipping %s (file exists)\n", outputPath)
			continue
		}

		// Render template
		content, err := shared.RenderTemplate(TemplateFS, mapping.templateFile, data)
		if err != nil {
			return errors.Wrap(err, "failed to render template %s", mapping.templateFile)
		}

		// Write file
		if err := shared.WriteFile(outputPath, content, opts.Force); err != nil {
			return errors.Wrap(err, "failed to write %s", outputPath)
		}

		fmt.Printf("   Generated %s\n", outputPath)
	}

	return nil
}

// templateMapping defines a mapping from template to output file
type templateMapping struct {
	templateFile string
	outputPath   string
	condition    func() bool
}

// getTemplateMappings returns all template mappings with their conditions
func getTemplateMappings(opts Options) []templateMapping {
	hasAuth := !opts.SkipAuth
	hasPwa := !opts.SkipPwa
	hasState := !opts.SkipState
	hasI18n := !opts.SkipI18n

	return []templateMapping{
		// Root configuration files
		{"templates/angular_json.tmpl", "angular.json", nil},
		{"templates/package_json.tmpl", "package.json", nil},
		{"templates/tsconfig_json.tmpl", "tsconfig.json", nil},
		{"templates/tsconfig_app_json.tmpl", "tsconfig.app.json", nil},
		{"templates/gitignore.tmpl", ".gitignore", nil},
		{"templates/editorconfig.tmpl", ".editorconfig", nil},
		{"templates/eslintrc_json.tmpl", ".eslintrc.json", nil},
		{"templates/prettierrc_json.tmpl", ".prettierrc.json", nil},
		{"templates/readme.tmpl", "README.md", nil},

		// Entry points
		{"templates/index_html.tmpl", "src/index.html", nil},
		{"templates/main_ts.tmpl", "src/main.ts", nil},
		{"templates/styles_scss.tmpl", "src/styles.scss", nil},

		// App shell
		{"templates/app_component_ts.tmpl", "src/app/app.component.ts", nil},
		{"templates/app_component_html.tmpl", "src/app/app.component.html", nil},
		{"templates/app_component_scss.tmpl", "src/app/app.component.scss", nil},
		{"templates/app_config_ts.tmpl", "src/app/app.config.ts", nil},
		{"templates/app_routes_ts.tmpl", "src/app/app.routes.ts", nil},

		// Environments
		{"templates/environment_ts.tmpl", "src/environments/environment.ts", nil},
		{"templates/environment_local_ts.tmpl", "src/environments/environment.local.ts", nil},
		{"templates/environment_development_ts.tmpl", "src/environments/environment.development.ts", nil},
		{"templates/environment_staging_ts.tmpl", "src/environments/environment.staging.ts", nil},

		// Core - API services
		{"templates/api_service_ts.tmpl", "src/app/_core/api/api.service.ts", nil},
		{"templates/auth_api_service_ts.tmpl", "src/app/_core/api/auth-api.service.ts", func() bool { return hasAuth }},
		{"templates/user_api_service_ts.tmpl", "src/app/_core/api/user-api.service.ts", func() bool { return hasAuth }},

		// Core - Auth services
		{"templates/token_service_ts.tmpl", "src/app/_core/services/auth/token.service.ts", func() bool { return hasAuth }},
		{"templates/can_activate_ts.tmpl", "src/app/_core/services/auth/canActivate.function.ts", func() bool { return hasAuth }},

		// Core - Interceptors
		{"templates/auth_interceptor_ts.tmpl", "src/app/_core/interceptors/auth.interceptor.ts", func() bool { return hasAuth }},
		{"templates/error_interceptor_ts.tmpl", "src/app/_core/interceptors/error.interceptor.ts", nil},

		// Core - Models
		{"templates/user_model_ts.tmpl", "src/app/_core/models/user.model.ts", nil},
		{"templates/auth_model_ts.tmpl", "src/app/_core/models/auth.model.ts", func() bool { return hasAuth }},
		{"templates/api_response_model_ts.tmpl", "src/app/_core/models/api-response.model.ts", nil},

		// Core - Constants & Enums
		{"templates/api_constants_ts.tmpl", "src/app/_core/constants/api.constants.ts", nil},
		{"templates/auth_enum_ts.tmpl", "src/app/_core/enums/auth.enum.ts", func() bool { return hasAuth }},

		// Auth module
		{"templates/auth_routes_ts.tmpl", "src/app/auth/auth.routes.ts", func() bool { return hasAuth }},
		{"templates/sign_in_component_ts.tmpl", "src/app/auth/sign-in/sign-in.component.ts", func() bool { return hasAuth }},
		{"templates/sign_in_component_html.tmpl", "src/app/auth/sign-in/sign-in.component.html", func() bool { return hasAuth }},
		{"templates/sign_in_component_scss.tmpl", "src/app/auth/sign-in/sign-in.component.scss", func() bool { return hasAuth }},
		{"templates/sign_up_component_ts.tmpl", "src/app/auth/sign-up/sign-up.component.ts", func() bool { return hasAuth }},
		{"templates/sign_up_component_html.tmpl", "src/app/auth/sign-up/sign-up.component.html", func() bool { return hasAuth }},
		{"templates/sign_up_component_scss.tmpl", "src/app/auth/sign-up/sign-up.component.scss", func() bool { return hasAuth }},
		{"templates/reset_password_component_ts.tmpl", "src/app/auth/reset-password/reset-password.component.ts", func() bool { return hasAuth }},
		{"templates/reset_password_component_html.tmpl", "src/app/auth/reset-password/reset-password.component.html", func() bool { return hasAuth }},
		{"templates/reset_password_component_scss.tmpl", "src/app/auth/reset-password/reset-password.component.scss", func() bool { return hasAuth }},

		// State management
		{"templates/auth_state_ts.tmpl", "src/app/_core/stores/auth/auth.state.ts", func() bool { return hasState && hasAuth }},
		{"templates/auth_actions_ts.tmpl", "src/app/_core/stores/auth/auth.actions.ts", func() bool { return hasState && hasAuth }},
		{"templates/user_state_ts.tmpl", "src/app/_core/stores/user/user.state.ts", func() bool { return hasState && hasAuth }},
		{"templates/user_actions_ts.tmpl", "src/app/_core/stores/user/user.actions.ts", func() bool { return hasState && hasAuth }},

		// Shared components
		{"templates/button_component_ts.tmpl", "src/app/shared/components/button/button.component.ts", nil},
		{"templates/button_component_html.tmpl", "src/app/shared/components/button/button.component.html", nil},
		{"templates/button_component_scss.tmpl", "src/app/shared/components/button/button.component.scss", nil},
		{"templates/input_component_ts.tmpl", "src/app/shared/components/input/input.component.ts", nil},
		{"templates/input_component_html.tmpl", "src/app/shared/components/input/input.component.html", nil},
		{"templates/input_component_scss.tmpl", "src/app/shared/components/input/input.component.scss", nil},
		{"templates/loader_component_ts.tmpl", "src/app/shared/components/loader/loader.component.ts", nil},
		{"templates/loader_component_html.tmpl", "src/app/shared/components/loader/loader.component.html", nil},
		{"templates/loader_component_scss.tmpl", "src/app/shared/components/loader/loader.component.scss", nil},
		{"templates/toast_service_ts.tmpl", "src/app/shared/services/toast.service.ts", nil},
		{"templates/toast_component_ts.tmpl", "src/app/shared/components/toast/toast.component.ts", nil},
		{"templates/toast_component_html.tmpl", "src/app/shared/components/toast/toast.component.html", nil},
		{"templates/toast_component_scss.tmpl", "src/app/shared/components/toast/toast.component.scss", nil},
		{"templates/modal_component_ts.tmpl", "src/app/shared/components/modal/modal.component.ts", nil},
		{"templates/modal_component_html.tmpl", "src/app/shared/components/modal/modal.component.html", nil},
		{"templates/modal_component_scss.tmpl", "src/app/shared/components/modal/modal.component.scss", nil},
		{"templates/navbar_component_ts.tmpl", "src/app/shared/components/navbar/navbar.component.ts", nil},
		{"templates/navbar_component_html.tmpl", "src/app/shared/components/navbar/navbar.component.html", nil},
		{"templates/navbar_component_scss.tmpl", "src/app/shared/components/navbar/navbar.component.scss", nil},
		{"templates/sidebar_component_ts.tmpl", "src/app/shared/components/sidebar/sidebar.component.ts", nil},
		{"templates/sidebar_component_html.tmpl", "src/app/shared/components/sidebar/sidebar.component.html", nil},
		{"templates/sidebar_component_scss.tmpl", "src/app/shared/components/sidebar/sidebar.component.scss", nil},
		{"templates/click_outside_directive_ts.tmpl", "src/app/shared/directives/click-outside.directive.ts", nil},

		// Layouts
		{"templates/main_layout_component_ts.tmpl", "src/app/layouts/main-layout/main-layout.component.ts", nil},
		{"templates/main_layout_component_html.tmpl", "src/app/layouts/main-layout/main-layout.component.html", nil},
		{"templates/main_layout_component_scss.tmpl", "src/app/layouts/main-layout/main-layout.component.scss", nil},
		{"templates/split_layout_component_ts.tmpl", "src/app/layouts/split-layout/split-layout.component.ts", func() bool { return hasAuth }},
		{"templates/split_layout_component_html.tmpl", "src/app/layouts/split-layout/split-layout.component.html", func() bool { return hasAuth }},
		{"templates/split_layout_component_scss.tmpl", "src/app/layouts/split-layout/split-layout.component.scss", func() bool { return hasAuth }},

		// Dashboard
		{"templates/dashboard_component_ts.tmpl", "src/app/dashboard/dashboard.component.ts", nil},
		{"templates/dashboard_component_html.tmpl", "src/app/dashboard/dashboard.component.html", nil},
		{"templates/dashboard_component_scss.tmpl", "src/app/dashboard/dashboard.component.scss", nil},

		// SCSS Design System
		{"templates/variables_scss.tmpl", "src/styles/_variables.scss", nil},
		{"templates/design_system_scss.tmpl", "src/styles/_design-system.scss", nil},
		{"templates/typography_scss.tmpl", "src/styles/_typography.scss", nil},
		{"templates/base_scss.tmpl", "src/styles/_base.scss", nil},
		{"templates/utilities_scss.tmpl", "src/styles/_utilities.scss", nil},
		{"templates/scrollbar_scss.tmpl", "src/styles/_scrollbar.scss", nil},
		{"templates/styles_index_scss.tmpl", "src/styles/_index.scss", nil},
		{"templates/button_styles_scss.tmpl", "src/styles/components/_button.scss", nil},
		{"templates/card_styles_scss.tmpl", "src/styles/components/_card.scss", nil},

		// PWA
		{"templates/ngsw_config_json.tmpl", "ngsw-config.json", func() bool { return hasPwa }},
		{"templates/manifest_webmanifest.tmpl", "src/manifest.webmanifest", func() bool { return hasPwa }},

		// i18n
		{"templates/i18n_en_json.tmpl", "src/assets/i18n/en.json", func() bool { return hasI18n }},
	}
}

// printNextSteps prints the next steps after initialization
func printNextSteps(opts Options) {
	fmt.Printf("Next steps:\n\n")
	fmt.Printf("1. Navigate to the project directory:\n")
	fmt.Printf("   cd %s\n\n", opts.Output)
	fmt.Printf("2. Install dependencies:\n")
	fmt.Printf("   npm install\n\n")
	fmt.Printf("3. Start development server:\n")
	fmt.Printf("   ng serve\n\n")
	fmt.Printf("4. Open http://localhost:4200\n\n")
	if !opts.SkipAuth {
		fmt.Printf("5. Test authentication flows:\n")
		fmt.Printf("   - Sign up: http://localhost:4200/auth/sign-up\n")
		fmt.Printf("   - Sign in: http://localhost:4200/auth/sign-in\n")
		fmt.Printf("   - Reset password: http://localhost:4200/auth/reset-password\n\n")
	}
	fmt.Printf("For more information, see README.md\n")
}
