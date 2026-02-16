package scaffold

import (
	"fmt"
	"path/filepath"

	"github.com/pixie-sh/errors-go"
	genshared "github.com/pixie-sh/pixie-cli/internal/cli/pixie/generate_cmd/shared"
	initshared "github.com/pixie-sh/pixie-cli/internal/cli/pixie/init_cmd/shared"
	"github.com/spf13/cobra"
)

// ServiceOptions holds all the options for service generation.
type ServiceOptions struct {
	Domain      string
	ServiceName string
	ModuleName  string
	Force       bool
}

// ServiceCmd returns the cobra command for service generation.
func ServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Generate an additional service within an existing domain",
		Long: `Generate an additional service within an existing domain.

This command creates a new service file alongside the existing domain service,
allowing you to add specialized services to handle specific business logic.

Examples:
  # Generate an email service in the users domain
  pixie generate service --domain users --name email

  # Generate a payment service in the orders domain
  pixie generate service --domain orders --name payment

  # Force overwrite existing service
  pixie generate service --domain users --name email --force
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var domain, _ = cmd.Flags().GetString("domain")
			var serviceName, _ = cmd.Flags().GetString("name")
			var moduleName, _ = cmd.Flags().GetString("module-name")
			var force, _ = cmd.Flags().GetBool("force")

			opts := ServiceOptions{
				Domain:      domain,
				ServiceName: serviceName,
				ModuleName:  moduleName,
				Force:       force,
			}

			return generateService(opts)
		},
	}

	// Required flags
	cmd.Flags().String("domain", "", "Existing domain name (required)")
	cmd.Flags().String("name", "", "Service name (required)")

	// Optional flags
	cmd.Flags().String("module-name", "", "Go module name (auto-detected if not provided)")
	cmd.Flags().Bool("force", false, "Force overwrite existing files")

	// Mark required flags
	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("name")

	return cmd
}

func generateService(opts ServiceOptions) error {
	if !genshared.IsValidIdentifier(opts.Domain) {
		return errors.New("domain name must be a valid identifier (e.g., users)")
	}
	if !genshared.IsValidIdentifier(opts.ServiceName) {
		return errors.New("service name must be a valid identifier (e.g., email)")
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
	data.ServiceName = opts.ServiceName
	data.ServiceNameCamel = initshared.ToCamelCase(opts.ServiceName)
	data.ModuleName = moduleName

	fmt.Printf("Generating service: %s\n", data.ServiceNameCamel)
	fmt.Printf("   Domain: %s\n", opts.Domain)
	fmt.Printf("   Module: %s\n\n", data.ModuleName)

	outputPath := filepath.Join(cfg.DomainDir, opts.Domain, opts.Domain+"_services", opts.ServiceName+"_service.go")

	if !opts.Force && initshared.FileExists(outputPath) {
		return errors.New("service file already exists: %s (use --force to overwrite)", outputPath)
	}

	fmt.Printf("   Generating %s\n", outputPath)

	content, err := initshared.RenderTemplate(Templates, "templates/services.go.tmpl", data)
	if err != nil {
		return errors.Wrap(err, "failed to render service template")
	}

	if err := initshared.WriteFile(outputPath, content, opts.Force); err != nil {
		return errors.Wrap(err, "failed to write service file")
	}

	fmt.Printf("Successfully generated service: %s\n\n", data.ServiceNameCamel)
	printServiceNextSteps(data, opts, cfg)

	return nil
}

func printServiceNextSteps(data genshared.TemplateData, opts ServiceOptions, cfg genshared.GeneratorConfig) {
	fmt.Printf("Next steps:\n\n")

	fmt.Printf("1. Review the generated service:\n")
	fmt.Printf("   ./%s/%s/%s_services/%s_service.go\n\n",
		cfg.DomainDir, opts.Domain, opts.Domain, opts.ServiceName)

	fmt.Printf("2. Update the domain registry if needed:\n")
	fmt.Printf("   ./%s/%s/registry.go\n", cfg.DomainDir, opts.Domain)
	fmt.Printf("   Add DI registration for your new service\n\n")

	fmt.Printf("3. Implement the service methods\n\n")

	fmt.Printf("4. Test the service:\n")
	fmt.Printf("   go build ./%s/%s/%s_services/\n", cfg.DomainDir, opts.Domain, opts.Domain)
	fmt.Printf("   go test ./%s/%s/%s_services/\n\n", cfg.DomainDir, opts.Domain, opts.Domain)
}
