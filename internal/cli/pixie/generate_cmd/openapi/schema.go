package openapi

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

// SchemaSpec represents a JSON schema
type SchemaSpec struct {
	Type                 string                `json:"type,omitempty" yaml:"type,omitempty"`
	Format               string                `json:"format,omitempty" yaml:"format,omitempty"`
	Description          string                `json:"description,omitempty" yaml:"description,omitempty"`
	Ref                  string                `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Properties           map[string]SchemaSpec `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required             []string              `json:"required,omitempty" yaml:"required,omitempty"`
	Items                *SchemaSpec           `json:"items,omitempty" yaml:"items,omitempty"`
	Enum                 []interface{}         `json:"enum,omitempty" yaml:"enum,omitempty"`
	Minimum              *float64              `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Maximum              *float64              `json:"maximum,omitempty" yaml:"maximum,omitempty"`
	MinLength            *int                  `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	MaxLength            *int                  `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
	Pattern              string                `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	AdditionalProperties interface{}           `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`
}

// TypeResolver resolves Go types to schemas
type TypeResolver struct {
	projectRoot string
	modelsDir   string
	schemas     map[string]SchemaSpec
	processed   map[string]bool
	verbose     bool
}

// NewTypeResolver creates a new type resolver
func NewTypeResolver(projectRoot string, modelsDir string, verbose bool) *TypeResolver {
	return &TypeResolver{
		projectRoot: projectRoot,
		modelsDir:   modelsDir,
		schemas:     make(map[string]SchemaSpec),
		processed:   make(map[string]bool),
		verbose:     verbose,
	}
}

// ResolveType resolves a Go type to a schema
func (r *TypeResolver) ResolveType(typeName string) (SchemaSpec, error) {
	// Handle built-in types
	if schema := r.resolveBuiltInType(typeName); schema.Type != "" {
		return schema, nil
	}

	cleanName := r.cleanTypeName(typeName)

	// Check if already processed (or currently being processed)
	if r.processed[typeName] {
		return SchemaSpec{
			Ref: fmt.Sprintf("#/components/schemas/%s", cleanName),
		}, nil
	}

	// Mark as processed BEFORE parsing to prevent infinite recursion
	// when structs have self-referential fields (e.g., Feed has field Feed *Feed)
	r.processed[typeName] = true

	// Find and parse the type definition
	schema, err := r.findAndParseType(typeName)
	if err != nil {
		if r.verbose {
			fmt.Printf("    Warning: Could not resolve type %s: %v\n", typeName, err)
		}
		// Return a generic object schema
		return SchemaSpec{Type: "object"}, nil
	}

	r.schemas[cleanName] = schema

	return SchemaSpec{
		Ref: fmt.Sprintf("#/components/schemas/%s", cleanName),
	}, nil
}

// GetSchemas returns all resolved schemas
func (r *TypeResolver) GetSchemas() map[string]SchemaSpec {
	return r.schemas
}

// resolveBuiltInType resolves built-in Go types and common custom types
func (r *TypeResolver) resolveBuiltInType(typeName string) SchemaSpec {
	switch typeName {
	case "string":
		return SchemaSpec{Type: "string"}
	case "int", "int8", "int16", "int32", "int64":
		return SchemaSpec{Type: "integer", Format: "int64"}
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return SchemaSpec{Type: "integer", Format: "uint64"}
	case "float32":
		return SchemaSpec{Type: "number", Format: "float"}
	case "float64":
		return SchemaSpec{Type: "number", Format: "double"}
	case "bool":
		return SchemaSpec{Type: "boolean"}
	case "object":
		return SchemaSpec{Type: "object"}
	// Custom types with specific OpenAPI formats
	case "time.Time", "Time":
		return SchemaSpec{Type: "string", Format: "date-time"}
	case "uid.UID", "UID":
		return SchemaSpec{Type: "string", Format: "uuid"}
	}
	return SchemaSpec{}
}

// cleanTypeName cleans a type name for use in schemas
func (r *TypeResolver) cleanTypeName(typeName string) string {
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
			return r.cleanTypeName(innerType)
		}
	}

	// Remove package prefix (e.g., "companies_models.CompanyCreate" -> "CompanyCreate")
	parts := strings.Split(typeName, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return typeName
}

// findAndParseType finds and parses a type definition
func (r *TypeResolver) findAndParseType(typeName string) (SchemaSpec, error) {
	// Extract package and type name
	var pkgName, structName string
	if strings.Contains(typeName, ".") {
		parts := strings.Split(typeName, ".")
		pkgName = parts[0]
		structName = parts[1]
	} else {
		structName = typeName
	}

	// Search for the type in the configured models directory
	modelsDir := filepath.Join(r.projectRoot, r.modelsDir)

	// Try to find the package directory
	var targetDir string
	if pkgName != "" {
		// Map common package aliases to directories
		pkgDir := r.mapPackageNameToDir(pkgName)
		targetDir = filepath.Join(modelsDir, pkgDir)
	} else {
		// Search all model directories
		targetDir = modelsDir
	}

	// Find Go files in the target directory
	files, err := r.findGoFiles(targetDir)
	if err != nil {
		return SchemaSpec{}, fmt.Errorf("failed to find files: %w", err)
	}

	// Parse each file and look for the type (struct, enum, or type alias)
	for _, filePath := range files {
		schema, found, err := r.parseFileForType(filePath, structName)
		if err != nil {
			continue
		}
		if found {
			return schema, nil
		}
	}

	return SchemaSpec{}, fmt.Errorf("type %s not found", typeName)
}

// mapPackageNameToDir maps package names to directory names
func (r *TypeResolver) mapPackageNameToDir(pkgName string) string {
	// Generic fallback: strip _models suffix
	parts := strings.Split(pkgName, "_")
	if len(parts) > 1 && parts[len(parts)-1] == "models" {
		return strings.Join(parts[:len(parts)-1], "_")
	}

	return pkgName
}

// findGoFiles finds all Go files in a directory recursively
func (r *TypeResolver) findGoFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip directories we can't access
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// parseFileForStruct parses a Go file and looks for a specific struct
func (r *TypeResolver) parseFileForStruct(filePath, structName string) (SchemaSpec, bool, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return SchemaSpec{}, false, err
	}

	var targetStruct *ast.TypeSpec
	var structDoc *ast.CommentGroup

	// Find the struct declaration
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GenDecl:
			for _, spec := range x.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if typeSpec.Name.Name == structName {
						targetStruct = typeSpec
						structDoc = x.Doc
						return false
					}
				}
			}
		}
		return true
	})

	if targetStruct == nil {
		return SchemaSpec{}, false, nil
	}

	// Parse the struct
	structType, ok := targetStruct.Type.(*ast.StructType)
	if !ok {
		return SchemaSpec{}, false, fmt.Errorf("not a struct type")
	}

	schema := r.parseStruct(structType, structDoc)
	return schema, true, nil
}

// parseStruct parses a struct and generates a schema
func (r *TypeResolver) parseStruct(structType *ast.StructType, doc *ast.CommentGroup) SchemaSpec {
	schema := SchemaSpec{
		Type:       "object",
		Properties: make(map[string]SchemaSpec),
		Required:   []string{},
	}

	// Add description from doc comments
	if doc != nil {
		comments := extractComments(doc)
		if len(comments) > 0 {
			schema.Description = strings.Join(comments, " ")
		}
	}

	// Parse fields
	for _, field := range structType.Fields.List {
		r.parseField(field, &schema)
	}

	return schema
}

// parseField parses a struct field and adds it to the schema
func (r *TypeResolver) parseField(field *ast.Field, schema *SchemaSpec) {
	// Extract field name from JSON tag
	var jsonName string
	var omitempty bool
	var required bool

	if field.Tag != nil {
		tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))

		// Parse JSON tag
		if jsonTag := tag.Get("json"); jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			jsonName = parts[0]

			// Check for omitempty
			for _, opt := range parts[1:] {
				if opt == "omitempty" {
					omitempty = true
				}
			}
		}

		// Parse validate tag to determine if required
		if validateTag := tag.Get("validate"); validateTag != "" {
			if strings.Contains(validateTag, "required") {
				required = true
			}
		}
	}

	// Skip if no JSON name (unexported or ignored)
	if jsonName == "" || jsonName == "-" {
		return
	}

	// Get field type
	fieldSchema := r.parseFieldType(field.Type)

	// Add description from field comments
	if field.Doc != nil {
		comments := extractComments(field.Doc)
		if len(comments) > 0 {
			fieldSchema.Description = strings.Join(comments, " ")
		}
	}

	// Add to schema
	schema.Properties[jsonName] = fieldSchema

	// Add to required if not omitempty and required
	if !omitempty || required {
		schema.Required = append(schema.Required, jsonName)
	}
}

// parseFieldType parses a field type and returns a schema
func (r *TypeResolver) parseFieldType(fieldType ast.Expr) SchemaSpec {
	switch t := fieldType.(type) {
	case *ast.Ident:
		// Simple type - check if it's a built-in type first
		if builtInSchema := r.resolveBuiltInType(t.Name); builtInSchema.Type != "" {
			return builtInSchema
		}

		// Not a built-in type - resolve as a custom type
		// This handles nested structs like CompanySocialMedia, CreateCompanyLocationRequest, enums, etc.
		schema, err := r.ResolveType(t.Name)
		if err != nil {
			if r.verbose {
				fmt.Printf("    Warning: Could not resolve nested type %s: %v, using generic object\n", t.Name, err)
			}
			// If resolution fails, return generic object
			return SchemaSpec{Type: "object"}
		}
		return schema

	case *ast.SelectorExpr:
		// Qualified type (package.Type)
		if ident, ok := t.X.(*ast.Ident); ok {
			typeName := fmt.Sprintf("%s.%s", ident.Name, t.Sel.Name)

			// Check if this is a known custom type with special format mapping
			if builtInSchema := r.resolveBuiltInType(typeName); builtInSchema.Type != "" {
				return builtInSchema
			}

			// Otherwise, attempt full type resolution
			schema, _ := r.ResolveType(typeName)
			return schema
		}

	case *ast.ArrayType:
		// Array/slice type
		itemSchema := r.parseFieldType(t.Elt)
		return SchemaSpec{
			Type:  "array",
			Items: &itemSchema,
		}

	case *ast.MapType:
		// Map type
		valueSchema := r.parseFieldType(t.Value)
		return SchemaSpec{
			Type:                 "object",
			AdditionalProperties: valueSchema,
		}

	case *ast.StarExpr:
		// Pointer type - unwrap it
		return r.parseFieldType(t.X)
	}

	// Default to object
	return SchemaSpec{Type: "object"}
}

// parseFileForType parses a Go file and looks for a specific type declaration
// This handles both structs and type aliases (including enums)
func (r *TypeResolver) parseFileForType(filePath, typeName string) (SchemaSpec, bool, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return SchemaSpec{}, false, err
	}

	var targetTypeSpec *ast.TypeSpec
	var typeDoc *ast.CommentGroup

	// Find the type declaration
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GenDecl:
			for _, spec := range x.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if typeSpec.Name.Name == typeName {
						targetTypeSpec = typeSpec
						typeDoc = x.Doc
						return false
					}
				}
			}
		}
		return true
	})

	if targetTypeSpec == nil {
		return SchemaSpec{}, false, nil
	}

	// Check what kind of type this is
	switch t := targetTypeSpec.Type.(type) {
	case *ast.StructType:
		// It's a struct - use existing logic
		schema := r.parseStruct(t, typeDoc)
		return schema, true, nil

	case *ast.Ident:
		// Type alias or definition: type MyEnum string
		underlyingType := t.Name

		// Check if this is an enum by naming convention (must end with "Enum")
		if strings.HasSuffix(typeName, "Enum") {
			schema := SchemaSpec{
				Type:        r.mapUnderlyingTypeToOpenAPI(underlyingType),
				Description: fmt.Sprintf("Enum type: %s", typeName),
			}

			// Try to extract enum values (best effort)
			if values, ok := r.tryExtractEnumValues(node, typeName); ok && len(values) > 0 {
				schema.Enum = values
			}

			return schema, true, nil
		}

		// Not an enum - treat as the underlying type
		return r.resolveBuiltInType(underlyingType), true, nil

	case *ast.SelectorExpr:
		// Type alias to external type: type MyState = state_machine.State
		// Treat as the underlying type
		if ident, ok := t.X.(*ast.Ident); ok {
			underlyingType := fmt.Sprintf("%s.%s", ident.Name, t.Sel.Name)

			// Check if it's an enum by naming convention
			if strings.HasSuffix(typeName, "Enum") {
				schema := SchemaSpec{
					Type:        "string", // Default for external type aliases
					Description: fmt.Sprintf("Enum type: %s", typeName),
				}

				// Try to extract enum values
				if values, ok := r.tryExtractEnumValues(node, typeName); ok && len(values) > 0 {
					schema.Enum = values
				}

				return schema, true, nil
			}

			return r.resolveBuiltInType(underlyingType), true, nil
		}
	}

	return SchemaSpec{}, false, fmt.Errorf("unsupported type")
}

// tryExtractEnumValues attempts to extract enum values from const declarations
// Returns the values and true if successful
func (r *TypeResolver) tryExtractEnumValues(fileNode *ast.File, typeName string) ([]interface{}, bool) {
	var enumValues []interface{}

	// Iterate through all declarations in the file
	ast.Inspect(fileNode, func(n ast.Node) bool {
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			return true
		}

		// Check each const in this block
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			// Check if the const is of our enum type
			// IMPORTANT: Skip untyped constants (valueSpec.Type == nil) to avoid
			// extracting random string constants as enum values
			if valueSpec.Type == nil {
				continue
			}

			// Verify the type matches our enum type name
			if ident, ok := valueSpec.Type.(*ast.Ident); ok {
				if ident.Name != typeName {
					continue
				}
			} else {
				// Type is not a simple identifier, skip it
				continue
			}

			// Extract the value(s)
			for _, value := range valueSpec.Values {
				if basicLit, ok := value.(*ast.BasicLit); ok {
					// String literal
					if basicLit.Kind == token.STRING {
						if unquoted, err := strconv.Unquote(basicLit.Value); err == nil {
							enumValues = append(enumValues, unquoted)
						}
					} else if basicLit.Kind == token.INT {
						// Integer literal
						if intVal, err := strconv.Atoi(basicLit.Value); err == nil {
							enumValues = append(enumValues, intVal)
						}
					}
				}
			}
		}

		return true
	})

	return enumValues, len(enumValues) > 0
}

// mapUnderlyingTypeToOpenAPI maps Go underlying types to OpenAPI types
func (r *TypeResolver) mapUnderlyingTypeToOpenAPI(goType string) string {
	switch goType {
	case "string":
		return "string"
	case "int", "int8", "int16", "int32", "int64":
		return "integer"
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return "integer"
	case "float32", "float64":
		return "number"
	case "bool":
		return "boolean"
	default:
		return "string" // Default to string for unknown types
	}
}
