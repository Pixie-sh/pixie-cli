package scaffold

import (
	"fmt"
	"path/filepath"

	"github.com/pixie-sh/errors-go"
	genshared "github.com/pixie-sh/pixie-cli/internal/cli/pixie/generate_cmd/shared"
	initshared "github.com/pixie-sh/pixie-cli/internal/cli/pixie/init_cmd/shared"
	"github.com/spf13/cobra"
)

// RepositoryOptions holds all the options for repository generation.
type RepositoryOptions struct {
	Domain         string
	RepositoryName string
	EntityName     string
	ModuleName     string
	Force          bool
}

// RepositoryCmd returns the cobra command for repository generation.
func RepositoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repository",
		Short: "Generate an additional repository within an existing domain",
		Long: `Generate an additional repository within an existing domain.

This command creates a new repository file alongside the existing domain repository,
allowing you to add specialized repositories to handle specific data access patterns.

Examples:
  # Generate an email repository in the users domain (uses User entity)
  pixie generate repository --domain users --name email

  # Generate a payment repository for orders with custom entity
  pixie generate repository --domain orders --name payment --entity transaction

  # Force overwrite existing repository
  pixie generate repository --domain users --name email --force
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var domain, _ = cmd.Flags().GetString("domain")
			var repositoryName, _ = cmd.Flags().GetString("name")
			var entityName, _ = cmd.Flags().GetString("entity")
			var moduleName, _ = cmd.Flags().GetString("module-name")
			var force, _ = cmd.Flags().GetBool("force")

			// Default entity name to domain name if not provided
			if entityName == "" {
				entityName = domain
			}

			opts := RepositoryOptions{
				Domain:         domain,
				RepositoryName: repositoryName,
				EntityName:     entityName,
				ModuleName:     moduleName,
				Force:          force,
			}

			return generateRepository(opts)
		},
	}

	// Required flags
	cmd.Flags().String("domain", "", "Existing domain name (required)")
	cmd.Flags().String("name", "", "Repository name (required)")

	// Optional flags
	cmd.Flags().String("entity", "", "Entity name to reference (defaults to domain name)")
	cmd.Flags().String("module-name", "", "Go module name (auto-detected if not provided)")
	cmd.Flags().Bool("force", false, "Force overwrite existing files")

	// Mark required flags
	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("name")

	return cmd
}

func generateRepository(opts RepositoryOptions) error {
	if !genshared.IsValidIdentifier(opts.Domain) {
		return errors.New("domain name must be a valid identifier (e.g., users)")
	}
	if !genshared.IsValidIdentifier(opts.RepositoryName) {
		return errors.New("repository name must be a valid identifier (e.g., email)")
	}

	cfg, err := genshared.LoadConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	moduleName, err := genshared.ResolveModule(opts.ModuleName)
	if err != nil {
		return errors.Wrap(err, "failed to detect module name")
	}

	data := genshared.NewTemplateData()
	data.DomainName = opts.Domain
	data.DomainNameCamel = initshared.ToCamelCase(opts.Domain)
	data.ModuleName = moduleName
	data.RepositoryName = opts.RepositoryName
	data.RepositoryNameCamel = initshared.ToCamelCase(opts.RepositoryName)
	data.EntityName = opts.EntityName
	data.EntityNameCamel = initshared.ToCamelCase(opts.EntityName)
	data.Features = map[string]bool{"database": true}

	fmt.Printf("Generating repository: %s\n", data.RepositoryNameCamel)
	fmt.Printf("   Domain: %s\n", opts.Domain)
	fmt.Printf("   Entity: %s\n", data.EntityNameCamel)
	fmt.Printf("   Module: %s\n\n", data.ModuleName)

	outputPath := filepath.Join(cfg.DomainDir, opts.Domain, opts.Domain+"_data_layer", opts.Domain+"_repositories", opts.RepositoryName+"_repository.go")

	if !opts.Force && initshared.FileExists(outputPath) {
		return errors.New("repository file already exists: %s (use --force to overwrite)", outputPath)
	}

	fmt.Printf("   Generating %s\n", outputPath)

	content, err := initshared.RenderTemplate(Templates, "templates/repositories.go.tmpl", data)
	if err != nil {
		return errors.Wrap(err, "failed to render repository template")
	}

	if err := initshared.WriteFile(outputPath, content, opts.Force); err != nil {
		return errors.Wrap(err, "failed to write repository file")
	}

	fmt.Printf("Successfully generated repository: %s\n\n", data.RepositoryNameCamel)
	printRepositoryNextSteps(data, opts, cfg)

	return nil
}

func printRepositoryNextSteps(data genshared.TemplateData, opts RepositoryOptions, cfg genshared.GeneratorConfig) {
	fmt.Printf("Next steps:\n\n")

	fmt.Printf("1. Review the generated repository:\n")
	fmt.Printf("   ./%s/%s/%s_data_layer/%s_repositories/%s_repository.go\n\n",
		cfg.DomainDir, opts.Domain, opts.Domain, opts.Domain, opts.RepositoryName)

	fmt.Printf("2. Update the data layer to include your new repository:\n")
	fmt.Printf("   ./%s/%s/%s_data_layer/%s_data_layer.go\n\n",
		cfg.DomainDir, opts.Domain, opts.Domain, opts.Domain)

	fmt.Printf("3. Update the domain registry if needed:\n")
	fmt.Printf("   ./%s/%s/registry.go\n\n", cfg.DomainDir, opts.Domain)

	fmt.Printf("4. Test the repository:\n")
	fmt.Printf("   go build ./%s/%s/%s_data_layer/%s_repositories/\n",
		cfg.DomainDir, opts.Domain, opts.Domain, opts.Domain)
	fmt.Printf("   go test ./%s/%s/%s_data_layer/%s_repositories/\n\n",
		cfg.DomainDir, opts.Domain, opts.Domain, opts.Domain)
}
