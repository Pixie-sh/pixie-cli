package bootstrap_cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/pixie-sh/errors-go"
	"github.com/spf13/cobra"

	"github.com/pixie-sh/pixie-cli/internal/cli/core/templates"
)

// MicroserviceConfig defines configuration for a microservice to generate
type MicroserviceConfig struct {
	Name        string   // e.g., "authentication"
	Port        int      // HTTP server port
	MetricsPort int      // Metrics server port
	Domains     []string // Domains this MS owns
	Features    []string // Features enabled for this MS
}

// BootstrapOptions holds all options for the bootstrap command
type BootstrapOptions struct {
	Name          string   // Project name
	Module        string   // Go module path
	Output        string   // Output directory
	Microservices []string // Microservices to generate
	Force         bool     // Overwrite existing files
	ProjectMS     string   // Custom name for the project microservice
}

// DomainInfo holds domain information for template generation
type DomainInfo struct {
	Name      string
	NameCamel string
}

// TemplateData holds the data passed to templates (matches generator_cmd.TemplateData)
type TemplateData struct {
	// Project-level information
	ProjectName string // Project name (for docker-compose, README, etc.)
	ProjectMS   string // Custom project microservice name

	// Basic information
	ServiceName         string // user_management (for microservices) or custom service name
	ServiceNameCamel    string // UserManagement (for microservices) or custom service name
	DomainName          string // users
	DomainNameCamel     string // Users
	ModuleName          string // github.com/pixie-sh/grupoegor-backend-system
	RepositoryName      string // For repositories: custom name or domain name
	RepositoryNameCamel string // CamelCase version of RepositoryName
	EntityName          string // For entities: custom name or domain name
	EntityNameCamel     string // CamelCase version of EntityName

	// All domains for injection_tokens.go template
	Domains []DomainInfo

	// Features
	Features map[string]bool // Feature flags

	// Configuration
	Port               int    // HTTP server port
	MetricsPort        int    // Metrics server port
	Timestamp          string // Generation timestamp
	MigrationTimestamp string // Migration timestamp for DB migrations

	// Database configuration (with defaults)
	DatabaseHost                  string
	DatabasePort                  int
	DatabaseName                  string
	DatabaseUsername              string
	DatabasePassword              string
	DatabaseSSLMode               string
	DatabaseMaxOpenConnections    int
	DatabaseMaxIdleConnections    int
	DatabaseConnectionMaxLifetime string

	// Redis configuration (with defaults)
	RedisHost     string
	RedisPort     int
	RedisPassword string
	RedisDatabase int

	// Security configuration (with defaults)
	JWTSecretKey string
	AdminAPIKey  string
}

// BootstrapCmd returns the bootstrap command
func BootstrapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap a complete backend project",
		Long: `Bootstrap a complete backend project with multiple microservices.

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
  - E2E test infrastructure

Examples:
  # Bootstrap a new project
  cli_core bootstrap --name my-backend --module github.com/company/my-backend

  # Bootstrap with custom output directory
  cli_core bootstrap --name my-backend --module github.com/company/my-backend --output /path/to/output

  # Bootstrap with custom project microservice name
  cli_core bootstrap --name my-backend --module github.com/company/my-backend --project-ms orders

  # Bootstrap specific microservices only
  cli_core bootstrap --name my-backend --module github.com/company/my-backend --microservices authentication,notifications

  # Force overwrite existing files
  cli_core bootstrap --name my-backend --module github.com/company/my-backend --force
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var name, _ = cmd.Flags().GetString("name")
			var module, _ = cmd.Flags().GetString("module")
			var output, _ = cmd.Flags().GetString("output")
			var microservices, _ = cmd.Flags().GetStringSlice("microservices")
			var force, _ = cmd.Flags().GetBool("force")
			var projectMS, _ = cmd.Flags().GetString("project-ms")

			opts := BootstrapOptions{
				Name:          name,
				Module:        module,
				Output:        output,
				Microservices: microservices,
				Force:         force,
				ProjectMS:     projectMS,
			}

			return runBootstrap(opts)
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

// runBootstrap executes the bootstrap command
func runBootstrap(opts BootstrapOptions) error {
	// Validate inputs
	if err := validateBootstrapOptions(opts); err != nil {
		return errors.Wrap(err, "validation failed")
	}

	// Define microservice configurations
	msConfigs := getDefaultMicroserviceConfigs(opts.ProjectMS)

	// Filter to requested microservices
	selectedConfigs := filterMicroservices(msConfigs, opts.Microservices)

	fmt.Printf("Bootstrapping project: %s\n", opts.Name)
	fmt.Printf("   Module: %s\n", opts.Module)
	fmt.Printf("   Output: %s\n", opts.Output)
	fmt.Printf("   Microservices: %s\n\n", strings.Join(opts.Microservices, ", "))

	// Collect all domains for injection_tokens.go
	allDomains := collectAllDomains(selectedConfigs)

	// Generate project-level infrastructure files (once)
	fmt.Println("Generating project infrastructure...")
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

	fmt.Printf("\nBootstrap complete!\n\n")

	// Print next steps
	printBootstrapNextSteps(opts, selectedConfigs)

	return nil
}

// collectAllDomains collects all unique domains from microservice configs
func collectAllDomains(configs []MicroserviceConfig) []DomainInfo {
	seen := make(map[string]bool)
	var domains []DomainInfo

	for _, config := range configs {
		for _, domain := range config.Domains {
			if !seen[domain] {
				seen[domain] = true
				domains = append(domains, DomainInfo{
					Name:      domain,
					NameCamel: toCamelCase(domain),
				})
			}
		}
	}

	return domains
}

// generateProjectInfrastructure generates project-level files
func generateProjectInfrastructure(opts BootstrapOptions, domains []DomainInfo, configs []MicroserviceConfig) error {
	// Determine if auth and notification features are needed
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
		// Check if this is the notifications microservice
		if config.Name == "notifications" {
			hasNotifications = true
		}
	}

	// Create base template data for project-level files
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

	// Project-level template mappings
	projectTemplates := []struct {
		templateFile string
		outputPath   string
		condition    func() bool
	}{
		// Root project files
		{
			templateFile: "go_mod.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "go.mod"),
		},
		{
			templateFile: "makefile.tmpl",
			outputPath:   filepath.Join(opts.Output, "Makefile"),
		},
		{
			templateFile: "common_mk.tmpl",
			outputPath:   filepath.Join(opts.Output, "common.mk"),
		},
		{
			templateFile: "dev_mk.tmpl",
			outputPath:   filepath.Join(opts.Output, "dev.mk"),
		},
		{
			templateFile: "dockers_mk.tmpl",
			outputPath:   filepath.Join(opts.Output, "dockers.mk"),
		},
		{
			templateFile: "gitignore.tmpl",
			outputPath:   filepath.Join(opts.Output, ".gitignore"),
		},
		{
			templateFile: "readme.tmpl",
			outputPath:   filepath.Join(opts.Output, "README.md"),
		},
		{
			templateFile: "env_example.tmpl",
			outputPath:   filepath.Join(opts.Output, ".env.example"),
		},

		// Docker configuration
		{
			templateFile: "docker_compose.yaml.tmpl",
			outputPath:   filepath.Join(opts.Output, "misc/dockerfiles/docker-compose.yaml"),
		},

		// Infra - DI tokens
		{
			templateFile: "injection_tokens.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "infra/di/injection_tokens.go"),
		},

		// Infra - Version
		{
			templateFile: "version.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "infra/version/version.go"),
		},

		// Bundles
		{
			templateFile: "bundles_database.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "bundles/database_bundle.go"),
		},
		{
			templateFile: "bundles_http_server.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "bundles/http_server_bundle.go"),
		},
		{
			templateFile: "bundles_registry.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "bundles/registry.go"),
		},
		{
			templateFile: "bundles_token_services.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "bundles/token_bundle.go"),
			condition:    func() bool { return hasAuth },
		},
		{
			templateFile: "bundles_authorization_gates.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "bundles/authorization_gates_bundle.go"),
			condition:    func() bool { return hasAuth },
		},

		// Gates
		{
			templateFile: "gates_is_authenticated.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "infra/gates/is_authenticated.go"),
			condition:    func() bool { return hasAuth },
		},
		{
			templateFile: "gates_authorization_token.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "infra/gates/authorization_token.go"),
			condition:    func() bool { return hasAuth },
		},

		// Session Manager (split into multiple files)
		{
			templateFile: "session_manager.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "infra/session_manager/session_manager.go"),
			condition:    func() bool { return hasAuth },
		},
		{
			templateFile: "session_jwt_service.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "infra/session_manager/jwt_service.go"),
			condition:    func() bool { return hasAuth },
		},
		{
			templateFile: "session_jwt_functions.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "infra/session_manager/jwt_functions.go"),
			condition:    func() bool { return hasAuth },
		},
		{
			templateFile: "session_service.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "infra/session_manager/session_service.go"),
			condition:    func() bool { return hasAuth },
		},
		{
			templateFile: "session_functions.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "infra/session_manager/session_functions.go"),
			condition:    func() bool { return hasAuth },
		},
		{
			templateFile: "session_singleton.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "infra/session_manager/singleton.go"),
			condition:    func() bool { return hasAuth },
		},
		{
			templateFile: "session_manager_models.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "pkg/models/session_manager_models/session_manager_models.go"),
			condition:    func() bool { return hasAuth },
		},

		// Pkg context (base context package)
		{
			templateFile: "pkg_context.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "pkg/context/context.go"),
			condition:    func() bool { return hasAuth },
		},

		// Pkg context http (http-specific context helpers)
		{
			templateFile: "pkg_context_http.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "pkg/context/http/context.go"),
			condition:    func() bool { return hasAuth },
		},

		// Message packs (always needed - referenced by microservice.go.tmpl)
		{
			templateFile: "message_packs.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "infra/message_packs/message_packs.go"),
		},

		// APIs registry (always needed - referenced by ms_registry.go.tmpl)
		{
			templateFile: "apis_registry.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "infra/apis/registry.go"),
		},

		// E2E utils (always needed - referenced by e2e_bootstrap_test.go.tmpl)
		{
			templateFile: "e2e_utils.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "internal/e2e_tests/e2e_utils/e2e_utils.go"),
		},

		// UID generator (always needed - referenced by session_manager)
		{
			templateFile: "uidgen.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "infra/uidgen/uidgen.go"),
			condition:    func() bool { return hasAuth },
		},
		{
			templateFile: "uidgen_checks.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "infra/uidgen/checks.go"),
			condition:    func() bool { return hasAuth },
		},

		// Event definitions (always needed - referenced by business layers)
		{
			templateFile: "event.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "infra/event/event.go"),
			condition:    func() bool { return hasAuth },
		},

		// Authentication adapters (needed when auth is enabled)
		{
			templateFile: "auth_adapters.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "internal/adapters/authentication_adapters/adapters.go"),
			condition:    func() bool { return hasAuth },
		},

		// Rate limiter bundle (needed when auth is enabled)
		{
			templateFile: "rate_limiter_bundle.go.tmpl",
			outputPath:   filepath.Join(opts.Output, "bundles/rate_limiter_bundle.go"),
			condition:    func() bool { return hasAuth },
		},
	}

	// Generate each project-level file
	for _, mapping := range projectTemplates {
		// Check condition if present
		if mapping.condition != nil && !mapping.condition() {
			continue
		}

		// Check if file already exists and force flag is not set
		if !opts.Force && fileExists(mapping.outputPath) {
			fmt.Printf("   WARNING: Skipping %s (file exists, use --force to overwrite)\n", mapping.outputPath)
			continue
		}

		fmt.Printf("   Generating %s\n", mapping.outputPath)

		if err := generateFileFromTemplate(mapping.templateFile, mapping.outputPath, data); err != nil {
			return errors.Wrap(err, "failed to generate %s", mapping.outputPath)
		}
	}

	return nil
}

// validateBootstrapOptions validates the bootstrap options
func validateBootstrapOptions(opts BootstrapOptions) error {
	if opts.Name == "" {
		return errors.New("project name is required")
	}

	if opts.Module == "" {
		return errors.New("module path is required")
	}

	// Validate module path format
	if !strings.Contains(opts.Module, "/") {
		return errors.New("module path must be a valid Go module path (e.g., github.com/company/project)")
	}

	// Validate microservices
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

// filterMicroservices filters the microservice configs to only include requested ones
func filterMicroservices(configs []MicroserviceConfig, requested []string) []MicroserviceConfig {
	requestedMap := make(map[string]bool)
	for _, ms := range requested {
		requestedMap[ms] = true
	}

	var filtered []MicroserviceConfig
	for _, config := range configs {
		// Check if this microservice or "project" (which maps to custom name) is requested
		if requestedMap[config.Name] || (config.Name != "authentication" && config.Name != "notifications" && requestedMap["project"]) {
			filtered = append(filtered, config)
		}
	}

	return filtered
}

// generateMicroserviceFromConfig generates a microservice from its configuration
func generateMicroserviceFromConfig(msConfig MicroserviceConfig, opts BootstrapOptions) error {
	// Parse features into map
	features := make(map[string]bool)
	for _, f := range msConfig.Features {
		features[f] = true
	}

	// Resolve feature dependencies
	features = resolveFeatureDependencies(features)

	// Use first domain as primary
	domainName := msConfig.Name
	if len(msConfig.Domains) > 0 {
		domainName = msConfig.Domains[0]
	}

	// Create template data
	data := TemplateData{
		ProjectName:         opts.Name,
		ProjectMS:           opts.ProjectMS,
		ServiceName:         msConfig.Name,
		ServiceNameCamel:    toCamelCase(msConfig.Name),
		DomainName:          domainName,
		DomainNameCamel:     toCamelCase(domainName),
		ModuleName:          opts.Module,
		RepositoryName:      domainName,
		RepositoryNameCamel: toCamelCase(domainName),
		EntityName:          domainName,
		EntityNameCamel:     toCamelCase(domainName),
		Features:            features,
		Port:                msConfig.Port,
		MetricsPort:         msConfig.MetricsPort,
		Timestamp:           time.Now().Format(time.RFC3339),
		MigrationTimestamp:  fmt.Sprintf("%d", time.Now().Unix()),

		// Database defaults
		DatabaseHost:                  "localhost",
		DatabasePort:                  5432,
		DatabaseName:                  "backend_system",
		DatabaseUsername:              "postgres",
		DatabasePassword:              "password",
		DatabaseSSLMode:               "disable",
		DatabaseMaxOpenConnections:    100,
		DatabaseMaxIdleConnections:    10,
		DatabaseConnectionMaxLifetime: "1h",

		// Redis defaults
		RedisHost:     "localhost",
		RedisPort:     6379,
		RedisPassword: "",
		RedisDatabase: 0,

		// Security defaults
		JWTSecretKey: "your-secret-key",
		AdminAPIKey:  "admin-api-key",
	}

	fmt.Printf("   Domain: %s\n", data.DomainName)
	fmt.Printf("   Features: %s\n", strings.Join(getEnabledFeatures(features), ", "))
	fmt.Printf("   Port: %d (metrics: %d)\n", data.Port, data.MetricsPort)

	// Generate files for this microservice
	if err := generateMicroserviceFiles(data, opts); err != nil {
		return err
	}

	return nil
}

// generateMicroserviceFiles generates all files for a microservice
func generateMicroserviceFiles(data TemplateData, opts BootstrapOptions) error {
	// Route to specialized generators for known microservices
	switch data.ServiceName {
	case "authentication":
		return generateAuthenticationMicroservice(data, opts)
	case "notifications":
		return generateNotificationsMicroservice(data, opts)
	default:
		return generateGenericMicroservice(data, opts)
	}
}

// generateAuthenticationMicroservice generates all files for the authentication microservice
func generateAuthenticationMicroservice(data TemplateData, opts BootstrapOptions) error {
	basePath := opts.Output
	domainPath := filepath.Join(basePath, "internal/domains/authentication")
	msPath := filepath.Join(basePath, "internal/ms/ms_authentication")
	modelsPath := filepath.Join(basePath, "pkg/models/auth")

	// Define all auth-specific template mappings
	templateMappings := []struct {
		templateFile string
		outputPath   string
	}{
		// Entry point
		{
			templateFile: "cmd_application.go.tmpl",
			outputPath:   filepath.Join(basePath, "cmd/ms/ms_authentication/application.go"),
		},

		// Microservice layer
		{
			templateFile: "auth_microservice.go.tmpl",
			outputPath:   filepath.Join(msPath, "microservice.go"),
		},
		{
			templateFile: "ms_registry.go.tmpl",
			outputPath:   filepath.Join(msPath, "registry.go"),
		},
		{
			templateFile: "auth_http_controllers.go.tmpl",
			outputPath:   filepath.Join(msPath, "http_controllers.go"),
		},
		{
			templateFile: "auth_http_bo_controllers.go.tmpl",
			outputPath:   filepath.Join(msPath, "http_bo_controllers.go"),
		},

		// Domain - Entities
		{
			templateFile: "auth_entity_user.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_data_layer/auth_entities/user.go"),
		},
		{
			templateFile: "auth_entity_otp.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_data_layer/auth_entities/otp.go"),
		},
		{
			templateFile: "auth_entity_password_resets.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_data_layer/auth_entities/password_resets.go"),
		},

		// Domain - Migrations
		{
			templateFile: "auth_migration_user.go.tmpl",
			outputPath:   filepath.Join(domainPath, fmt.Sprintf("authentication_data_layer/auth_migrations/%s1_create_user_table.go", data.MigrationTimestamp)),
		},
		{
			templateFile: "auth_migration_password_resets.go.tmpl",
			outputPath:   filepath.Join(domainPath, fmt.Sprintf("authentication_data_layer/auth_migrations/%s2_create_password_resets.go", data.MigrationTimestamp)),
		},
		{
			templateFile: "auth_migration_otp.go.tmpl",
			outputPath:   filepath.Join(domainPath, fmt.Sprintf("authentication_data_layer/auth_migrations/%s3_create_otp_table.go", data.MigrationTimestamp)),
		},
		{
			templateFile: "auth_migrations.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_data_layer/auth_migrations/migrations.go"),
		},

		// Domain - Repositories
		{
			templateFile: "auth_repo_user.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_data_layer/auth_repositories/user_repository.go"),
		},
		{
			templateFile: "auth_repo_password.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_data_layer/auth_repositories/password_repository.go"),
		},
		{
			templateFile: "auth_repo_otp.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_data_layer/auth_repositories/otp_repository.go"),
		},

		// Domain - Data Layers
		{
			templateFile: "auth_data_layer.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_data_layer/authentication_data_layer.go"),
		},
		{
			templateFile: "auth_user_data_layer.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_data_layer/user_data_layer.go"),
		},
		{
			templateFile: "auth_password_data_layer.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_data_layer/password_data_layer.go"),
		},
		{
			templateFile: "auth_otp_data_layer.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_data_layer/otp_data_layer.go"),
		},
		{
			templateFile: "auth_registration_data_layer.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_data_layer/registration_data_layer.go"),
		},

		// Domain - Business Layers
		{
			templateFile: "auth_business_layer.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_business_layer/authentication_business_layer.go"),
		},
		{
			templateFile: "auth_password_business_layer.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_business_layer/password_business_layer.go"),
		},
		{
			templateFile: "auth_registration_business_layer.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_business_layer/registration_business_layer.go"),
		},
		{
			templateFile: "auth_user_profile_business_layer.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_business_layer/user_profile_business_layer.go"),
		},

		// Domain - Services
		{
			templateFile: "auth_login_service.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_services/login_service.go"),
		},
		{
			templateFile: "auth_password_service.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_services/password_service.go"),
		},
		{
			templateFile: "auth_otp_service.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_services/user_otp_service.go"),
		},
		{
			templateFile: "auth_users_service.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_services/users_service.go"),
		},
		{
			templateFile: "auth_state_machine.go.tmpl",
			outputPath:   filepath.Join(domainPath, "authentication_services/user_state_machine.go"),
		},

		// Domain - Registry
		{
			templateFile: "auth_domain_registry.go.tmpl",
			outputPath:   filepath.Join(domainPath, "registry.go"),
		},

		// Models
		{
			templateFile: "auth_models.go.tmpl",
			outputPath:   filepath.Join(modelsPath, "auth_models.go"),
		},

		// Config
		{
			templateFile: "config.json.tmpl",
			outputPath:   filepath.Join(basePath, "misc/configs/ms_authentication.json"),
		},

		// E2E Tests
		{
			templateFile: "e2e_configuration.go.tmpl",
			outputPath:   filepath.Join(basePath, "internal/e2e_tests/e2e_ms_authentication_tests/configuration.go"),
		},
		{
			templateFile: "e2e_bootstrap_test.go.tmpl",
			outputPath:   filepath.Join(basePath, "internal/e2e_tests/e2e_ms_authentication_tests/ms_authentication_bootstrap_test.go"),
		},
	}

	// Generate each file
	for _, mapping := range templateMappings {
		if !opts.Force && fileExists(mapping.outputPath) {
			fmt.Printf("   WARNING: Skipping %s (file exists, use --force to overwrite)\n", mapping.outputPath)
			continue
		}

		fmt.Printf("   Generating %s\n", mapping.outputPath)

		if err := generateFileFromTemplate(mapping.templateFile, mapping.outputPath, data); err != nil {
			return errors.Wrap(err, "failed to generate %s", mapping.outputPath)
		}
	}

	return nil
}

// generateNotificationsMicroservice generates all files for the notifications microservice
func generateNotificationsMicroservice(data TemplateData, opts BootstrapOptions) error {
	basePath := opts.Output
	domainPath := filepath.Join(basePath, "internal/domains/notifications")
	msPath := filepath.Join(basePath, "internal/ms/ms_notifications")
	modelsPath := filepath.Join(basePath, "pkg/models/notifications")
	adaptersPath := filepath.Join(basePath, "internal/adapters/notification_adapters")

	// Define all notification-specific template mappings
	templateMappings := []struct {
		templateFile string
		outputPath   string
	}{
		// Entry point
		{
			templateFile: "cmd_application.go.tmpl",
			outputPath:   filepath.Join(basePath, "cmd/ms/ms_notifications/application.go"),
		},

		// Microservice layer
		{
			templateFile: "microservice.go.tmpl",
			outputPath:   filepath.Join(msPath, "microservice.go"),
		},
		{
			templateFile: "ms_registry.go.tmpl",
			outputPath:   filepath.Join(msPath, "registry.go"),
		},
		{
			templateFile: "notif_http_controllers.go.tmpl",
			outputPath:   filepath.Join(msPath, "http_controllers.go"),
		},

		// Domain - Entities
		{
			templateFile: "notif_entity_template.go.tmpl",
			outputPath:   filepath.Join(domainPath, "notifications_data_layer/notifications_entities/template.go"),
		},
		{
			templateFile: "notif_entity_action.go.tmpl",
			outputPath:   filepath.Join(domainPath, "notifications_data_layer/notifications_entities/action.go"),
		},
		{
			templateFile: "notif_entity_activity_log.go.tmpl",
			outputPath:   filepath.Join(domainPath, "notifications_data_layer/notifications_entities/activity_log.go"),
		},
		{
			templateFile: "notif_entity_firebase_token.go.tmpl",
			outputPath:   filepath.Join(domainPath, "notifications_data_layer/notifications_entities/firebase_token.go"),
		},

		// Domain - Repositories
		{
			templateFile: "notif_repo_templates.go.tmpl",
			outputPath:   filepath.Join(domainPath, "notifications_data_layer/notifications_repositories/templates_repository.go"),
		},
		{
			templateFile: "notif_repo_actions.go.tmpl",
			outputPath:   filepath.Join(domainPath, "notifications_data_layer/notifications_repositories/actions_repository.go"),
		},
		{
			templateFile: "notif_repo_activity_logs.go.tmpl",
			outputPath:   filepath.Join(domainPath, "notifications_data_layer/notifications_repositories/activity_logs_repository.go"),
		},
		{
			templateFile: "notif_repo_firebase_tokens.go.tmpl",
			outputPath:   filepath.Join(domainPath, "notifications_data_layer/notifications_repositories/firebase_tokens_repository.go"),
		},

		// Domain - Data Layer
		{
			templateFile: "notif_data_layer.go.tmpl",
			outputPath:   filepath.Join(domainPath, "notifications_data_layer/notifications_data_layer.go"),
		},

		// Domain - Services
		{
			templateFile: "notif_email_service.go.tmpl",
			outputPath:   filepath.Join(domainPath, "notifications_services/email_service.go"),
		},
		{
			templateFile: "notif_firebase_service.go.tmpl",
			outputPath:   filepath.Join(domainPath, "notifications_services/firebase_service.go"),
		},
		{
			templateFile: "notif_fcm_token_service.go.tmpl",
			outputPath:   filepath.Join(domainPath, "notifications_services/fcm_token_service.go"),
		},
		{
			templateFile: "notif_sms_service.go.tmpl",
			outputPath:   filepath.Join(domainPath, "notifications_services/sms_service.go"),
		},
		{
			templateFile: "notif_helpers_service.go.tmpl",
			outputPath:   filepath.Join(domainPath, "notifications_services/helpers_service.go"),
		},

		// Domain - Business Layer
		{
			templateFile: "notif_business_layer.go.tmpl",
			outputPath:   filepath.Join(domainPath, "notifications_business_layer/notifications_business_layer.go"),
		},

		// Domain - Registry
		{
			templateFile: "notif_registry.go.tmpl",
			outputPath:   filepath.Join(domainPath, "registry.go"),
		},

		// Adapters
		{
			templateFile: "notif_adapters.go.tmpl",
			outputPath:   filepath.Join(adaptersPath, "adapters.go"),
		},
		{
			templateFile: "notif_adapter_actions.go.tmpl",
			outputPath:   filepath.Join(adaptersPath, "actions_adapter.go"),
		},
		{
			templateFile: "notif_adapter_activity_logs.go.tmpl",
			outputPath:   filepath.Join(adaptersPath, "activity_logs_adapter.go"),
		},
		{
			templateFile: "notif_adapter_templates.go.tmpl",
			outputPath:   filepath.Join(adaptersPath, "templates_adapter.go"),
		},

		// Models
		{
			templateFile: "notif_models.go.tmpl",
			outputPath:   filepath.Join(modelsPath, "notifications_models.go"),
		},

		// Config
		{
			templateFile: "config.json.tmpl",
			outputPath:   filepath.Join(basePath, "misc/configs/ms_notifications.json"),
		},

		// E2E Tests
		{
			templateFile: "e2e_configuration.go.tmpl",
			outputPath:   filepath.Join(basePath, "internal/e2e_tests/e2e_ms_notifications_tests/configuration.go"),
		},
		{
			templateFile: "e2e_bootstrap_test.go.tmpl",
			outputPath:   filepath.Join(basePath, "internal/e2e_tests/e2e_ms_notifications_tests/ms_notifications_bootstrap_test.go"),
		},
	}

	// Generate each file
	for _, mapping := range templateMappings {
		if !opts.Force && fileExists(mapping.outputPath) {
			fmt.Printf("   WARNING: Skipping %s (file exists, use --force to overwrite)\n", mapping.outputPath)
			continue
		}

		fmt.Printf("   Generating %s\n", mapping.outputPath)

		if err := generateFileFromTemplate(mapping.templateFile, mapping.outputPath, data); err != nil {
			return errors.Wrap(err, "failed to generate %s", mapping.outputPath)
		}
	}

	return nil
}

// generateGenericMicroservice generates files for a generic microservice (non-auth, non-notifications)
func generateGenericMicroservice(data TemplateData, opts BootstrapOptions) error {
	// Define template mappings
	templateMappings := []struct {
		templateFile string
		outputPath   string
		condition    func() bool
	}{
		{
			templateFile: "cmd_application.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("cmd/ms/ms_%s/application.go", data.ServiceName)),
		},
		{
			templateFile: "microservice.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("internal/ms/ms_%s/microservice.go", data.ServiceName)),
		},
		{
			templateFile: "ms_registry.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("internal/ms/ms_%s/registry.go", data.ServiceName)),
		},
		{
			templateFile: "http_controllers.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("internal/ms/ms_%s/http_controllers.go", data.ServiceName)),
		},
		{
			templateFile: "http_bo_controllers.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("internal/ms/ms_%s/http_bo_controllers.go", data.ServiceName)),
			condition:    func() bool { return data.Features["backoffice"] },
		},
		{
			templateFile: "business_layer.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/%s_business_layer/%s_business_layer.go", data.DomainName, data.DomainName, data.DomainName)),
		},
		{
			templateFile: "data_layer.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/%s_data_layer/%s_data_layer.go", data.DomainName, data.DomainName, data.DomainName)),
			condition:    func() bool { return data.Features["database"] },
		},
		{
			templateFile: "services.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/%s_services/%s_service.go", data.DomainName, data.DomainName, data.DomainName)),
		},
		{
			templateFile: "domain_registry.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/registry.go", data.DomainName)),
		},
		{
			templateFile: "entities.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/%s_data_layer/%s_entities/%s.go", data.DomainName, data.DomainName, data.DomainName, data.DomainName)),
			condition:    func() bool { return data.Features["database"] },
		},
		{
			templateFile: "repositories.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/%s_data_layer/%s_repositories/%s_repository.go", data.DomainName, data.DomainName, data.DomainName, data.DomainName)),
			condition:    func() bool { return data.Features["database"] },
		},
		{
			templateFile: "entity_migration.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/%s_data_layer/%s_migrations/%s_create_%s_table.go", data.DomainName, data.DomainName, data.DomainName, data.MigrationTimestamp, data.DomainName)),
			condition:    func() bool { return data.Features["database"] },
		},
		{
			templateFile: "domain_migrations.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("internal/domains/%s/%s_data_layer/%s_migrations/migrations.go", data.DomainName, data.DomainName, data.DomainName)),
			condition:    func() bool { return data.Features["database"] },
		},
		{
			templateFile: "models.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("pkg/models/%s/%s_models.go", data.DomainName, data.DomainName)),
		},
		{
			templateFile: "config.json.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("misc/configs/ms_%s.json", data.ServiceName)),
		},
		{
			templateFile: "e2e_configuration.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("internal/e2e_tests/e2e_ms_%s_tests/configuration.go", data.ServiceName)),
			condition:    func() bool { return data.Features["e2e"] },
		},
		{
			templateFile: "e2e_bootstrap_test.go.tmpl",
			outputPath:   filepath.Join(opts.Output, fmt.Sprintf("internal/e2e_tests/e2e_ms_%s_tests/ms_%s_bootstrap_test.go", data.ServiceName, data.ServiceName)),
			condition:    func() bool { return data.Features["e2e"] },
		},
	}

	// Generate each file
	for _, mapping := range templateMappings {
		// Check condition if present
		if mapping.condition != nil && !mapping.condition() {
			continue
		}

		// Check if file already exists and force flag is not set
		if !opts.Force && fileExists(mapping.outputPath) {
			fmt.Printf("   WARNING: Skipping %s (file exists, use --force to overwrite)\n", mapping.outputPath)
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
	// Read template from embedded filesystem
	templateContent, err := templates.TemplateFS.ReadFile(templateFile)
	if err != nil {
		return errors.Wrap(err, "template file not found: %s", templateFile)
	}

	// Parse template
	tmpl, err := template.New(templateFile).Parse(string(templateContent))
	if err != nil {
		return errors.Wrap(err, "failed to parse template")
	}

	// Create output directory
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return errors.Wrap(err, "failed to create output directory")
	}

	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return errors.Wrap(err, "failed to create output file")
	}
	defer file.Close()

	// Execute template
	if err := tmpl.Execute(file, data); err != nil {
		return errors.Wrap(err, "failed to execute template")
	}

	return nil
}

// resolveFeatureDependencies resolves feature dependencies
func resolveFeatureDependencies(features map[string]bool) map[string]bool {
	// Feature dependency rules
	dependencies := map[string][]string{
		"auth":          {"tokens"},
		"backoffice":    {"auth"},
		"events":        {"cache"},
		"notifications": {"events"},
	}

	// Keep resolving until no more dependencies are added
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

// toCamelCase converts snake_case to CamelCase
func toCamelCase(s string) string {
	words := strings.Split(s, "_")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, "")
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
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

// printBootstrapNextSteps prints the next steps after bootstrap
func printBootstrapNextSteps(opts BootstrapOptions, configs []MicroserviceConfig) {
	fmt.Printf("Next steps:\n\n")

	fmt.Printf("1. Navigate to the output directory:\n")
	fmt.Printf("   cd %s\n\n", opts.Output)

	fmt.Printf("2. Install dependencies:\n")
	fmt.Printf("   go mod tidy\n\n")

	fmt.Printf("3. Start infrastructure (Docker):\n")
	fmt.Printf("   docker-compose -f misc/dockerfiles/docker-compose.yaml up -d\n\n")

	fmt.Printf("4. Configure environment:\n")
	fmt.Printf("   cp .env.example .env\n")
	fmt.Printf("   # Edit .env with your configuration\n\n")

	fmt.Printf("5. Build and run microservices:\n")
	for _, config := range configs {
		fmt.Printf("   # %s (port %d)\n", config.Name, config.Port)
		fmt.Printf("   make run MS=%s\n\n", config.Name)
	}

	fmt.Printf("6. Run tests:\n")
	fmt.Printf("   make test\n\n")

	fmt.Printf("For more information, see README.md\n")
}
