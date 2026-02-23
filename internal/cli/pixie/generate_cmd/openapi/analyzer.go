package openapi

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// EndpointSpec represents a full endpoint specification with request/response details
type EndpointSpec struct {
	Path            string
	Method          string
	Handler         string
	MSName          string
	Middleware      []string
	Summary         string
	Description     string
	Tags            []string
	Parameters      []ParameterSpec
	RequestBody     *RequestBodySpec
	Responses       map[string]ResponseSpec
	Security        []SecurityRequirement
	ControllerFile  string
	HandlerFunction *ast.FuncDecl
}

// ParameterSpec represents a path, query, or header parameter
type ParameterSpec struct {
	Name        string
	In          string // path, query, header
	Required    bool
	Description string
	Schema      SchemaSpec
}

// RequestBodySpec represents the request body
type RequestBodySpec struct {
	Description string
	Required    bool
	ContentType string
	Schema      SchemaSpec
}

// ResponseSpec represents a response
type ResponseSpec struct {
	Description string
	ContentType string
	Schema      SchemaSpec
}

// SecurityRequirement represents security requirements
type SecurityRequirement struct {
	Name   string
	Scopes []string
}

// analyzeControllerFile analyzes a controller file and extracts endpoint specifications
func analyzeControllerFile(filePath, msName, projectRoot string, verbose bool, blRegistry *BusinessLayerRegistry) ([]*EndpointSpec, error) {
	// First, extract basic endpoint info using existing logic
	basicEndpoints, err := parseControllerFile(filePath, msName, verbose)
	if err != nil {
		return nil, err
	}

	// Create analyzer context
	analyzerCtx := &analyzerContext{
		fset:                  token.NewFileSet(),
		projectRoot:           projectRoot,
		msName:                msName,
		filePath:              filePath,
		verbose:               verbose,
		functions:             make(map[string]*ast.FuncDecl),
		imports:               make(map[string]string),
		businessLayerRegistry: blRegistry,
		controllerFields:      make(map[string]string),
	}

	// Parse all Go files in the same directory to find handler functions
	dirPath := filepath.Dir(filePath)
	err = analyzerCtx.parseDirectoryFiles(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse directory files: %w", err)
	}

	if verbose {
		fmt.Printf("    Collected %d functions from directory %s\n", len(analyzerCtx.functions), dirPath)
	}

	// Enhance basic endpoints with detailed analysis
	var enhancedEndpoints []*EndpointSpec
	for _, basicEndpoint := range basicEndpoints {
		enhanced := analyzerCtx.enhanceEndpoint(basicEndpoint)
		if enhanced != nil {
			enhancedEndpoints = append(enhancedEndpoints, enhanced)
		}
	}

	return enhancedEndpoints, nil
}

// analyzerContext holds state during analysis
type analyzerContext struct {
	fset                  *token.FileSet
	projectRoot           string
	msName                string
	filePath              string
	verbose               bool
	functions             map[string]*ast.FuncDecl
	imports               map[string]string
	businessLayerRegistry *BusinessLayerRegistry
	// controllerFields maps "controllerType.fieldName" to field type
	// (e.g., "dealsBOController.businessLayer" -> "*deals_business_layer.DealBusinessLayer")
	// This prevents collisions when multiple controllers have fields with the same name
	controllerFields map[string]string
}

// knownPermissionConstants maps constant references to their actual string values.
// Add new constants here as they are used in middleware.
var knownPermissionConstants = map[string]string{
	"session_manager_models.SuperadminRoleFeature": "superadmin",
}

// parseDirectoryFiles parses all Go files in a directory to collect function declarations
func (ac *analyzerContext) parseDirectoryFiles(dirPath string) error {
	// Read all files in the directory
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// Parse each Go file
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		node, err := parser.ParseFile(ac.fset, filePath, nil, parser.ParseComments)
		if err != nil {
			if ac.verbose {
				fmt.Printf("      Warning: failed to parse %s: %v\n", entry.Name(), err)
			}
			continue
		}

		// Collect function declarations, imports, and controller struct fields from this file
		ast.Inspect(node, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				// Store both regular functions and methods (with receivers)
				if x.Name != nil {
					ac.functions[x.Name.Name] = x
				}
			case *ast.ImportSpec:
				if x.Path != nil {
					importPath := strings.Trim(x.Path.Value, `"`)
					if x.Name != nil {
						ac.imports[x.Name.Name] = importPath
					} else {
						// Extract package name from path
						parts := strings.Split(importPath, "/")
						pkgName := parts[len(parts)-1]
						ac.imports[pkgName] = importPath
					}
				}
			case *ast.TypeSpec:
				// Look for controller struct types
				if structType, ok := x.Type.(*ast.StructType); ok {
					if strings.HasSuffix(strings.ToLower(x.Name.Name), "controller") {
						ac.extractControllerFields(x.Name.Name, structType)
					}
				}
			}
			return true
		})
	}

	return nil
}

// extractControllerFields extracts field names and their types from a controller struct
// The controllerTypeName is used to namespace fields (e.g., "dealsBOController.businessLayer")
func (ac *analyzerContext) extractControllerFields(controllerTypeName string, structType *ast.StructType) {
	if structType.Fields == nil {
		return
	}

	for _, field := range structType.Fields.List {
		// Get field names
		for _, name := range field.Names {
			fieldType := ac.extractFullTypeName(field.Type)
			if fieldType != "" {
				// Use namespaced key: "controllerType.fieldName"
				key := fmt.Sprintf("%s.%s", controllerTypeName, name.Name)
				ac.controllerFields[key] = fieldType
				if ac.verbose {
					fmt.Printf("      Controller field: %s -> %s\n", key, fieldType)
				}
			}
		}
	}
}

// extractFullTypeName extracts the full type name including package prefix for pointer types
// It also resolves import aliases to actual package names
func (ac *analyzerContext) extractFullTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		// Package.Type format
		if ident, ok := t.X.(*ast.Ident); ok {
			// Try to resolve import alias to actual package name
			pkgName := ident.Name
			if importPath, exists := ac.imports[pkgName]; exists {
				// Extract the actual package name from the import path
				parts := strings.Split(importPath, "/")
				actualPkgName := parts[len(parts)-1]
				return fmt.Sprintf("%s.%s", actualPkgName, t.Sel.Name)
			}
			return fmt.Sprintf("%s.%s", pkgName, t.Sel.Name)
		}
		return t.Sel.Name
	case *ast.StarExpr:
		// Pointer type: *Type or *Package.Type
		inner := ac.extractFullTypeName(t.X)
		if inner != "" {
			return "*" + inner
		}
	case *ast.ArrayType:
		// Slice type: []Type
		inner := ac.extractFullTypeName(t.Elt)
		if inner != "" {
			return "[]" + inner
		}
	}
	return ""
}

// extractReceiverType extracts the receiver type name from a method declaration
// e.g., for `func (s *dealsBOController) GetAllDeals(...)`, returns "dealsBOController"
func (ac *analyzerContext) extractReceiverType(funcDecl *ast.FuncDecl) string {
	if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
		return ""
	}

	// Get the receiver type
	recvField := funcDecl.Recv.List[0]
	switch t := recvField.Type.(type) {
	case *ast.StarExpr:
		// Pointer receiver: *dealsBOController
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.Ident:
		// Value receiver: dealsBOController
		return t.Name
	}
	return ""
}

// getControllerFieldType looks up a field type for a specific controller
// Uses namespaced key: "controllerType.fieldName"
func (ac *analyzerContext) getControllerFieldType(controllerType, fieldName string) (string, bool) {
	key := fmt.Sprintf("%s.%s", controllerType, fieldName)
	fieldType, exists := ac.controllerFields[key]
	return fieldType, exists
}

// enhanceEndpoint enhances a basic endpoint with detailed request/response information
func (ac *analyzerContext) enhanceEndpoint(basic EndpointInfo) *EndpointSpec {
	spec := &EndpointSpec{
		Path:       basic.Path,
		Method:     strings.ToUpper(basic.Method),
		Handler:    basic.Handler,
		MSName:     basic.MSName,
		Middleware: basic.Middleware,
		Tags:       ac.assignTags(basic.Path),
		Parameters: []ParameterSpec{},
		Responses:  make(map[string]ResponseSpec),
	}

	// Extract security from middleware
	spec.Security = ac.extractSecurity(basic.Middleware)

	// Find and analyze the handler function
	handlerFunc := ac.functions[basic.Handler]
	if handlerFunc == nil {
		if ac.verbose {
			fmt.Printf("    Warning: Handler function %s not found\n", basic.Handler)
		}
		return spec
	}

	spec.HandlerFunction = handlerFunc

	// Extract the receiver type (controller type) from the function
	receiverType := ac.extractReceiverType(handlerFunc)

	// Extract comments for summary and description
	if handlerFunc.Doc != nil {
		comments := extractComments(handlerFunc.Doc)
		if len(comments) > 0 {
			spec.Summary = comments[0]
			if len(comments) > 1 {
				spec.Description = strings.Join(comments[1:], "\n")
			}
		}
	}

	// Analyze function body with the receiver type for proper field lookup
	if handlerFunc.Body != nil {
		ac.analyzeFunctionBody(handlerFunc.Body, spec, receiverType)
	}

	// Extract path parameters from path (only if not already found in function body)
	// This ensures we don't miss parameters that aren't explicitly accessed via ctx.Params()
	existingParams := make(map[string]bool)
	for _, param := range spec.Parameters {
		existingParams[param.Name] = true
	}

	pathParams := ac.extractPathParameters(spec.Path)
	for _, pathParam := range pathParams {
		if !existingParams[pathParam.Name] {
			spec.Parameters = append(spec.Parameters, pathParam)
		}
	}

	// Set default response if none found
	if len(spec.Responses) == 0 {
		spec.Responses["200"] = ResponseSpec{
			Description: "Successful response",
			ContentType: "application/json",
			Schema:      SchemaSpec{Type: "object"},
		}
	}

	return spec
}

// assignTags assigns tags to an endpoint based on the first segment of its path
// e.g., "/orders/items" -> "Orders", "/onboarding/step1" -> "Onboarding"
func (ac *analyzerContext) assignTags(path string) []string {
	// Extract the first path segment (after the leading slash)
	tag := extractPathPrefix(path)
	if tag == "" {
		// Fallback to microservice name if no valid path prefix found
		return []string{ac.msName}
	}
	return []string{tag}
}

// extractPathPrefix extracts and formats the first path segment as a tag name
// e.g., "/orders/items" -> "Orders", "/onboarding/step1" -> "Onboarding"
func extractPathPrefix(path string) string {
	// Remove leading slash and split by "/"
	trimmedPath := strings.TrimPrefix(path, "/")
	if trimmedPath == "" {
		return ""
	}

	// Get the first segment
	firstSlash := strings.Index(trimmedPath, "/")
	var firstSegment string
	if firstSlash == -1 {
		firstSegment = trimmedPath
	} else {
		firstSegment = trimmedPath[:firstSlash]
	}

	// Skip if it's a path parameter (starts with ":")
	if strings.HasPrefix(firstSegment, ":") {
		return ""
	}

	// Capitalize the first letter for the tag name
	if len(firstSegment) == 0 {
		return ""
	}

	// Convert to title case (first letter uppercase, rest lowercase)
	// Handle hyphenated segments like "user-profiles" -> "User Profiles"
	return formatTagName(firstSegment)
}

// formatTagName formats a path segment into a proper tag name
// e.g., "orders" -> "Orders", "user-profiles" -> "User Profiles"
func formatTagName(segment string) string {
	// Split by hyphens and underscores
	var parts []string
	var current string
	for _, c := range segment {
		if c == '-' || c == '_' {
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

	// Capitalize each part
	for i, part := range parts {
		if len(part) > 0 {
			first := part[0]
			if first >= 'a' && first <= 'z' {
				first = first - 'a' + 'A'
			}
			parts[i] = string(first) + strings.ToLower(part[1:])
		}
	}

	return strings.Join(parts, " ")
}

// analyzeFunctionBody analyzes the function body to extract request/response details
// receiverType is the controller type name (e.g., "dealsBOController") used for field lookups
func (ac *analyzerContext) analyzeFunctionBody(body *ast.BlockStmt, spec *EndpointSpec, receiverType string) {
	// First pass: collect variable declarations
	varTypes := make(map[string]string)
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.DeclStmt:
			// Variable declarations like: var request common.CreateContentTypeRequest
			if genDecl, ok := x.Decl.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
				for _, spec := range genDecl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for i, name := range valueSpec.Names {
							if valueSpec.Type != nil {
								varTypes[name.Name] = ac.extractTypeName(valueSpec.Type)
							} else if i < len(valueSpec.Values) {
								// Type inference from initialization
								varTypes[name.Name] = ac.inferTypeFromExpr(valueSpec.Values[i])
							}
						}
					}
				}
			}
		case *ast.AssignStmt:
			// Short variable declarations like: request := Request{}
			// Also handles: contentType, err := businessLayer.Method()
			if x.Tok == token.DEFINE {
				// For assignments with function calls on the right side, try to infer better type names
				if len(x.Rhs) == 1 {
					if callExpr, ok := x.Rhs[0].(*ast.CallExpr); ok {
						// This is a function call - try to infer a better type name
						inferredType := ac.inferResponseTypeFromCall(callExpr, receiverType)

						// Assign the inferred type to the first non-error variable
						for _, lhs := range x.Lhs {
							if ident, ok := lhs.(*ast.Ident); ok && ident.Name != "err" && ident.Name != "_" {
								if inferredType != "" {
									varTypes[ident.Name] = inferredType
								} else {
									varTypes[ident.Name] = ac.inferTypeFromExpr(x.Rhs[0])
								}
								break
							}
						}
					} else {
						// Regular assignment
						for i, lhs := range x.Lhs {
							if ident, ok := lhs.(*ast.Ident); ok && i < len(x.Rhs) {
								varTypes[ident.Name] = ac.inferTypeFromExpr(x.Rhs[i])
							}
						}
					}
				} else {
					// Multiple right-hand side values
					for i, lhs := range x.Lhs {
						if ident, ok := lhs.(*ast.Ident); ok && i < len(x.Rhs) {
							varTypes[ident.Name] = ac.inferTypeFromExpr(x.Rhs[i])
						}
					}
				}
			}
		}
		return true
	})

	// Second pass: analyze function calls with variable type information
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			ac.analyzeCallExpr(x, spec, varTypes, receiverType)
		}
		return true
	})
}

// analyzeCallExpr analyzes a function call to extract request/response information
// receiverType is the controller type name for looking up fields
func (ac *analyzerContext) analyzeCallExpr(call *ast.CallExpr, spec *EndpointSpec, varTypes map[string]string, receiverType string) {
	// Check if this is a selector expression (method call)
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	methodName := sel.Sel.Name

	switch methodName {
	case "Params", "ParamsUID", "ParamsUint64", "ParamsInt":
		// Extract path parameter: ctx.Params("param_name"), http.ParamsUID(ctx, "param_name"), etc.
		if len(call.Args) > 0 {
			// For http.ParamsUID, http.ParamsUint64, etc., the parameter name is in the second argument
			argIndex := 0
			if methodName != "Params" {
				// Methods like http.ParamsUID(ctx, "field") have the parameter name as second argument
				argIndex = 1
			}

			if argIndex < len(call.Args) {
				if paramName := extractStringLiteral(call.Args[argIndex]); paramName != "" {
					spec.Parameters = append(spec.Parameters, ParameterSpec{
						Name:        paramName,
						In:          "path",
						Required:    true,
						Description: fmt.Sprintf("Path parameter: %s", paramName),
						Schema:      ac.inferParamSchema(methodName),
					})
				}
			}
		}

	case "DeserializeFromFn":
		// Extract request body: serializer.DeserializeFromFn(ctx.BodyParser, &req)
		if len(call.Args) >= 2 {
			if reqType := ac.extractTypeFromUnaryExpr(call.Args[1], varTypes); reqType != "" {
				// Clean the type name to match the schema name in components
				cleanedTypeName := ac.cleanTypeName(reqType)
				spec.RequestBody = &RequestBodySpec{
					Description: "Request body",
					Required:    true,
					ContentType: "application/json",
					Schema: SchemaSpec{
						Ref: fmt.Sprintf("#/components/schemas/%s", cleanedTypeName),
					},
				}
			}
		}

	case "Response":
		// Extract response: http.Response(ctx, data) or http.Response(ctx, statusCode, data) or http.Response(ctx, statusCode)
		// When 3 args: first is ctx, second is status code (int), third is data
		// When 2 args with int: http.Response(ctx, statusCode) - returns {"data": "Ok"}
		// When 2 args without int: http.Response(ctx, data) - returns {"data": <data>}
		if len(call.Args) >= 2 {
			statusCode := "200"
			hasData := true
			dataArgIndex := 1

			if len(call.Args) >= 3 && ac.isIntegerLiteral(call.Args[1]) {
				// http.Response(ctx, statusCode, data)
				statusCode = ac.extractIntegerLiteral(call.Args[1])
				dataArgIndex = 2
			} else if len(call.Args) == 2 && ac.isIntegerLiteral(call.Args[1]) {
				// http.Response(ctx, statusCode) - no data, returns {"data": "Ok"}
				statusCode = ac.extractIntegerLiteral(call.Args[1])
				hasData = false
			}
			// else: http.Response(ctx, data) - dataArgIndex is already 1

			if !hasData {
				// http.Response(ctx, statusCode) returns {"data": "Ok"}
				// The wrapper will add the "data" field, so we just specify the inner type as string
				spec.Responses[statusCode] = ResponseSpec{
					Description: "Successful response",
					ContentType: "application/json",
					Schema: SchemaSpec{
						Type:        "string",
						Description: "Returns \"Ok\"",
					},
				}
			} else {
				// Check if the data argument is "err" - treat as error response
				dataArg := call.Args[dataArgIndex]
				isErrorResponse := false
				if ident, ok := dataArg.(*ast.Ident); ok && ident.Name == "err" {
					isErrorResponse = true
				}

				if isErrorResponse {
					// Error response - use the status code from the call
					spec.Responses[statusCode] = ResponseSpec{
						Description: "Error response",
						ContentType: "application/json",
						Schema: SchemaSpec{
							Type: "object",
							Properties: map[string]SchemaSpec{
								"error": {Type: "string"},
							},
						},
					}
				} else {
					respType := ac.inferTypeFromExprWithVars(dataArg, varTypes, receiverType)
					// Clean the type name to match the schema name in components
					cleanedTypeName := ac.cleanTypeName(respType)

					// Only add response schema if we found a valid type (not just a variable name without type info)
					if cleanedTypeName != "" && cleanedTypeName != "object" {
						spec.Responses[statusCode] = ResponseSpec{
							Description: "Successful response",
							ContentType: "application/json",
							Schema: SchemaSpec{
								Ref: fmt.Sprintf("#/components/schemas/%s", cleanedTypeName),
							},
						}
					} else {
						// Fallback to generic object response
						spec.Responses[statusCode] = ResponseSpec{
							Description: "Successful response",
							ContentType: "application/json",
							Schema:      SchemaSpec{Type: "object"},
						}
					}
				}
			}
		} else {
			// http.Response(ctx) - empty response
			spec.Responses["204"] = ResponseSpec{
				Description: "No content",
			}
		}

	case "ParseQueryParameters":
		// Detected query parameters
		spec.Parameters = append(spec.Parameters, ParameterSpec{
			Name:        "query",
			In:          "query",
			Required:    false,
			Description: "Query parameters for filtering, sorting, and pagination",
			Schema: SchemaSpec{
				Type: "object",
			},
		})

	case "APIError":
		// Error response
		if _, exists := spec.Responses["400"]; !exists {
			spec.Responses["400"] = ResponseSpec{
				Description: "Bad request or error response",
				ContentType: "application/json",
				Schema: SchemaSpec{
					Type: "object",
					Properties: map[string]SchemaSpec{
						"error": {Type: "string"},
					},
				},
			}
		}
	}
}

// extractPathParameters extracts parameter names from path
func (ac *analyzerContext) extractPathParameters(path string) []ParameterSpec {
	var params []ParameterSpec
	parts := strings.Split(path, "/")

	for _, part := range parts {
		if strings.HasPrefix(part, ":") {
			paramName := strings.TrimPrefix(part, ":")
			params = append(params, ParameterSpec{
				Name:        paramName,
				In:          "path",
				Required:    true,
				Description: fmt.Sprintf("Path parameter: %s", paramName),
				Schema: SchemaSpec{
					Type: "string",
				},
			})
		}
	}

	return params
}

// extractSecurity extracts security requirements from middleware
// Endpoints with NotAuthenticated() middleware do NOT require authentication
func (ac *analyzerContext) extractSecurity(middleware []string) []SecurityRequirement {
	var security []SecurityRequirement
	var scopes []string
	hasAuth := false

	for _, mw := range middleware {
		// Skip if this is a NotAuthenticated middleware - these endpoints don't require auth
		if strings.Contains(mw, "NotAuthenticated") {
			return security // Return empty security (no auth required)
		}

		// Check for authentication middleware (Authenticated, AuthenticatedInactive, etc.)
		if strings.Contains(mw, "Authenticated") || strings.Contains(mw, "IsAuthenticated") {
			hasAuth = true
		}

		// Extract permissions from middleware
		if extracted := ac.extractPermissionsFromMiddleware(mw); len(extracted) > 0 {
			scopes = append(scopes, extracted...)
		}
	}

	if hasAuth {
		if len(scopes) > 0 {
			// Has specific permissions - use "permissions" OAuth2 scheme
			security = append(security, SecurityRequirement{
				Name:   "permissions",
				Scopes: scopes,
			})
		} else {
			// Auth only, no specific permissions - use "bearerAuth"
			security = append(security, SecurityRequirement{
				Name:   "bearerAuth",
				Scopes: []string{},
			})
		}
	}

	return security
}

// extractPermissionsFromMiddleware extracts permission strings from middleware.
// Supports patterns: hasPermission("perm"), AllFeaturesOf("role"), AnyFeaturesOf("r1", "r2")
func (ac *analyzerContext) extractPermissionsFromMiddleware(mw string) []string {
	var permissions []string

	// Permission function patterns to look for
	patterns := []struct {
		prefix     string
		multiValue bool // true if can have multiple comma-separated values
	}{
		{"hasPermission(", false},
		{"AllFeaturesOf(", true},
		{"AnyFeaturesOf(", true},
		{"RequirePermissions(", true},
		{"RequireAnyPermission(", true},
	}

	for _, pattern := range patterns {
		idx := strings.Index(mw, pattern.prefix)
		if idx == -1 {
			continue
		}

		// Extract arguments between parentheses
		start := idx + len(pattern.prefix)
		// Find matching closing paren (handle nested parens)
		parenDepth := 1
		end := start
		for i := start; i < len(mw) && parenDepth > 0; i++ {
			switch mw[i] {
			case '(':
				parenDepth++
			case ')':
				parenDepth--
				if parenDepth == 0 {
					end = i
				}
			}
		}

		if end > start {
			args := mw[start:end]

			if pattern.multiValue {
				// Split by comma and clean each value
				parts := strings.Split(args, ",")
				for _, part := range parts {
					cleaned := ac.cleanPermissionArg(part)
					if cleaned != "" {
						permissions = append(permissions, cleaned)
					}
				}
			} else {
				cleaned := ac.cleanPermissionArg(args)
				if cleaned != "" {
					permissions = append(permissions, cleaned)
				}
			}
		}
	}

	return permissions
}

// cleanPermissionArg cleans a permission argument by removing quotes and whitespace,
// and resolves known constant references to their actual values.
func (ac *analyzerContext) cleanPermissionArg(arg string) string {
	// Trim whitespace
	arg = strings.TrimSpace(arg)

	// Remove surrounding quotes (single or double)
	if len(arg) >= 2 {
		if (arg[0] == '"' && arg[len(arg)-1] == '"') ||
			(arg[0] == '\'' && arg[len(arg)-1] == '\'') {
			arg = arg[1 : len(arg)-1]
		}
	}

	// Resolve known constant references to their actual values
	if resolved, ok := knownPermissionConstants[arg]; ok {
		return resolved
	}

	return arg
}

// inferParamSchema infers the parameter schema (type and format) from method name
func (ac *analyzerContext) inferParamSchema(methodName string) SchemaSpec {
	switch methodName {
	case "ParamsUID":
		return SchemaSpec{Type: "string", Format: "uuid"}
	case "ParamsUint64":
		return SchemaSpec{Type: "integer", Format: "uint64"}
	case "ParamsInt":
		return SchemaSpec{Type: "integer", Format: "int64"}
	case "Params":
		return SchemaSpec{Type: "string"}
	default:
		return SchemaSpec{Type: "string"}
	}
}

// extractTypeFromUnaryExpr extracts type name from &Type expression
func (ac *analyzerContext) extractTypeFromUnaryExpr(expr ast.Expr, varTypes map[string]string) string {
	unary, ok := expr.(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return ""
	}

	// Handle composite literal: &Type{}
	if comp, ok := unary.X.(*ast.CompositeLit); ok {
		return ac.extractTypeName(comp.Type)
	}

	// Handle identifier: &variable
	// Look up the variable type from the varTypes map
	if ident, ok := unary.X.(*ast.Ident); ok {
		if typeName, exists := varTypes[ident.Name]; exists {
			return typeName
		}
		// Fallback to the identifier name
		return ident.Name
	}

	return ""
}

// inferTypeFromExpr infers the type from an expression
func (ac *analyzerContext) inferTypeFromExpr(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.CompositeLit:
		return ac.extractTypeName(x.Type)
	case *ast.CallExpr:
		// Function call that returns a type
		if ident, ok := x.Fun.(*ast.Ident); ok {
			return ident.Name
		}
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
			return sel.Sel.Name
		}
	case *ast.Ident:
		return x.Name
	}
	return "object"
}

// inferTypeFromExprWithVars infers the type from an expression, using varTypes for lookups
// receiverType is the controller type name for looking up fields
func (ac *analyzerContext) inferTypeFromExprWithVars(expr ast.Expr, varTypes map[string]string, receiverType string) string {
	switch x := expr.(type) {
	case *ast.CompositeLit:
		return ac.extractTypeName(x.Type)
	case *ast.CallExpr:
		// Function call that returns a type - try to look up from business layer registry
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
			methodName := sel.Sel.Name

			// Check if this is a call on a business layer field (e.g., s.dealsBusinessLayer.Method())
			if fieldSel, ok := sel.X.(*ast.SelectorExpr); ok {
				// This is a chained selector: receiver.field.method
				fieldName := fieldSel.Sel.Name
				if fieldType, exists := ac.getControllerFieldType(receiverType, fieldName); exists {
					// Look up the return type from business layer registry
					if ac.businessLayerRegistry != nil {
						returnType := ac.businessLayerRegistry.LookupMethodByFieldType(fieldType, methodName)
						if returnType != "" {
							if ac.verbose {
								fmt.Printf("      Found business layer return type: %s.%s() -> %s\n", fieldName, methodName, returnType)
							}
							return returnType
						}
					}
				}
			}

			// Fallback to method name
			return sel.Sel.Name
		}
		if ident, ok := x.Fun.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.Ident:
		// Look up the variable type from varTypes map
		if typeName, exists := varTypes[x.Name]; exists {
			return typeName
		}
		// If not in varTypes, return the identifier name as fallback
		return x.Name
	case *ast.UnaryExpr:
		// Handle &variable or &Type{}
		if x.Op == token.AND {
			return ac.inferTypeFromExprWithVars(x.X, varTypes, receiverType)
		}
	}
	return "object"
}

// inferResponseTypeFromCall tries to infer a response type from a function call
// For business layer calls like: businessLayer.CreateContentType(...)
// It first looks up the return type in the business layer registry,
// then falls back to heuristics based on method name patterns
// receiverType is the controller type name for looking up fields
func (ac *analyzerContext) inferResponseTypeFromCall(call *ast.CallExpr, receiverType string) string {
	// Check if this is a method call (receiver.Method())
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		methodName := sel.Sel.Name

		// First, try to look up from business layer registry
		// Check if this is a call on a business layer field (e.g., s.businessLayer.Method())
		if fieldSel, ok := sel.X.(*ast.SelectorExpr); ok {
			// This is a chained selector: receiver.field.method
			fieldName := fieldSel.Sel.Name
			if fieldType, exists := ac.getControllerFieldType(receiverType, fieldName); exists {
				// Look up the return type from business layer registry
				if ac.businessLayerRegistry != nil {
					returnType := ac.businessLayerRegistry.LookupMethodByFieldType(fieldType, methodName)
					if returnType != "" {
						if ac.verbose {
							fmt.Printf("      Found business layer return type (from assignment): %s.%s() -> %s\n", fieldName, methodName, returnType)
						}
						return returnType
					}
				}
			}
		}

		// Fallback to heuristics: common patterns for response type names based on method names
		// CreateX -> XResponse, XView, X
		// UpdateX -> XResponse, XView, X
		// GetX -> XResponse, XView, X
		// ListX -> XListResponse, []X, XList

		// Try to extract the entity name from method name
		if strings.HasPrefix(methodName, "Create") {
			entity := strings.TrimPrefix(methodName, "Create")
			// Try common suffixes for response types
			return ac.tryFindResponseType(entity, []string{"Response", "View", ""})
		} else if strings.HasPrefix(methodName, "Update") {
			entity := strings.TrimPrefix(methodName, "Update")
			return ac.tryFindResponseType(entity, []string{"Response", "View", ""})
		} else if strings.HasPrefix(methodName, "Get") {
			entity := strings.TrimPrefix(methodName, "Get")
			return ac.tryFindResponseType(entity, []string{"Response", "View", ""})
		} else if strings.HasPrefix(methodName, "List") {
			entity := strings.TrimPrefix(methodName, "List")
			return ac.tryFindResponseType(entity, []string{"ListResponse", "List", "Response"})
		}

		// Default: use the method name with Response suffix
		return ac.tryFindResponseType(methodName, []string{"Response", ""})
	}

	return ""
}

// tryFindResponseType checks if a response type exists with various suffixes
func (ac *analyzerContext) tryFindResponseType(baseName string, suffixes []string) string {
	// For now, we'll construct the most likely type name
	// The TypeResolver will handle checking if it actually exists
	for _, suffix := range suffixes {
		typeName := baseName + suffix
		if typeName != "" {
			// Return the first candidate - TypeResolver will validate it
			return typeName
		}
	}
	return baseName
}

// extractTypeName extracts the type name from an AST type expression
func (ac *analyzerContext) extractTypeName(typeExpr ast.Expr) string {
	switch t := typeExpr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		// Package.Type format
		if ident, ok := t.X.(*ast.Ident); ok {
			return fmt.Sprintf("%s.%s", ident.Name, t.Sel.Name)
		}
		return t.Sel.Name
	}
	return ""
}

// isIntegerLiteral checks if an expression is an integer literal
func (ac *analyzerContext) isIntegerLiteral(expr ast.Expr) bool {
	basicLit, ok := expr.(*ast.BasicLit)
	if !ok {
		return false
	}
	return basicLit.Kind == token.INT
}

// extractIntegerLiteral extracts the integer value as a string from an expression
func (ac *analyzerContext) extractIntegerLiteral(expr ast.Expr) string {
	basicLit, ok := expr.(*ast.BasicLit)
	if !ok {
		return ""
	}
	if basicLit.Kind == token.INT {
		return basicLit.Value
	}
	return ""
}

// extractComments extracts comment text from a comment group
func extractComments(commentGroup *ast.CommentGroup) []string {
	var comments []string
	for _, comment := range commentGroup.List {
		text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
		text = strings.TrimSpace(strings.TrimPrefix(text, "/*"))
		text = strings.TrimSpace(strings.TrimSuffix(text, "*/"))
		if text != "" {
			comments = append(comments, text)
		}
	}
	return comments
}

// cleanTypeName cleans a type name for use in schema references
// This matches the behavior of TypeResolver.cleanTypeName to ensure references match schema names
func (ac *analyzerContext) cleanTypeName(typeName string) string {
	// Handle generic types like "operators.PaginatedResult[[]deals_models.DealPartnerList]"
	// We need to extract the inner type parameter for the schema reference
	if strings.Contains(typeName, "[") {
		// Extract the type parameter
		start := strings.Index(typeName, "[")
		end := strings.LastIndex(typeName, "]")
		if start != -1 && end != -1 && end > start {
			innerType := typeName[start+1 : end]
			// Remove slice prefix if present (e.g., "[]deals_models.DealPartnerList" -> "deals_models.DealPartnerList")
			innerType = strings.TrimPrefix(innerType, "[]")
			// Clean the inner type recursively
			return ac.cleanTypeName(innerType)
		}
	}

	// Remove package prefix (e.g., "common.CreateContentTypeRequest" -> "CreateContentTypeRequest")
	parts := strings.Split(typeName, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return typeName
}
