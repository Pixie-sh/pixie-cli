package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pixie-sh/errors-go"
	genshared "github.com/pixie-sh/pixie-cli/internal/cli/pixie/generate_cmd/shared"
	initshared "github.com/pixie-sh/pixie-cli/internal/cli/pixie/init_cmd/shared"
	"github.com/spf13/cobra"
)

// MicroserviceOptions holds all the options for microservice generation.
type MicroserviceOptions struct {
	Name        string
	Domain      string
	Features    string
	Template    string
	Output      string
	Port        int
	MetricsPort int
	ModuleName  string
	Force       bool
}

// MicroserviceCmd returns the cobra command for microservice generation.
func MicroserviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "microservice",
		Short: "Generate a new microservice",
		Long: `Generate a new microservice with the specified domain and features.

This command creates a complete microservice implementation following the established
architecture patterns, including entry point, domain layers, and configuration.

Available features:
  - database: PostgreSQL integration with GORM
  - metrics: Prometheus metrics and health checks
  - auth: JWT authentication and authorization (requires tokens)
  - cache: Redis caching integration  
  - tokens: JWT token management and sessions
  - events: Event publishing and handling
  - notifications: Push notification services
  - backoffice: Admin/management endpoints (requires auth)
  - validation: Request validation bundle
  - adapters: External service adapters
  - apis: External API integration

Examples:
  # Generate a basic microservice
  pixie generate microservice --name user_management --domain users

  # Generate with multiple features
  pixie generate microservice --name order_service --domain orders --features database,auth,cache

  # Generate with custom ports
  pixie generate microservice --name payment_service --domain payments --port 8081 --metrics-port 9091

  # Force overwrite existing files
  pixie generate microservice --name existing_service --domain existing --force
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var name, _ = cmd.Flags().GetString("name")
			var domain, _ = cmd.Flags().GetString("domain")
			var features, _ = cmd.Flags().GetString("features")
			var template, _ = cmd.Flags().GetString("template")
			var output, _ = cmd.Flags().GetString("output")
			var port, _ = cmd.Flags().GetInt("port")
			var metricsPort, _ = cmd.Flags().GetInt("metrics-port")
			var moduleName, _ = cmd.Flags().GetString("module-name")
			var force, _ = cmd.Flags().GetBool("force")

			opts := MicroserviceOptions{
				Name:        name,
				Domain:      domain,
				Features:    features,
				Template:    template,
				Output:      output,
				Port:        port,
				MetricsPort: metricsPort,
				ModuleName:  moduleName,
				Force:       force,
			}

			return generateMicroservice(opts)
		},
	}

	// Required flags
	cmd.Flags().String("name", "", "Microservice name in snake_case (required)")
	cmd.Flags().String("domain", "", "Domain name for business logic (required)")

	// Optional flags
	cmd.Flags().String("features", "database,metrics,auth", "Comma-separated list of features")
	cmd.Flags().String("template", "standard", "Template type (minimal, standard, full)")
	cmd.Flags().String("output", "", "Output directory (default: ./<ms_dir>/<ms_prefix>{name})")
	cmd.Flags().Int("port", 8080, "HTTP server port")
	cmd.Flags().Int("metrics-port", 9090, "Metrics server port")
	cmd.Flags().String("module-name", "", "Go module name (auto-detected if not provided)")
	cmd.Flags().Bool("force", false, "Force overwrite existing files")

	// Mark required flags
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("domain")

	return cmd
}

func generateMicroservice(opts MicroserviceOptions) error {
	// Validate inputs
	if err := validateMicroserviceOptions(opts); err != nil {
		return errors.Wrap(err, "validation failed")
	}

	// Load config
	cfg, err := genshared.LoadConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	// Auto-detect module name if not provided
	moduleName, err := genshared.ResolveModule(opts.ModuleName)
	if err != nil {
		return errors.Wrap(err, "failed to detect module name")
	}
	if cfg.ModuleName == "" {
		cfg.ModuleName = moduleName
	}

	// Parse features
	features := genshared.ParseFeatures(opts.Features, opts.Template)
	features = genshared.ResolveFeatureDependencies(features)

	// Create template data
	data := genshared.NewTemplateData()
	data.ServiceName = opts.Name
	data.ServiceNameCamel = initshared.ToCamelCase(opts.Name)
	data.DomainName = opts.Domain
	data.DomainNameCamel = initshared.ToCamelCase(opts.Domain)
	data.ModuleName = moduleName
	data.RepositoryName = opts.Domain
	data.RepositoryNameCamel = initshared.ToCamelCase(opts.Domain)
	data.EntityName = opts.Domain
	data.EntityNameCamel = initshared.ToCamelCase(opts.Domain)
	data.Features = features
	data.Port = opts.Port
	data.MetricsPort = opts.MetricsPort

	// Print generation summary
	fmt.Printf("Generating microservice: %s\n", data.ServiceNameCamel)
	fmt.Printf("   Domain: %s\n", data.DomainName)
	fmt.Printf("   Features: %s\n", strings.Join(genshared.EnabledFeatures(features), ", "))
	fmt.Printf("   Module: %s\n", data.ModuleName)
	fmt.Printf("   Port: %d (metrics: %d)\n\n", data.Port, data.MetricsPort)

	// Generate files
	if err := generateMicroserviceFiles(data, opts, cfg); err != nil {
		return errors.Wrap(err, "failed to generate files")
	}

	fmt.Printf("Successfully generated microservice: %s\n\n", data.ServiceNameCamel)
	printMicroserviceNextSteps(data, cfg)

	return nil
}

func validateMicroserviceOptions(opts MicroserviceOptions) error {
	if !genshared.IsValidSnakeCase(opts.Name) {
		return errors.New("service name must be in snake_case format (e.g., user_management)")
	}
	if !genshared.IsValidIdentifier(opts.Domain) {
		return errors.New("domain name must be a valid identifier (e.g., users)")
	}
	if opts.Port < 1 || opts.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if opts.MetricsPort < 1 || opts.MetricsPort > 65535 {
		return errors.New("metrics port must be between 1 and 65535")
	}
	if opts.Port == opts.MetricsPort {
		return errors.New("port and metrics port must be different")
	}

	validTemplates := []string{"minimal", "standard", "full"}
	isValid := false
	for _, t := range validTemplates {
		if opts.Template == t {
			isValid = true
			break
		}
	}
	if !isValid {
		return errors.New("template must be one of: %s", strings.Join(validTemplates, ", "))
	}

	return nil
}

func generateMicroserviceFiles(data genshared.TemplateData, opts MicroserviceOptions, cfg genshared.GeneratorConfig) error {
	msDir := filepath.Join(cfg.MicroserviceDir, cfg.MicroservicePrefix+data.ServiceName)

	getOutputPath := func(fileName string) string {
		if opts.Output != "" {
			return filepath.Join(opts.Output, fileName)
		}
		return filepath.Join(msDir, fileName)
	}

	templateMappings := []struct {
		templateFile string
		outputPath   string
		condition    func() bool
	}{
		{
			templateFile: "templates/cmd_application.go.tmpl",
			outputPath:   filepath.Join(cfg.CmdDir, cfg.MicroservicePrefix+data.ServiceName, "application.go"),
		},
		{
			templateFile: "templates/microservice.go.tmpl",
			outputPath:   getOutputPath("microservice.go"),
		},
		{
			templateFile: "templates/ms_registry.go.tmpl",
			outputPath:   getOutputPath("registry.go"),
		},
		{
			templateFile: "templates/http_controllers.go.tmpl",
			outputPath:   getOutputPath("http_controllers.go"),
		},
		{
			templateFile: "templates/http_bo_controllers.go.tmpl",
			outputPath:   getOutputPath("http_bo_controllers.go"),
			condition:    func() bool { return data.Features["backoffice"] },
		},
		{
			templateFile: "templates/business_layer.go.tmpl",
			outputPath:   filepath.Join(cfg.DomainDir, data.DomainName, data.DomainName+cfg.BusinessLayerSuffix, data.DomainName+cfg.BusinessLayerSuffix+".go"),
		},
		{
			templateFile: "templates/data_layer.go.tmpl",
			outputPath:   filepath.Join(cfg.DomainDir, data.DomainName, data.DomainName+"_data_layer", data.DomainName+"_data_layer.go"),
			condition:    func() bool { return data.Features["database"] },
		},
		{
			templateFile: "templates/services.go.tmpl",
			outputPath:   filepath.Join(cfg.DomainDir, data.DomainName, data.DomainName+"_services", data.DomainName+"_service.go"),
		},
		{
			templateFile: "templates/domain_registry.go.tmpl",
			outputPath:   filepath.Join(cfg.DomainDir, data.DomainName, "registry.go"),
		},
		{
			templateFile: "templates/entities.go.tmpl",
			outputPath:   filepath.Join(cfg.DomainDir, data.DomainName, data.DomainName+"_data_layer", data.DomainName+"_entities", data.DomainName+".go"),
			condition:    func() bool { return data.Features["database"] },
		},
		{
			templateFile: "templates/repositories.go.tmpl",
			outputPath:   filepath.Join(cfg.DomainDir, data.DomainName, data.DomainName+"_data_layer", data.DomainName+"_repositories", data.DomainName+"_repository.go"),
			condition:    func() bool { return data.Features["database"] },
		},
		{
			templateFile: "templates/entity_migration.go.tmpl",
			outputPath:   filepath.Join(cfg.DomainDir, data.DomainName, data.DomainName+"_data_layer", data.DomainName+"_migrations", data.MigrationTimestamp+"_create_"+data.DomainName+"_table.go"),
			condition:    func() bool { return data.Features["database"] },
		},
		{
			templateFile: "templates/domain_migrations.go.tmpl",
			outputPath:   filepath.Join(cfg.DomainDir, data.DomainName, data.DomainName+"_data_layer", data.DomainName+"_migrations", "migrations.go"),
			condition:    func() bool { return data.Features["database"] },
		},
		{
			templateFile: "templates/models.go.tmpl",
			outputPath:   filepath.Join(cfg.ModelsDir, data.DomainName, data.DomainName+"_models.go"),
		},
		{
			templateFile: "templates/config.json.tmpl",
			outputPath:   filepath.Join(cfg.ConfigsDir, cfg.MicroservicePrefix+data.ServiceName+".json"),
		},
	}

	for _, mapping := range templateMappings {
		if mapping.condition != nil && !mapping.condition() {
			fmt.Printf("   Skipping %s (feature not enabled)\n", mapping.templateFile)
			continue
		}

		if !opts.Force && initshared.FileExists(mapping.outputPath) {
			fmt.Printf("   WARNING: Skipping %s (file exists, use --force to overwrite)\n", mapping.outputPath)
			continue
		}

		fmt.Printf("   Generating %s\n", mapping.outputPath)

		content, err := initshared.RenderTemplate(Templates, mapping.templateFile, data)
		if err != nil {
			return errors.Wrap(err, "failed to render template %s", mapping.templateFile)
		}

		if err := initshared.WriteFile(mapping.outputPath, content, opts.Force); err != nil {
			return errors.Wrap(err, "failed to write %s", mapping.outputPath)
		}
	}

	return nil
}

func printMicroserviceNextSteps(data genshared.TemplateData, cfg genshared.GeneratorConfig) {
	fmt.Printf("Next steps:\n\n")

	fmt.Printf("1. Update DI tokens in infra/di/injection_tokens.go:\n")
	fmt.Printf("   RegistryToken%sDataLayer     = pdi.RegisterInjectionToken(\"%s_data_layer\")\n", data.DomainNameCamel, data.DomainName)
	fmt.Printf("   RegistryToken%sBusinessLayer = pdi.RegisterInjectionToken(\"%s_business_layer\")\n", data.DomainNameCamel, data.DomainName)
	fmt.Printf("   RegistryToken%sService       = pdi.RegisterInjectionToken(\"%s_service\")\n\n", data.DomainNameCamel, data.DomainName)

	fmt.Printf("2. Update dependencies:\n")
	fmt.Printf("   go mod tidy\n\n")

	if data.Features["database"] {
		fmt.Printf("3. Run database migrations (if enabled):\n")
		fmt.Printf("   export DATABASE_HOST=%s\n", data.DatabaseHost)
		fmt.Printf("   export DATABASE_PORT=%d\n", data.DatabasePort)
		fmt.Printf("   export DATABASE_NAME=%s\n", data.DatabaseName)
		fmt.Printf("   export DATABASE_USERNAME=%s\n", data.DatabaseUsername)
		fmt.Printf("   export DATABASE_PASSWORD=%s\n\n", data.DatabasePassword)
	}

	if data.Features["auth"] {
		fmt.Printf("4. Set security environment variables:\n")
		fmt.Printf("   export JWT_SECRET_KEY=your-secure-jwt-secret\n")
		fmt.Printf("   export ADMIN_API_KEY=your-admin-api-key\n\n")
	}

	if data.Features["cache"] {
		fmt.Printf("5. Configure Redis (if cache enabled):\n")
		fmt.Printf("   export REDIS_HOST=%s\n", data.RedisHost)
		fmt.Printf("   export REDIS_PORT=%d\n\n", data.RedisPort)
	}

	msPrefix := cfg.MicroservicePrefix
	fmt.Printf("6. Test the generated service:\n")
	fmt.Printf("   go build ./%s/%s%s/\n", cfg.CmdDir, msPrefix, data.ServiceName)
	fmt.Printf("   go test ./%s/%s%s/...\n\n", cfg.MicroserviceDir, msPrefix, data.ServiceName)

	fmt.Printf("7. Start the service:\n")
	fmt.Printf("   cd %s/%s%s && go run application.go --config=../../../%s/%s%s.json\n\n",
		cfg.CmdDir, msPrefix, data.ServiceName,
		cfg.ConfigsDir, msPrefix, data.ServiceName)
}
