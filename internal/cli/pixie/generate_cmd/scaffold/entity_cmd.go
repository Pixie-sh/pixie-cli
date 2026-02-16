package scaffold

import (
	"fmt"
	"path/filepath"

	"github.com/pixie-sh/errors-go"
	genshared "github.com/pixie-sh/pixie-cli/internal/cli/pixie/generate_cmd/shared"
	initshared "github.com/pixie-sh/pixie-cli/internal/cli/pixie/init_cmd/shared"
	"github.com/spf13/cobra"
)

// EntityOptions holds all the options for entity generation.
type EntityOptions struct {
	Domain     string
	EntityName string
	Features   string
	ModuleName string
	Force      bool
}

// EntityCmd returns the cobra command for entity generation.
func EntityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entity",
		Short: "Generate a new entity and migration within an existing domain",
		Long: `Generate a new entity and migration within an existing domain.

This command creates:
- A new entity file in the domain's entities directory
- A corresponding migration file in the domain's migrations directory

Available features:
  - auth: Adds UserID field for user-scoped entities

Examples:
  # Generate a product entity in the catalog domain
  pixie generate entity --domain catalog --name product

  # Generate a user-scoped comment entity in the posts domain
  pixie generate entity --domain posts --name comment --features auth

  # Force overwrite existing entity and migration
  pixie generate entity --domain catalog --name product --force
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var domain, _ = cmd.Flags().GetString("domain")
			var entityName, _ = cmd.Flags().GetString("name")
			var features, _ = cmd.Flags().GetString("features")
			var moduleName, _ = cmd.Flags().GetString("module-name")
			var force, _ = cmd.Flags().GetBool("force")

			opts := EntityOptions{
				Domain:     domain,
				EntityName: entityName,
				Features:   features,
				ModuleName: moduleName,
				Force:      force,
			}

			return generateEntity(opts)
		},
	}

	// Required flags
	cmd.Flags().String("domain", "", "Existing domain name (required)")
	cmd.Flags().String("name", "", "Entity name (required)")

	// Optional flags
	cmd.Flags().String("features", "", "Comma-separated list of features (auth)")
	cmd.Flags().String("module-name", "", "Go module name (auto-detected if not provided)")
	cmd.Flags().Bool("force", false, "Force overwrite existing files")

	// Mark required flags
	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("name")

	return cmd
}

func generateEntity(opts EntityOptions) error {
	if !genshared.IsValidIdentifier(opts.Domain) {
		return errors.New("domain name must be a valid identifier (e.g., catalog)")
	}
	if !genshared.IsValidIdentifier(opts.EntityName) {
		return errors.New("entity name must be a valid identifier (e.g., product)")
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

	data := genshared.NewTemplateData()
	data.DomainName = opts.Domain
	data.DomainNameCamel = initshared.ToCamelCase(opts.Domain)
	data.EntityName = opts.EntityName
	data.EntityNameCamel = initshared.ToCamelCase(opts.EntityName)
	data.ModuleName = moduleName
	data.Features = features

	fmt.Printf("Generating entity: %s\n", data.EntityNameCamel)
	fmt.Printf("   Domain: %s\n", opts.Domain)
	fmt.Printf("   Features: %s\n", genshared.FeaturesListString(features))
	fmt.Printf("   Module: %s\n\n", data.ModuleName)

	// Generate entity file
	entityPath := filepath.Join(cfg.DomainDir, opts.Domain, opts.Domain+"_data_layer", opts.Domain+"_entities", opts.EntityName+".go")

	if !opts.Force && initshared.FileExists(entityPath) {
		fmt.Printf("   WARNING: Entity file already exists: %s (use --force to overwrite)\n", entityPath)
	} else {
		fmt.Printf("   Generating %s\n", entityPath)
		content, err := initshared.RenderTemplate(Templates, "templates/entities.go.tmpl", data)
		if err != nil {
			return errors.Wrap(err, "failed to render entity template")
		}
		if err := initshared.WriteFile(entityPath, content, opts.Force); err != nil {
			return errors.Wrap(err, "failed to write entity file")
		}
	}

	// Generate migration file
	migrationPath := filepath.Join(cfg.DomainDir, opts.Domain, opts.Domain+"_data_layer", opts.Domain+"_migrations",
		data.MigrationTimestamp+"_create_"+opts.EntityName+"_table.go")

	if !opts.Force && initshared.FileExists(migrationPath) {
		fmt.Printf("   WARNING: Migration file already exists: %s (use --force to overwrite)\n", migrationPath)
	} else {
		fmt.Printf("   Generating %s\n", migrationPath)
		content, err := initshared.RenderTemplate(Templates, "templates/entity_migration.go.tmpl", data)
		if err != nil {
			return errors.Wrap(err, "failed to render migration template")
		}
		if err := initshared.WriteFile(migrationPath, content, opts.Force); err != nil {
			return errors.Wrap(err, "failed to write migration file")
		}
	}

	fmt.Printf("Successfully generated entity: %s\n\n", data.EntityNameCamel)
	printEntityNextSteps(data, opts, cfg)

	return nil
}

func printEntityNextSteps(data genshared.TemplateData, opts EntityOptions, cfg genshared.GeneratorConfig) {
	fmt.Printf("Next steps:\n\n")

	fmt.Printf("1. Review the generated entity:\n")
	fmt.Printf("   ./%s/%s/%s_data_layer/%s_entities/%s.go\n\n",
		cfg.DomainDir, opts.Domain, opts.Domain, opts.Domain, opts.EntityName)

	fmt.Printf("2. Review the generated migration:\n")
	fmt.Printf("   ./%s/%s/%s_data_layer/%s_migrations/%s_create_%s_table.go\n\n",
		cfg.DomainDir, opts.Domain, opts.Domain, opts.Domain, data.MigrationTimestamp, opts.EntityName)

	fmt.Printf("3. Update the domain migrations file to include your new migration:\n")
	fmt.Printf("   ./%s/%s/%s_data_layer/%s_migrations/migrations.go\n\n",
		cfg.DomainDir, opts.Domain, opts.Domain, opts.Domain)

	fmt.Printf("4. Consider creating a repository for this entity:\n")
	fmt.Printf("   pixie generate repository --domain %s --name %s --entity %s\n\n",
		opts.Domain, opts.EntityName, opts.EntityName)

	fmt.Printf("5. Test the entity and migration:\n")
	fmt.Printf("   go build ./%s/%s/%s_data_layer/%s_entities/\n",
		cfg.DomainDir, opts.Domain, opts.Domain, opts.Domain)
	fmt.Printf("   go build ./%s/%s/%s_data_layer/%s_migrations/\n\n",
		cfg.DomainDir, opts.Domain, opts.Domain, opts.Domain)
}
