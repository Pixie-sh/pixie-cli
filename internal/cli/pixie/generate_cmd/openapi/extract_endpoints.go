package openapi

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pixie-sh/errors-go"
	"github.com/spf13/cobra"

	genshared "github.com/pixie-sh/pixie-cli/internal/cli/pixie/generate_cmd/shared"
)

// EndpointInfo represents the structure of an extracted HTTP endpoint
type EndpointInfo struct {
	Path       string   `json:"path"`
	Method     string   `json:"method"`
	Handler    string   `json:"handler"`
	MSName     string   `json:"msName"`
	Middleware []string `json:"middleware,omitempty"`
}

// ExtractEndpointsCmd returns the cobra command for extracting HTTP endpoints
func ExtractEndpointsCmd() *cobra.Command {
	var (
		outputFile   string
		outputFormat string
		verbose      bool
		msFilter     []string
	)

	cmd := &cobra.Command{
		Use:   "extract-endpoints",
		Short: "Extract HTTP endpoints from microservice controllers",
		Long: `Extract all registered HTTP endpoints from microservice controller files using AST parsing.

		This command scans the microservice directory for controller files and extracts HTTP endpoint registrations,
		including path, method, handler function, and middleware information.

		Examples:
		# Extract all endpoints and output as JSON
		pixie generate extract-endpoints

		# Extract endpoints from specific microservices
		pixie generate extract-endpoints --ms payments --ms notifications

		# Output to a file
		pixie generate extract-endpoints --output endpoints.json

		# Enable verbose logging
		pixie generate extract-endpoints --verbose
`,
		Run: func(cmd *cobra.Command, args []string) {
			endpoints, err := extractEndpoints(msFilter, verbose)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error extracting endpoints: %v\n", err)
				os.Exit(1)
			}

			// Sort endpoints for consistent output
			sort.Slice(endpoints, func(i, j int) bool {
				if endpoints[i].MSName == endpoints[j].MSName {
					return endpoints[i].Path < endpoints[j].Path
				}
				return endpoints[i].MSName < endpoints[j].MSName
			})

			output, err := json.MarshalIndent(endpoints, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				os.Exit(1)
			}

			if outputFile != "" {
				err = os.WriteFile(outputFile, output, 0644)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error writing to file: %v\n", err)
					os.Exit(1)
				}
				if verbose {
					fmt.Printf("Endpoints written to %s\n", outputFile)
				}
			} else {
				fmt.Println(string(output))
			}
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path (default: stdout)")
	cmd.Flags().StringVar(&outputFormat, "format", "json", "Output format (json, yaml, table)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	cmd.Flags().StringSliceVar(&msFilter, "ms", []string{}, "Filter by microservice names (e.g., --ms payments --ms notifications)")

	return cmd
}

// extractEndpoints scans microservice directories and extracts HTTP endpoints
func extractEndpoints(msFilter []string, verbose bool) ([]EndpointInfo, error) {
	var endpoints []EndpointInfo

	// Get the project root directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current working directory")
	}

	// Load configuration for directory conventions
	cfg, err := genshared.LoadConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load config")
	}

	msDir := filepath.Join(cwd, cfg.MicroserviceDir)
	if verbose {
		fmt.Printf("Scanning microservices directory: %s\n", msDir)
	}

	// Find all microservice directories
	msDirectories, err := filepath.Glob(filepath.Join(msDir, cfg.MicroservicePrefix+"*"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to scan microservice directories")
	}

	for _, msPath := range msDirectories {
		msName := filepath.Base(msPath)

		// Apply microservice filter if specified
		if len(msFilter) > 0 {
			found := false
			for _, filter := range msFilter {
				if strings.Contains(msName, filter) {
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

		// Find all controller files in the microservice
		controllerFiles, err := findControllerFiles(msPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to scan controller files in %s: %v\n", msName, err)
			continue
		}

		// Process each controller file
		for _, filePath := range controllerFiles {
			if verbose {
				fmt.Printf("  Processing file: %s\n", filePath)
			}

			fileEndpoints, err := parseControllerFile(filePath, msName, verbose)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", filePath, err)
				continue
			}

			endpoints = append(endpoints, fileEndpoints...)
		}
	}

	if verbose {
		fmt.Printf("Total endpoints extracted: %d\n", len(endpoints))
	}

	return endpoints, nil
}

// findControllerFiles searches for controller files in a microservice directory
func findControllerFiles(msPath string) ([]string, error) {
	var controllerFiles []string

	err := filepath.Walk(msPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			// Check if the file looks like a controller file
			basename := filepath.Base(path)
			if strings.Contains(basename, "controller") ||
				strings.Contains(basename, "http") ||
				strings.Contains(basename, "setup") {
				controllerFiles = append(controllerFiles, path)
			}
		}

		return nil
	})

	return controllerFiles, err
}

// parseControllerFile parses a Go file and extracts HTTP endpoint information
func parseControllerFile(filePath, msName string, verbose bool) ([]EndpointInfo, error) {
	var endpoints []EndpointInfo

	// Create a new token file set
	fset := token.NewFileSet()

	// Parse the Go file
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse file")
	}

	// Create a context to track route groups and variable assignments
	ctx := &parseContext{
		groupPaths:      make(map[string]string),
		groupMiddleware: make(map[string][]string),
		variables:       make(map[string]string),
		msName:          msName,
		fset:            fset,
		verbose:         verbose,
	}

	// Walk the AST to find HTTP endpoint registrations and group assignments
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			// Track variable assignments for route groups
			ctx.handleAssignment(x)
		case *ast.CallExpr:
			// Check if this is a method call that registers an HTTP endpoint
			if endpoint := ctx.parseHTTPMethodCall(x); endpoint != nil {
				endpoints = append(endpoints, *endpoint)
			}
		}
		return true
	})

	return endpoints, nil
}

// parseContext holds state during AST parsing
type parseContext struct {
	groupPaths      map[string]string   // variable name -> group path
	groupMiddleware map[string][]string // variable name -> middleware list
	variables       map[string]string   // variable name -> variable type/context
	msName          string
	fset            *token.FileSet
	verbose         bool
}

// handleAssignment processes variable assignments to track route groups and middleware
func (ctx *parseContext) handleAssignment(assign *ast.AssignStmt) {
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return
	}

	// Get the variable name being assigned
	lhs, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return
	}

	varName := lhs.Name

	// Check if this is a route group assignment
	if call, ok := assign.Rhs[0].(*ast.CallExpr); ok {
		// Check if it's a .Group() call
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Group" {
			// Extract the group path
			if len(call.Args) >= 1 {
				groupPath := extractStringLiteral(call.Args[0])
				if groupPath != "" {
					// Check if this is a nested group (group of another group)
					parentMiddleware := []string{}
					if ident, ok := sel.X.(*ast.Ident); ok {
						if parentPath, exists := ctx.groupPaths[ident.Name]; exists {
							// This is a nested group
							groupPath = parentPath + groupPath
							// Inherit parent middleware
							if parentMW, exists := ctx.groupMiddleware[ident.Name]; exists {
								parentMiddleware = parentMW
							}
						}
					}

					ctx.groupPaths[varName] = groupPath

					// Extract group-level middleware (all arguments after the path)
					groupMW := make([]string, 0)
					// Add inherited middleware first
					groupMW = append(groupMW, parentMiddleware...)

					// Add this group's middleware
					for i := 1; i < len(call.Args); i++ {
						if mw := ctx.extractEnhancedMiddlewareName(call.Args[i]); mw != "" {
							groupMW = append(groupMW, mw)
						}
					}

					ctx.groupMiddleware[varName] = groupMW

					if ctx.verbose {
						fmt.Printf("  Tracked group: %s -> %s (middleware: %v)\n", varName, groupPath, groupMW)
					}
				}
			}
		}
		// Track middleware variable assignments like: hasPermission := c.gates.IsAuthenticated.AllFeaturesOf
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			ctx.variables[varName] = fmt.Sprintf("%s.%s", extractExpressionAsString(sel.X), sel.Sel.Name)
		}
	} else if sel, ok := assign.Rhs[0].(*ast.SelectorExpr); ok {
		// Handle direct assignments like: hasPermission := c.gates.IsAuthenticated.AllFeaturesOf
		ctx.variables[varName] = extractExpressionAsString(sel)
	}
}

// parseHTTPMethodCall analyzes a method call to see if it's an HTTP endpoint registration
func (ctx *parseContext) parseHTTPMethodCall(call *ast.CallExpr) *EndpointInfo {
	// Check if this is a selector expression (e.g., group.Get, server.Post)
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	// Check if the method name is an HTTP method
	method := sel.Sel.Name
	if !isHTTPMethod(method) {
		return nil
	}

	// Extract arguments
	if len(call.Args) < 2 {
		return nil
	}

	// Extract the path (first argument)
	path := extractStringLiteral(call.Args[0])
	if path == "" {
		path = "/"
	}

	// Determine the group path and middleware by checking the selector's receiver
	groupPath := ""
	groupMW := []string{}
	if ident, ok := sel.X.(*ast.Ident); ok {
		if foundGroupPath, exists := ctx.groupPaths[ident.Name]; exists {
			groupPath = foundGroupPath
		}
		if foundGroupMW, exists := ctx.groupMiddleware[ident.Name]; exists {
			groupMW = foundGroupMW
		}
	}

	// Build full path
	fullPath := normalizePath(groupPath, path)

	// Extract middleware and handler with better logic
	var middleware []string
	var handler string

	// Parse arguments: path, [middleware...], handler
	// The last argument that looks like a handler function is the handler
	// Everything else between path and handler is middleware
	handlerIndex := -1
	for i := len(call.Args) - 1; i >= 1; i-- {
		if extractHandlerName(call.Args[i]) != "" {
			handler = extractHandlerName(call.Args[i])
			handlerIndex = i
			break
		}
	}

	if handler == "" {
		return nil
	}

	// Start with group-level middleware
	middleware = append(middleware, groupMW...)

	// Extract endpoint-specific middleware (all arguments between path and handler)
	for i := 1; i < handlerIndex; i++ {
		if mw := ctx.extractEnhancedMiddlewareName(call.Args[i]); mw != "" {
			middleware = append(middleware, mw)
		}
	}

	if ctx.verbose {
		fmt.Printf("    Found endpoint: %s %s -> %s (middleware: %v)\n", method, fullPath, handler, middleware)
	}

	return &EndpointInfo{
		Path:       fullPath,
		Method:     method,
		Handler:    handler,
		MSName:     ctx.msName,
		Middleware: middleware,
	}
}

// isHTTPMethod checks if a string is a valid HTTP method name
func isHTTPMethod(method string) bool {
	httpMethods := []string{"Get", "Post", "Put", "Delete", "Patch", "Head", "Options"}
	for _, m := range httpMethods {
		if method == m {
			return true
		}
	}
	return false
}

// extractStringLiteral extracts a string value from an AST expression
func extractStringLiteral(expr ast.Expr) string {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		// Remove quotes and return the string value
		if value, err := strconv.Unquote(lit.Value); err == nil {
			return value
		}
	}
	return ""
}

// extractHandlerName extracts the handler function name from an AST expression
func extractHandlerName(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		// Direct function reference: handlerFunc
		return x.Name
	case *ast.CallExpr:
		// Function call: handlerFunc(layers)
		if ident, ok := x.Fun.(*ast.Ident); ok {
			return ident.Name
		}
		// Method call: obj.handlerFunc(layers)
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
			return sel.Sel.Name
		}
	case *ast.SelectorExpr:
		// Method reference: c.handlerFunc
		return x.Sel.Name
	case *ast.FuncLit:
		// Inline anonymous function: func(ctx *fiber.Ctx) error { ... }
		return "<inline>"
	}
	return ""
}

// extractEnhancedMiddlewareName extracts middleware function name with parameters from an AST expression
func (ctx *parseContext) extractEnhancedMiddlewareName(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		// Simple identifier - might be a variable
		if varDef, exists := ctx.variables[x.Name]; exists {
			return varDef
		}
		return x.Name
	case *ast.CallExpr:
		// Function call - extract the function name and arguments
		if ident, ok := x.Fun.(*ast.Ident); ok {
			// Simple function call like hasPermission("Partner")
			if len(x.Args) > 0 {
				args := make([]string, len(x.Args))
				for i, arg := range x.Args {
					args[i] = ctx.extractArgument(arg)
				}
				return fmt.Sprintf("%s(%s)", ident.Name, strings.Join(args, ", "))
			}
			return ident.Name + "()"
		}
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
			// Method call like gates.IsAuthenticated.Authenticated()
			if len(x.Args) > 0 {
				args := make([]string, len(x.Args))
				for i, arg := range x.Args {
					args[i] = ctx.extractArgument(arg)
				}
				return fmt.Sprintf("%s(%s)", extractExpressionAsString(sel), strings.Join(args, ", "))
			}
			return extractExpressionAsString(sel) + "()"
		}
	case *ast.SelectorExpr:
		// Method reference without call
		return extractExpressionAsString(x)
	}
	return ""
}

// extractArgument extracts the string representation of a function argument
func (ctx *parseContext) extractArgument(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.BasicLit:
		return x.Value // This includes quotes for strings
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return extractExpressionAsString(x)
	default:
		return "?"
	}
}

// extractExpressionAsString converts an AST expression to its string representation
func extractExpressionAsString(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", extractExpressionAsString(x.X), x.Sel.Name)
	case *ast.CallExpr:
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
			return extractExpressionAsString(sel) + "()"
		}
		if ident, ok := x.Fun.(*ast.Ident); ok {
			return ident.Name + "()"
		}
		return "?"
	default:
		return "?"
	}
}

// normalizePath combines a group path and endpoint path, handling double slashes
func normalizePath(groupPath, endpointPath string) string {
	if groupPath == "" && endpointPath == "" {
		return "/"
	}

	if groupPath == "" {
		return endpointPath
	}

	if endpointPath == "" {
		return groupPath
	}

	groupPath = strings.TrimSuffix(groupPath, "/")

	if !strings.HasPrefix(endpointPath, "/") {
		endpointPath = "/" + endpointPath
	}

	if groupPath == "" || groupPath == "/" {
		return endpointPath
	}

	return groupPath + endpointPath
}

