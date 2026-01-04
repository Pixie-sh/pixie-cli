package golang

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/pixie-sh/errors-go"
	"github.com/spf13/cobra"

	"github.com/pixie-sh/pixie-cli/internal/cli/pixie/init_cmd/shared"
)

// Options holds all options for the golang init command
type Options struct {
	Name          string   // Project name
	Module        string   // Go module path
	Output        string   // Output directory
	Microservices []string // Microservices to generate
	Force         bool     // Overwrite existing files
	ProjectMS     string   // Custom name for the project microservice
}

// MicroserviceConfig defines configuration for a microservice to generate
type MicroserviceConfig struct {
	Name        string
	Port        int
	MetricsPort int
	Domains     []string
	Features    []string
}

// DomainInfo holds domain information for template generation
type DomainInfo struct {
	Name      string
	NameCamel string
}

// TemplateData holds the data passed to templates
type TemplateData struct {
	ProjectName         string
	ProjectMS           string
	ServiceName         string
	ServiceNameCamel    string
	DomainName          string
	DomainNameCamel     string
	ModuleName          string
	RepositoryName      string
	RepositoryNameCamel string
	EntityName          string
	EntityNameCamel     string
	Domains             []DomainInfo
	Features            map[string]bool
	Port                int
	MetricsPort         int
	Timestamp           string
	MigrationTimestamp  string

	// Database configuration
	DatabaseHost                  string
	DatabasePort                  int
	DatabaseName                  string
	DatabaseUsername              string
	DatabasePassword              string
	DatabaseSSLMode               string
	DatabaseMaxOpenConnections    int
	DatabaseMaxIdleConnections    int
	DatabaseConnectionMaxLifetime string

	// Redis configuration
	RedisHost     string
	RedisPort     int
	RedisPassword string
	RedisDatabase int

	// Security configuration
	JWTSecretKey string
	AdminAPIKey  string
}

// Cmd returns the golang init subcommand
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "golang",
		Short: "Initialize a Go backend project with microservices architecture",
		Long: `Initialize a complete Go backend project with microservices architecture.

This command generates a full project structure with:
  - ms_authentication: User registration, login, JWT tokens, sessions
  - ms_notifications: Email and push notification handling
  - ms_project (customizable): Core business logic microservice

The generated project includes:
  - Entry points (cmd/ms/ms_*/application.go)
  - Microservice implementations (internal/ms/ms_*/)
  - Domain layers (internal/domains/*)
  - Configuration files (misc/configs/*.json)
  - Infrastructure (bundles/, infra/)
  - go.mod, Makefile, docker-compose.yaml
  - GitHub Actions CI/CD workflows
  - E2E test infrastructure

Examples:
  # Initialize a new Go backend project
  pixie init golang --name my-backend --module github.com/company/my-backend

  # Initialize with custom output directory
  pixie init golang --name my-backend --module github.com/company/my-backend --output /path/to/output

  # Initialize with custom project microservice name
  pixie init golang --name my-backend --module github.com/company/my-backend --project-ms orders

  # Initialize specific microservices only
  pixie init golang --name my-backend --module github.com/company/my-backend --microservices authentication,notifications

  # Force overwrite existing files
  pixie init golang --name my-backend --module github.com/company/my-backend --force
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			module, _ := cmd.Flags().GetString("module")
			output, _ := cmd.Flags().GetString("output")
			microservices, _ := cmd.Flags().GetStringSlice("microservices")
			force, _ := cmd.Flags().GetBool("force")
			projectMS, _ := cmd.Flags().GetString("project-ms")

			opts := Options{
				Name:          name,
				Module:        module,
				Output:        output,
				Microservices: microservices,
				Force:         force,
				ProjectMS:     projectMS,
			}

			return Run(opts)
		},
	}

	// Required flags
	cmd.Flags().String("name", "", "Project name (required)")
	cmd.Flags().String("module", "", "Go module path (required)")

	// Optional flags
	cmd.Flags().String("output", ".", "Output directory")
	cmd.Flags().StringSlice("microservices", []string{"authentication", "notifications", "project"}, "Microservices to generate")
	cmd.Flags().Bool("force", false, "Force overwrite existing files")
	cmd.Flags().String("project-ms", "project", "Custom name for the project microservice (default: project)")

	// Mark required flags
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("module")

	return cmd
}

// Run executes the golang init command
func Run(opts Options) error {
	// Validate inputs
	if err := validateOptions(opts); err != nil {
		return errors.Wrap(err, "validation failed")
	}

	fmt.Printf("Initializing Go backend project: %s\n", opts.Name)
	fmt.Printf("   Module: %s\n", opts.Module)
	fmt.Printf("   Output: %s\n", opts.Output)
	fmt.Printf("   Microservices: %s\n\n", strings.Join(opts.Microservices, ", "))

	// Generate GitHub Actions workflows first
	fmt.Println("Generating GitHub Actions workflows...")
	if err := generateGitHubActions(opts); err != nil {
		return errors.Wrap(err, "failed to generate GitHub Actions")
	}

	// Define microservice configurations
	msConfigs := getDefaultMicroserviceConfigs(opts.ProjectMS)

	// Filter to requested microservices
	selectedConfigs := filterMicroservices(msConfigs, opts.Microservices)

	// Collect all domains
	allDomains := collectAllDomains(selectedConfigs)

	// Generate project-level infrastructure
	fmt.Println("\nGenerating project infrastructure...")
	if err := generateProjectInfrastructure(opts, allDomains, selectedConfigs); err != nil {
		return errors.Wrap(err, "failed to generate project infrastructure")
	}

	// Generate each microservice
	for _, msConfig := range selectedConfigs {
		fmt.Printf("\nGenerating microservice: %s\n", msConfig.Name)
		if err := generateMicroserviceFromConfig(msConfig, opts); err != nil {
			return errors.Wrap(err, "failed to generate microservice %s", msConfig.Name)
		}
	}

	fmt.Printf("\nGo backend project initialized successfully!\n\n")
	printNextSteps(opts)

	return nil
}

// validateOptions validates the init options
func validateOptions(opts Options) error {
	if opts.Name == "" {
		return errors.New("project name is required")
	}

	if opts.Module == "" {
		return errors.New("module path is required")
	}

	if !strings.Contains(opts.Module, "/") {
		return errors.New("module path must be a valid Go module path (e.g., github.com/company/project)")
	}

	validMS := map[string]bool{
		"authentication": true,
		"notifications":  true,
		"project":        true,
	}

	for _, ms := range opts.Microservices {
		if !validMS[ms] {
			return errors.New("invalid microservice: %s. Valid options: authentication, notifications, project", ms)
		}
	}

	return nil
}

// generateGitHubActions generates GitHub Actions workflow files
func generateGitHubActions(opts Options) error {
	workflowsDir := filepath.Join(opts.Output, ".github", "workflows")

	// Create workflows directory
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create workflows directory")
	}

	// Data for templates
	data := struct {
		ProjectName string
		ModuleName  string
	}{
		ProjectName: opts.Name,
		ModuleName:  opts.Module,
	}

	// Generate tests.yaml
	testsPath := filepath.Join(workflowsDir, "tests.yaml")
	if !opts.Force && shared.FileExists(testsPath) {
		fmt.Printf("   WARNING: Skipping %s (file exists, use --force to overwrite)\n", testsPath)
	} else {
		content, err := shared.RenderTemplate(shared.GitHubActionsTemplates, "templates/github_actions/tests.yaml.tmpl", data)
		if err != nil {
			return errors.Wrap(err, "failed to render tests.yaml template")
		}
		if err := shared.WriteFile(testsPath, content, opts.Force); err != nil {
			return errors.Wrap(err, "failed to write tests.yaml")
		}
		fmt.Printf("   Generated %s\n", testsPath)
	}

	// Generate build.yaml
	buildPath := filepath.Join(workflowsDir, "build.yaml")
	if !opts.Force && shared.FileExists(buildPath) {
		fmt.Printf("   WARNING: Skipping %s (file exists, use --force to overwrite)\n", buildPath)
	} else {
		content, err := shared.RenderTemplate(shared.GitHubActionsTemplates, "templates/github_actions/build.yaml.tmpl", data)
		if err != nil {
			return errors.Wrap(err, "failed to render build.yaml template")
		}
		if err := shared.WriteFile(buildPath, content, opts.Force); err != nil {
			return errors.Wrap(err, "failed to write build.yaml")
		}
		fmt.Printf("   Generated %s\n", buildPath)
	}

	return nil
}

// getDefaultMicroserviceConfigs returns the default microservice configurations
func getDefaultMicroserviceConfigs(projectMSName string) []MicroserviceConfig {
	if projectMSName == "" {
		projectMSName = "project"
	}

	return []MicroserviceConfig{
		{
			Name:        "authentication",
			Port:        3001,
			MetricsPort: 3101,
			Domains:     []string{"authentication"},
			Features:    []string{"database", "auth", "tokens", "metrics", "e2e"},
		},
		{
			Name:        "notifications",
			Port:        3002,
			MetricsPort: 3102,
			Domains:     []string{"notifications"},
			Features:    []string{"database", "auth", "tokens", "events", "metrics", "e2e", "cache"},
		},
		{
			Name:        projectMSName,
			Port:        3000,
			MetricsPort: 3100,
			Domains:     []string{projectMSName},
			Features:    []string{"database", "auth", "tokens", "events", "metrics", "e2e"},
		},
	}
}

// filterMicroservices filters the microservice configs
func filterMicroservices(configs []MicroserviceConfig, requested []string) []MicroserviceConfig {
	requestedMap := make(map[string]bool)
	for _, ms := range requested {
		requestedMap[ms] = true
	}

	var filtered []MicroserviceConfig
	for _, config := range configs {
		if requestedMap[config.Name] || (config.Name != "authentication" && config.Name != "notifications" && requestedMap["project"]) {
			filtered = append(filtered, config)
		}
	}

	return filtered
}

// collectAllDomains collects all unique domains
func collectAllDomains(configs []MicroserviceConfig) []DomainInfo {
	seen := make(map[string]bool)
	var domains []DomainInfo

	for _, config := range configs {
		for _, domain := range config.Domains {
			if !seen[domain] {
				seen[domain] = true
				domains = append(domains, DomainInfo{
					Name:      domain,
					NameCamel: shared.ToCamelCase(domain),
				})
			}
		}
	}

	return domains
}

// generateProjectInfrastructure generates project-level files
func generateProjectInfrastructure(opts Options, domains []DomainInfo, configs []MicroserviceConfig) error {
	hasAuth := false
	hasEvents := false
	hasNotifications := false

	for _, config := range configs {
		for _, feature := range config.Features {
			if feature == "auth" || feature == "tokens" {
				hasAuth = true
			}
			if feature == "events" {
				hasEvents = true
			}
		}
		if config.Name == "notifications" {
			hasNotifications = true
		}
	}

	data := TemplateData{
		ProjectName: opts.Name,
		ProjectMS:   opts.ProjectMS,
		ModuleName:  opts.Module,
		Domains:     domains,
		Features: map[string]bool{
			"auth":          hasAuth,
			"events":        hasEvents,
			"notifications": hasNotifications,
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	projectTemplates := []struct {
		templateFile string
		outputPath   string
		condition    func() bool
	}{
		{"go_mod.go.tmpl", filepath.Join(opts.Output, "go.mod"), nil},
		{"makefile.tmpl", filepath.Join(opts.Output, "Makefile"), nil},
		{"common_mk.tmpl", filepath.Join(opts.Output, "common.mk"), nil},
		{"dev_mk.tmpl", filepath.Join(opts.Output, "dev.mk"), nil},
		{"dockers_mk.tmpl", filepath.Join(opts.Output, "dockers.mk"), nil},
		{"gitignore.tmpl", filepath.Join(opts.Output, ".gitignore"), nil},
		{"readme.tmpl", filepath.Join(opts.Output, "README.md"), nil},
		{"env_example.tmpl", filepath.Join(opts.Output, ".env.example"), nil},
		{"docker_compose.yaml.tmpl", filepath.Join(opts.Output, "misc/dockerfiles/docker-compose.yaml"), nil},
		{"injection_tokens.go.tmpl", filepath.Join(opts.Output, "infra/di/injection_tokens.go"), nil},
		{"version.go.tmpl", filepath.Join(opts.Output, "infra/version/version.go"), nil},
		{"bundles_database.go.tmpl", filepath.Join(opts.Output, "bundles/database_bundle.go"), nil},
		{"bundles_http_server.go.tmpl", filepath.Join(opts.Output, "bundles/http_server_bundle.go"), nil},
		{"bundles_registry.go.tmpl", filepath.Join(opts.Output, "bundles/registry.go"), nil},
		{"bundles_token_services.go.tmpl", filepath.Join(opts.Output, "bundles/token_bundle.go"), func() bool { return hasAuth }},
		{"bundles_authorization_gates.go.tmpl", filepath.Join(opts.Output, "bundles/authorization_gates_bundle.go"), func() bool { return hasAuth }},
		{"gates_is_authenticated.go.tmpl", filepath.Join(opts.Output, "infra/gates/is_authenticated.go"), func() bool { return hasAuth }},
		{"gates_authorization_token.go.tmpl", filepath.Join(opts.Output, "infra/gates/authorization_token.go"), func() bool { return hasAuth }},
		{"session_manager.go.tmpl", filepath.Join(opts.Output, "infra/session_manager/session_manager.go"), func() bool { return hasAuth }},
		{"session_jwt_service.go.tmpl", filepath.Join(opts.Output, "infra/session_manager/jwt_service.go"), func() bool { return hasAuth }},
		{"session_jwt_functions.go.tmpl", filepath.Join(opts.Output, "infra/session_manager/jwt_functions.go"), func() bool { return hasAuth }},
		{"session_service.go.tmpl", filepath.Join(opts.Output, "infra/session_manager/session_service.go"), func() bool { return hasAuth }},
		{"session_functions.go.tmpl", filepath.Join(opts.Output, "infra/session_manager/session_functions.go"), func() bool { return hasAuth }},
		{"session_singleton.go.tmpl", filepath.Join(opts.Output, "infra/session_manager/singleton.go"), func() bool { return hasAuth }},
		{"session_manager_models.go.tmpl", filepath.Join(opts.Output, "pkg/models/session_manager_models/session_manager_models.go"), func() bool { return hasAuth }},
		{"pkg_context.go.tmpl", filepath.Join(opts.Output, "pkg/context/context.go"), func() bool { return hasAuth }},
		{"pkg_context_http.go.tmpl", filepath.Join(opts.Output, "pkg/context/http/context.go"), func() bool { return hasAuth }},
		{"message_packs.go.tmpl", filepath.Join(opts.Output, "infra/message_packs/message_packs.go"), nil},
		{"apis_registry.go.tmpl", filepath.Join(opts.Output, "infra/apis/registry.go"), nil},
		{"e2e_utils.go.tmpl", filepath.Join(opts.Output, "internal/e2e_tests/e2e_utils/e2e_utils.go"), nil},
		{"uidgen.go.tmpl", filepath.Join(opts.Output, "infra/uidgen/uidgen.go"), func() bool { return hasAuth }},
		{"uidgen_checks.go.tmpl", filepath.Join(opts.Output, "infra/uidgen/checks.go"), func() bool { return hasAuth }},
		{"event.go.tmpl", filepath.Join(opts.Output, "infra/event/event.go"), func() bool { return hasAuth }},
		{"auth_adapters.go.tmpl", filepath.Join(opts.Output, "internal/adapters/authentication_adapters/adapters.go"), func() bool { return hasAuth }},
		{"rate_limiter_bundle.go.tmpl", filepath.Join(opts.Output, "bundles/rate_limiter_bundle.go"), func() bool { return hasAuth }},
	}

	for _, mapping := range projectTemplates {
		if mapping.condition != nil && !mapping.condition() {
			continue
		}

		if !opts.Force && shared.FileExists(mapping.outputPath) {
			fmt.Printf("   WARNING: Skipping %s (file exists)\n", mapping.outputPath)
			continue
		}

		fmt.Printf("   Generating %s\n", mapping.outputPath)

		if err := generateFileFromTemplate(mapping.templateFile, mapping.outputPath, data); err != nil {
			return errors.Wrap(err, "failed to generate %s", mapping.outputPath)
		}
	}

	return nil
}

// generateMicroserviceFromConfig generates a microservice from its configuration
func generateMicroserviceFromConfig(msConfig MicroserviceConfig, opts Options) error {
	features := make(map[string]bool)
	for _, f := range msConfig.Features {
		features[f] = true
	}

	features = resolveFeatureDependencies(features)

	domainName := msConfig.Name
	if len(msConfig.Domains) > 0 {
		domainName = msConfig.Domains[0]
	}

	data := TemplateData{
		ProjectName:         opts.Name,
		ProjectMS:           opts.ProjectMS,
		ServiceName:         msConfig.Name,
		ServiceNameCamel:    shared.ToCamelCase(msConfig.Name),
		DomainName:          domainName,
		DomainNameCamel:     shared.ToCamelCase(domainName),
		ModuleName:          opts.Module,
		RepositoryName:      domainName,
		RepositoryNameCamel: shared.ToCamelCase(domainName),
		EntityName:          domainName,
		EntityNameCamel:     shared.ToCamelCase(domainName),
		Features:            features,
		Port:                msConfig.Port,
		MetricsPort:         msConfig.MetricsPort,
		Timestamp:           time.Now().Format(time.RFC3339),
		MigrationTimestamp:  fmt.Sprintf("%d", time.Now().Unix()),

		DatabaseHost:                  "localhost",
		DatabasePort:                  5432,
		DatabaseName:                  "backend_system",
		DatabaseUsername:              "postgres",
		DatabasePassword:              "password",
		DatabaseSSLMode:               "disable",
		DatabaseMaxOpenConnections:    100,
		DatabaseMaxIdleConnections:    10,
		DatabaseConnectionMaxLifetime: "1h",

		RedisHost:     "localhost",
		RedisPort:     6379,
		RedisPassword: "",
		RedisDatabase: 0,

		JWTSecretKey: "your-secret-key",
		AdminAPIKey:  "admin-api-key",
	}

	fmt.Printf("   Domain: %s\n", data.DomainName)
	fmt.Printf("   Features: %s\n", strings.Join(getEnabledFeatures(features), ", "))
	fmt.Printf("   Port: %d (metrics: %d)\n", data.Port, data.MetricsPort)

	switch data.ServiceName {
	case "authentication":
		return generateAuthenticationMicroservice(data, opts)
	case "notifications":
		return generateNotificationsMicroservice(data, opts)
	default:
		return generateGenericMicroservice(data, opts)
	}
}

// generateAuthenticationMicroservice generates auth microservice files
func generateAuthenticationMicroservice(data TemplateData, opts Options) error {
	basePath := opts.Output
	domainPath := filepath.Join(basePath, "internal/domains/authentication")
	msPath := filepath.Join(basePath, "internal/ms/ms_authentication")
	modelsPath := filepath.Join(basePath, "pkg/models/auth")

	templateMappings := []struct {
		templateFile string
		outputPath   string
	}{
		{"cmd_application.go.tmpl", filepath.Join(basePath, "cmd/ms/ms_authentication/application.go")},
		{"auth_microservice.go.tmpl", filepath.Join(msPath, "microservice.go")},
		{"ms_registry.go.tmpl", filepath.Join(msPath, "registry.go")},
		{"auth_http_controllers.go.tmpl", filepath.Join(msPath, "http_controllers.go")},
		{"auth_http_bo_controllers.go.tmpl", filepath.Join(msPath, "http_bo_controllers.go")},
		{"auth_entity_user.go.tmpl", filepath.Join(domainPath, "authentication_data_layer/auth_entities/user.go")},
		{"auth_entity_otp.go.tmpl", filepath.Join(domainPath, "authentication_data_layer/auth_entities/otp.go")},
		{"auth_entity_password_resets.go.tmpl", filepath.Join(domainPath, "authentication_data_layer/auth_entities/password_resets.go")},
		{"auth_migration_user.go.tmpl", filepath.Join(domainPath, fmt.Sprintf("authentication_data_layer/auth_migrations/%s1_create_user_table.go", data.MigrationTimestamp))},
		{"auth_migration_password_resets.go.tmpl", filepath.Join(domainPath, fmt.Sprintf("authentication_data_layer/auth_migrations/%s2_create_password_resets.go", data.MigrationTimestamp))},
		{"auth_migration_otp.go.tmpl", filepath.Join(domainPath, fmt.Sprintf("authentication_data_layer/auth_migrations/%s3_create_otp_table.go", data.MigrationTimestamp))},
		{"auth_migrations.go.tmpl", filepath.Join(domainPath, "authentication_data_layer/auth_migrations/migrations.go")},
		{"auth_repo_user.go.tmpl", filepath.Join(domainPath, "authentication_data_layer/auth_repositories/user_repository.go")},
		{"auth_repo_password.go.tmpl", filepath.Join(domainPath, "authentication_data_layer/auth_repositories/password_repository.go")},
		{"auth_repo_otp.go.tmpl", filepath.Join(domainPath, "authentication_data_layer/auth_repositories/otp_repository.go")},
		{"auth_data_layer.go.tmpl", filepath.Join(domainPath, "authentication_data_layer/authentication_data_layer.go")},
		{"auth_user_data_layer.go.tmpl", filepath.Join(domainPath, "authentication_data_layer/user_data_layer.go")},
		{"auth_password_data_layer.go.tmpl", filepath.Join(domainPath, "authentication_data_layer/password_data_layer.go")},
		{"auth_otp_data_layer.go.tmpl", filepath.Join(domainPath, "authentication_data_layer/otp_data_layer.go")},
		{"auth_registration_data_layer.go.tmpl", filepath.Join(domainPath, "authentication_data_layer/registration_data_layer.go")},
		{"auth_business_layer.go.tmpl", filepath.Join(domainPath, "authentication_business_layer/authentication_business_layer.go")},
		{"auth_password_business_layer.go.tmpl", filepath.Join(domainPath, "authentication_business_layer/password_business_layer.go")},
		{"auth_registration_business_layer.go.tmpl", filepath.Join(domainPath, "authentication_business_layer/registration_business_layer.go")},
		{"auth_user_profile_business_layer.go.tmpl", filepath.Join(domainPath, "authentication_business_layer/user_profile_business_layer.go")},
		{"auth_login_service.go.tmpl", filepath.Join(domainPath, "authentication_services/login_service.go")},
		{"auth_password_service.go.tmpl", filepath.Join(domainPath, "authentication_services/password_service.go")},
		{"auth_otp_service.go.tmpl", filepath.Join(domainPath, "authentication_services/user_otp_service.go")},
		{"auth_users_service.go.tmpl", filepath.Join(domainPath, "authentication_services/users_service.go")},
		{"auth_state_machine.go.tmpl", filepath.Join(domainPath, "authentication_services/user_state_machine.go")},
		{"auth_domain_registry.go.tmpl", filepath.Join(domainPath, "registry.go")},
		{"auth_models.go.tmpl", filepath.Join(modelsPath, "auth_models.go")},
		{"config.json.tmpl", filepath.Join(basePath, "misc/configs/ms_authentication.json")},
		{"e2e_configuration.go.tmpl", filepath.Join(basePath, "internal/e2e_tests/e2e_ms_authentication_tests/configuration.go")},
		{"e2e_bootstrap_test.go.tmpl", filepath.Join(basePath, "internal/e2e_tests/e2e_ms_authentication_tests/ms_authentication_bootstrap_test.go")},
	}

	for _, mapping := range templateMappings {
		if !opts.Force && shared.FileExists(mapping.outputPath) {
			fmt.Printf("   WARNING: Skipping %s (file exists)\n", mapping.outputPath)
			continue
		}

		fmt.Printf("   Generating %s\n", mapping.outputPath)

		if err := generateFileFromTemplate(mapping.templateFile, mapping.outputPath, data); err != nil {
			return errors.Wrap(err, "failed to generate %s", mapping.outputPath)
		}
	}

	return nil
}

// generateNotificationsMicroservice generates notifications microservice files
func generateNotificationsMicroservice(data TemplateData, opts Options) error {
	basePath := opts.Output
	domainPath := filepath.Join(basePath, "internal/domains/notifications")
	msPath := filepath.Join(basePath, "internal/ms/ms_notifications")
	modelsPath := filepath.Join(basePath, "pkg/models/notifications")
	adaptersPath := filepath.Join(basePath, "internal/adapters/notification_adapters")

	templateMappings := []struct {
		templateFile string
		outputPath   string
	}{
		{"cmd_application.go.tmpl", filepath.Join(basePath, "cmd/ms/ms_notifications/application.go")},
		{"microservice.go.tmpl", filepath.Join(msPath, "microservice.go")},
		{"ms_registry.go.tmpl", filepath.Join(msPath, "registry.go")},
		{"notif_http_controllers.go.tmpl", filepath.Join(msPath, "http_controllers.go")},
		{"notif_entity_template.go.tmpl", filepath.Join(domainPath, "notifications_data_layer/notifications_entities/template.go")},
		{"notif_entity_action.go.tmpl", filepath.Join(domainPath, "notifications_data_layer/notifications_entities/action.go")},
		{"notif_entity_activity_log.go.tmpl", filepath.Join(domainPath, "notifications_data_layer/notifications_entities/activity_log.go")},
		{"notif_entity_firebase_token.go.tmpl", filepath.Join(domainPath, "notifications_data_layer/notifications_entities/firebase_token.go")},
		{"notif_repo_templates.go.tmpl", filepath.Join(domainPath, "notifications_data_layer/notifications_repositories/templates_repository.go")},
		{"notif_repo_actions.go.tmpl", filepath.Join(domainPath, "notifications_data_layer/notifications_repositories/actions_repository.go")},
		{"notif_repo_activity_logs.go.tmpl", filepath.Join(domainPath, "notifications_data_layer/notifications_repositories/activity_logs_repository.go")},
		{"notif_repo_firebase_tokens.go.tmpl", filepath.Join(domainPath, "notifications_data_layer/notifications_repositories/firebase_tokens_repository.go")},
		{"notif_data_layer.go.tmpl", filepath.Join(domainPath, "notifications_data_layer/notifications_data_layer.go")},
		{"notif_email_service.go.tmpl", filepath.Join(domainPath, "notifications_services/email_service.go")},
		{"notif_firebase_service.go.tmpl", filepath.Join(domainPath, "notifications_services/firebase_service.go")},
		{"notif_fcm_token_service.go.tmpl", filepath.Join(domainPath, "notifications_services/fcm_token_service.go")},
		{"notif_sms_service.go.tmpl", filepath.Join(domainPath, "notifications_services/sms_service.go")},
		{"notif_helpers_service.go.tmpl", filepath.Join(domainPath, "notifications_services/helpers_service.go")},
		{"notif_business_layer.go.tmpl", filepath.Join(domainPath, "notifications_business_layer/notifications_business_layer.go")},
		{"notif_registry.go.tmpl", filepath.Join(domainPath, "registry.go")},
		{"notif_adapters.go.tmpl", filepath.Join(adaptersPath, "adapters.go")},
		{"notif_adapter_actions.go.tmpl", filepath.Join(adaptersPath, "actions_adapter.go")},
		{"notif_adapter_activity_logs.go.tmpl", filepath.Join(adaptersPath, "activity_logs_adapter.go")},
		{"notif_adapter_templates.go.tmpl", filepath.Join(adaptersPath, "templates_adapter.go")},
		{"notif_models.go.tmpl", filepath.Join(modelsPath, "notifications_models.go")},
		{"config.json.tmpl", filepath.Join(basePath, "misc/configs/ms_notifications.json")},
		{"e2e_configuration.go.tmpl", filepath.Join(basePath, "internal/e2e_tests/e2e_ms_notifications_tests/configuration.go")},
		{"e2e_bootstrap_test.go.tmpl", filepath.Join(basePath, "internal/e2e_tests/e2e_ms_notifications_tests/ms_notifications_bootstrap_test.go")},
	}

	for _, mapping := range templateMappings {
		if !opts.Force && shared.FileExists(mapping.outputPath) {
			fmt.Printf("   WARNING: Skipping %s (file exists)\n", mapping.outputPath)
			continue
		}

		fmt.Printf("   Generating %s\n", mapping.outputPath)

		if err := generateFileFromTemplate(mapping.templateFile, mapping.outputPath, data); err != nil {
			return errors.Wrap(err, "failed to generate %s", mapping.outputPath)
		}
	}

	return nil
}

// generateGenericMicroservice generates files for a generic microservice
func generateGenericMicroservice(data TemplateData, opts Options) error {
	templateMappings := []struct {
		templateFile string
		outputPath   string
		condition    func() bool
	}{
		{"cmd_application.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("cmd/ms/ms_%s/application.go", data.ServiceName)), nil},
		{"microservice.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("internal/ms/ms_%s/microservice.go", data.ServiceName)), nil},
		{"ms_registry.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("internal/ms/ms_%s/registry.go", data.ServiceName)), nil},
		{"http_controllers.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("internal/ms/ms_%s/http_controllers.go", data.ServiceName)), nil},
		{"http_bo_controllers.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("internal/ms/ms_%s/http_bo_controllers.go", data.ServiceName)), func() bool { return data.Features["backoffice"] }},
		{"business_layer.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/%s_business_layer/%s_business_layer.go", data.DomainName, data.DomainName, data.DomainName)), nil},
		{"data_layer.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/%s_data_layer/%s_data_layer.go", data.DomainName, data.DomainName, data.DomainName)), func() bool { return data.Features["database"] }},
		{"services.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/%s_services/%s_service.go", data.DomainName, data.DomainName, data.DomainName)), nil},
		{"domain_registry.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/registry.go", data.DomainName)), nil},
		{"entities.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/%s_data_layer/%s_entities/%s.go", data.DomainName, data.DomainName, data.DomainName, data.DomainName)), func() bool { return data.Features["database"] }},
		{"repositories.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/%s_data_layer/%s_repositories/%s_repository.go", data.DomainName, data.DomainName, data.DomainName, data.DomainName)), func() bool { return data.Features["database"] }},
		{"entity_migration.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/%s_data_layer/%s_migrations/%s_create_%s_table.go", data.DomainName, data.DomainName, data.DomainName, data.MigrationTimestamp, data.DomainName)), func() bool { return data.Features["database"] }},
		{"domain_migrations.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/%s_data_layer/%s_migrations/migrations.go", data.DomainName, data.DomainName, data.DomainName)), func() bool { return data.Features["database"] }},
		{"models.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("pkg/models/%s/%s_models.go", data.DomainName, data.DomainName)), nil},
		{"config.json.tmpl", filepath.Join(opts.Output, fmt.Sprintf("misc/configs/ms_%s.json", data.ServiceName)), nil},
		{"e2e_configuration.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("internal/e2e_tests/e2e_ms_%s_tests/configuration.go", data.ServiceName)), func() bool { return data.Features["e2e"] }},
		{"e2e_bootstrap_test.go.tmpl", filepath.Join(opts.Output, fmt.Sprintf("internal/e2e_tests/e2e_ms_%s_tests/ms_%s_bootstrap_test.go", data.ServiceName, data.ServiceName)), func() bool { return data.Features["e2e"] }},
	}

	for _, mapping := range templateMappings {
		if mapping.condition != nil && !mapping.condition() {
			continue
		}

		if !opts.Force && shared.FileExists(mapping.outputPath) {
			fmt.Printf("   WARNING: Skipping %s (file exists)\n", mapping.outputPath)
			continue
		}

		fmt.Printf("   Generating %s\n", mapping.outputPath)

		if err := generateFileFromTemplate(mapping.templateFile, mapping.outputPath, data); err != nil {
			return errors.Wrap(err, "failed to generate %s", mapping.outputPath)
		}
	}

	return nil
}

// generateFileFromTemplate generates a file from a template
func generateFileFromTemplate(templateFile, outputPath string, data TemplateData) error {
	templateContent, err := TemplateFS.ReadFile("templates/" + templateFile)
	if err != nil {
		return errors.Wrap(err, "template file not found: %s", templateFile)
	}

	tmpl, err := template.New(templateFile).Parse(string(templateContent))
	if err != nil {
		return errors.Wrap(err, "failed to parse template")
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return errors.Wrap(err, "failed to create output directory")
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return errors.Wrap(err, "failed to create output file")
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return errors.Wrap(err, "failed to execute template")
	}

	return nil
}

// resolveFeatureDependencies resolves feature dependencies
func resolveFeatureDependencies(features map[string]bool) map[string]bool {
	dependencies := map[string][]string{
		"auth":          {"tokens"},
		"backoffice":    {"auth"},
		"events":        {"cache"},
		"notifications": {"events"},
	}

	for {
		added := false
		for feature, deps := range dependencies {
			if features[feature] {
				for _, dep := range deps {
					if !features[dep] {
						features[dep] = true
						added = true
					}
				}
			}
		}
		if !added {
			break
		}
	}

	return features
}

// getEnabledFeatures returns a list of enabled feature names
func getEnabledFeatures(features map[string]bool) []string {
	var enabled []string
	for feature, isEnabled := range features {
		if isEnabled {
			enabled = append(enabled, feature)
		}
	}
	return enabled
}

// printNextSteps prints the next steps after initialization
func printNextSteps(opts Options) {
	fmt.Printf("Next steps:\n\n")
	fmt.Printf("1. Navigate to the project directory:\n")
	fmt.Printf("   cd %s\n\n", opts.Output)
	fmt.Printf("2. Install dependencies:\n")
	fmt.Printf("   go mod tidy\n\n")
	fmt.Printf("3. Start infrastructure (Docker):\n")
	fmt.Printf("   docker-compose -f misc/dockerfiles/docker-compose.yaml up -d\n\n")
	fmt.Printf("4. Configure environment:\n")
	fmt.Printf("   cp .env.example .env\n")
	fmt.Printf("   # Edit .env with your configuration\n\n")
	fmt.Printf("5. Run the application:\n")
	fmt.Printf("   make run MS=authentication\n\n")
	fmt.Printf("6. Run tests:\n")
	fmt.Printf("   make test\n\n")
	fmt.Printf("For more information, see README.md\n")
}
