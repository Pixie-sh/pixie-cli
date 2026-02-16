package openapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	genshared "github.com/pixie-sh/pixie-cli/internal/cli/pixie/generate_cmd/shared"
)

// OpenAPISpecCmd returns the cobra command for generating OpenAPI specifications
func OpenAPISpecCmd() *cobra.Command {
	var (
		outputFile   string
		outputFormat string
		verbose      bool
		msFilter     []string
		title        string
		version      string
		description  string
	)

	cmd := &cobra.Command{
		Use:   "openapi-spec",
		Short: "Generate OpenAPI specification from HTTP controllers",
		Long: `Generate OpenAPI 3.0 specification by analyzing HTTP controllers and their handlers.

		This command scans microservice controller files, analyzes handler functions to extract
		request/response schemas, path parameters, query parameters, and generates a complete
		OpenAPI specification.

		Examples:
		# Generate OpenAPI spec for all microservices
		pixie generate openapi-spec

		# Generate for specific microservices
		pixie generate openapi-spec --ms partners --ms authentication

		# Output to a file in YAML format
		pixie generate openapi-spec --output api-spec.yaml --format yaml

		# Enable verbose logging
		pixie generate openapi-spec --verbose
`,
		Run: func(cmd *cobra.Command, args []string) {
			spec, err := generateOpenAPISpec(msFilter, title, version, description, verbose)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating OpenAPI spec: %v\n", err)
				os.Exit(1)
			}

			var output []byte
			switch outputFormat {
			case "yaml", "yml":
				output, err = yaml.Marshal(spec)
			case "json":
				output, err = json.MarshalIndent(spec, "", "  ")
			default:
				fmt.Fprintf(os.Stderr, "Unsupported output format: %s\n", outputFormat)
				os.Exit(1)
			}

			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling spec: %v\n", err)
				os.Exit(1)
			}

			if outputFile != "" {
				err = os.WriteFile(outputFile, output, 0644)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error writing to file: %v\n", err)
					os.Exit(1)
				}
				if verbose {
					fmt.Printf("OpenAPI spec written to %s\n", outputFile)
				}
			} else {
				fmt.Println(string(output))
			}
		},
	}

	// Load config for default values
	cfg, _ := genshared.LoadConfig()

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path (default: stdout)")
	cmd.Flags().StringVar(&outputFormat, "format", "yaml", "Output format (json, yaml)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	cmd.Flags().StringSliceVar(&msFilter, "ms", []string{}, "Filter by microservice names (e.g., --ms partners --ms notifications)")
	cmd.Flags().StringVar(&title, "title", cfg.OpenAPITitle, "API title")
	cmd.Flags().StringVar(&version, "version", "1.0.0", "API version")
	cmd.Flags().StringVar(&description, "description", "Auto-generated API documentation", "API description")

	return cmd
}

// generateOpenAPISpec generates the OpenAPI specification
func generateOpenAPISpec(msFilter []string, title, version, description string, verbose bool) (*OpenAPISpec, error) {
	// Get the project root directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Load configuration for directory conventions
	cfg, err := genshared.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	msDir := filepath.Join(cwd, cfg.MicroserviceDir)
	if verbose {
		fmt.Printf("Scanning microservices directory: %s\n", msDir)
	}

	// Find all microservice directories
	msDirectories, err := filepath.Glob(filepath.Join(msDir, cfg.MicroservicePrefix+"*"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan microservice directories: %w", err)
	}

	// Auto-generate title from microservice name if filtering by a single microservice
	// and using the default title
	effectiveTitle := title
	if len(msFilter) == 1 && title == cfg.OpenAPITitle {
		effectiveTitle = formatMicroserviceTitle(msFilter[0])
	}

	// Convert config server URLs to ServerSpec slice
	var servers []ServerSpec
	for _, serverURL := range cfg.OpenAPIServers {
		servers = append(servers, ServerSpec{
			URL: serverURL,
		})
	}

	// Initialize OpenAPI spec
	spec := NewOpenAPISpec(effectiveTitle, version, description, servers)

	// Initialize TypeResolver
	typeResolver := NewTypeResolver(cwd, cfg.ModelsDir, verbose)

	// Initialize Business Layer Registry and scan business layers
	if verbose {
		fmt.Printf("Scanning business layers for method signatures...\n")
	}
	blRegistry := NewBusinessLayerRegistry(verbose)
	if err := blRegistry.ScanBusinessLayers(filepath.Join(cwd, cfg.DomainDir), cfg.BusinessLayerSuffix); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to scan business layers: %v\n", err)
		// Continue anyway - responses will fall back to generic object types
	}

	// Process each microservice
	for _, msPath := range msDirectories {
		msName := filepath.Base(msPath)

		// Apply microservice filter if specified
		if len(msFilter) > 0 {
			found := false
			for _, filter := range msFilter {
				if contains(msName, filter) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if verbose {
			fmt.Printf("Processing microservice: %s\n", msName)
		}

		// Find controller files
		controllerFiles, err := findControllerFiles(msPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to scan controller files in %s: %v\n", msName, err)
			continue
		}

		// Process each controller file
		for _, filePath := range controllerFiles {
			if verbose {
				fmt.Printf("  Analyzing file: %s\n", filePath)
			}

			endpoints, err := analyzeControllerFile(filePath, msName, cwd, verbose, blRegistry)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to analyze %s: %v\n", filePath, err)
				continue
			}

			// Add endpoints to spec
			for _, endpoint := range endpoints {
				spec.AddEndpoint(endpoint)
			}
		}
	}

	// Resolve all schemas from endpoints
	if verbose {
		fmt.Printf("\nResolving schemas...\n")
	}
	err = spec.resolveAllSchemas(typeResolver, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve schemas: %w", err)
	}

	// Finalize security schemes with collected permission scopes
	spec.FinalizeSecuritySchemes(cfg.OAuthAuthorize, cfg.OAuthToken)

	if verbose {
		fmt.Printf("Total endpoints processed: %d\n", len(spec.Paths))
		fmt.Printf("Total schemas generated: %d\n", len(spec.Components.Schemas))
	}

	return spec, nil
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

// formatMicroserviceTitle formats a microservice name into a proper API title
// e.g., "ms_orders" -> "MS Orders API", "orders" -> "MS Orders API"
func formatMicroserviceTitle(msName string) string {
	// Remove ms_ prefix if present
	name := msName
	if len(name) > 3 && name[:3] == "ms_" {
		name = name[3:]
	}

	// Split by underscore and capitalize each word
	parts := splitByUnderscore(name)
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = toUpperFirst(part)
		}
	}

	return "MS " + joinParts(parts, " ") + " API"
}

// splitByUnderscore splits a string by underscores
func splitByUnderscore(s string) []string {
	var parts []string
	var current string
	for _, c := range s {
		if c == '_' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// toUpperFirst capitalizes the first character of a string
func toUpperFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	first := s[0]
	if first >= 'a' && first <= 'z' {
		first = first - 'a' + 'A'
	}
	return string(first) + s[1:]
}

// joinParts joins string parts with a separator
func joinParts(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}
