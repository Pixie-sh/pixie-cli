package scaffold

import (
	"fmt"
	"path/filepath"

	"github.com/pixie-sh/errors-go"
	genshared "github.com/pixie-sh/pixie-cli/internal/cli/pixie/generate_cmd/shared"
	initshared "github.com/pixie-sh/pixie-cli/internal/cli/pixie/init_cmd/shared"
	"github.com/spf13/cobra"
)

// DomainOptions holds all the options for domain generation.
type DomainOptions struct {
	Domain     string
	Features   string
	ModuleName string
	Force      bool
}

// DomainCmd returns the cobra command for domain generation.
func DomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain",
		Short: "Generate a new domain",
		Long: `Generate a new domain with business layer, data layer, services, entities, repositories, and migrations.

This command creates a complete domain implementation without microservice-specific files
like HTTP controllers, command applications, or configurations.

Available features:
  - database: PostgreSQL integration with GORM (includes entities, repositories, migrations)
  - auth: JWT authentication fields in entities
  - apis: External API integration

Examples:
  # Generate a basic domain
  pixie generate domain --domain users

  # Generate with multiple features
  pixie generate domain --domain orders --features database,auth

  # Force overwrite existing files
  pixie generate domain --domain existing --force
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var domain, _ = cmd.Flags().GetString("domain")
			var features, _ = cmd.Flags().GetString("features")
			var moduleName, _ = cmd.Flags().GetString("module-name")
			var force, _ = cmd.Flags().GetBool("force")

			opts := DomainOptions{
				Domain:     domain,
				Features:   features,
				ModuleName: moduleName,
				Force:      force,
			}

			return generateDomain(opts)
		},
	}

	// Required flags
	cmd.Flags().String("domain", "", "Domain name for business logic (required)")

	// Optional flags
	cmd.Flags().String("features", "database", "Comma-separated list of features")
	cmd.Flags().String("module-name", "", "Go module name (auto-detected if not provided)")
	cmd.Flags().Bool("force", false, "Force overwrite existing files")

	// Mark required flags
	cmd.MarkFlagRequired("domain")

	return cmd
}

func generateDomain(opts DomainOptions) error {
	if !genshared.IsValidIdentifier(opts.Domain) {
		return errors.New("domain name must be a valid identifier (e.g., users)")
	}

	cfg, err := genshared.LoadConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	moduleName, err := genshared.ResolveModule(opts.ModuleName)
	if err != nil {
		return errors.Wrap(err, "failed to detect module name")
	}

	features := genshared.ParseFeatures(opts.Features, "minimal")
	features = genshared.ResolveFeatureDependencies(features)

	data := genshared.NewTemplateData()
	data.DomainName = opts.Domain
	data.DomainNameCamel = initshared.ToCamelCase(opts.Domain)
	data.ServiceName = opts.Domain
	data.ServiceNameCamel = initshared.ToCamelCase(opts.Domain)
	data.ModuleName = moduleName
	data.RepositoryName = opts.Domain
	data.RepositoryNameCamel = initshared.ToCamelCase(opts.Domain)
	data.EntityName = opts.Domain
	data.EntityNameCamel = initshared.ToCamelCase(opts.Domain)
	data.Features = features

	fmt.Printf("Generating domain: %s\n", data.DomainNameCamel)
	fmt.Printf("   Features: %s\n", genshared.FeaturesListString(features))
	fmt.Printf("   Module: %s\n\n", data.ModuleName)

	if err := generateDomainFiles(data, opts, cfg); err != nil {
		return errors.Wrap(err, "failed to generate domain files")
	}

	fmt.Printf("Successfully generated domain: %s\n\n", data.DomainNameCamel)
	printDomainNextSteps(data, cfg)

	return nil
}

func generateDomainFiles(data genshared.TemplateData, opts DomainOptions, cfg genshared.GeneratorConfig) error {
	templateMappings := []struct {
		templateFile string
		outputPath   string
		condition    func() bool
	}{
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

func printDomainNextSteps(data genshared.TemplateData, cfg genshared.GeneratorConfig) {
	fmt.Printf("Next steps:\n\n")

	fmt.Printf("1. Update DI tokens in infra/di/injection_tokens.go:\n")
	fmt.Printf("   RegistryToken%sDataLayer     = pdi.RegisterInjectionToken(\"%s_data_layer\")\n", data.DomainNameCamel, data.DomainName)
	fmt.Printf("   RegistryToken%sBusinessLayer = pdi.RegisterInjectionToken(\"%s_business_layer\")\n", data.DomainNameCamel, data.DomainName)
	fmt.Printf("   RegistryToken%sService       = pdi.RegisterInjectionToken(\"%s_service\")\n\n", data.DomainNameCamel, data.DomainName)

	fmt.Printf("2. Update dependencies:\n")
	fmt.Printf("   go mod tidy\n\n")

	if data.Features["database"] {
		fmt.Printf("3. Run database migrations (if database enabled):\n")
		fmt.Printf("   # Register migrations in your migration system\n")
		fmt.Printf("   # Add %s_migrations.Migrations to your migration registry\n\n", data.DomainName)
	}

	fmt.Printf("4. Test the generated domain:\n")
	fmt.Printf("   go build ./%s/%s/...\n", cfg.DomainDir, data.DomainName)
	fmt.Printf("   go test ./%s/%s/...\n\n", cfg.DomainDir, data.DomainName)

	fmt.Printf("To create a microservice using this domain:\n")
	fmt.Printf("   pixie generate microservice --name %s_service --domain %s\n\n", data.DomainName, data.DomainName)
}
