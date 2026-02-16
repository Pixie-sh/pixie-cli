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

// BusinessLayerMethod represents a method signature from a business layer
type BusinessLayerMethod struct {
	Name        string
	ReturnTypes []string // List of return type names (e.g., ["deals_models.DealDetails", "error"])
}

// BusinessLayerInfo holds information about a business layer type
type BusinessLayerInfo struct {
	PackagePath string                         // Full import path
	TypeName    string                         // Type name (e.g., "DealBusinessLayer")
	Methods     map[string]BusinessLayerMethod // Method name -> method info
}

// BusinessLayerRegistry stores all discovered business layers and their methods
type BusinessLayerRegistry struct {
	// Map from package path + type name to business layer info
	// Key format: "package_path.TypeName" (e.g., "deals_business_layer.DealBusinessLayer")
	layers  map[string]*BusinessLayerInfo
	verbose bool
}

// NewBusinessLayerRegistry creates a new business layer registry
func NewBusinessLayerRegistry(verbose bool) *BusinessLayerRegistry {
	return &BusinessLayerRegistry{
		layers:  make(map[string]*BusinessLayerInfo),
		verbose: verbose,
	}
}

// ScanBusinessLayers scans the domain directory for business layer files
// and extracts method signatures.
// domainDir is the absolute path to the domain directory.
// businessLayerSuffix is the suffix used to identify business layer directories (e.g., "_business_layer").
func (r *BusinessLayerRegistry) ScanBusinessLayers(domainDir string, businessLayerSuffix string) error {
	// Find all business layer directories
	err := filepath.Walk(domainDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip directories we can't access
		}

		// Look for directories matching the business layer suffix
		if info.IsDir() && strings.HasSuffix(info.Name(), businessLayerSuffix) {
			if r.verbose {
				fmt.Printf("  Scanning business layer directory: %s\n", path)
			}
			if scanErr := r.scanBusinessLayerDirectory(path); scanErr != nil {
				if r.verbose {
					fmt.Printf("    Warning: failed to scan %s: %v\n", path, scanErr)
				}
			}
		}
		return nil
	})

	if r.verbose {
		fmt.Printf("  Total business layers registered: %d\n", len(r.layers))
		for key, info := range r.layers {
			fmt.Printf("    - %s: %d methods\n", key, len(info.Methods))
		}
	}

	return err
}

// scanBusinessLayerDirectory scans a single business layer directory
func (r *BusinessLayerRegistry) scanBusinessLayerDirectory(dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	fset := token.NewFileSet()

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		if err := r.parseBusinessLayerFile(fset, filePath); err != nil {
			if r.verbose {
				fmt.Printf("      Warning: failed to parse %s: %v\n", entry.Name(), err)
			}
		}
	}

	return nil
}

// parseBusinessLayerFile parses a Go file and extracts business layer types and their methods
func (r *BusinessLayerRegistry) parseBusinessLayerFile(fset *token.FileSet, filePath string) error {
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	packageName := node.Name.Name

	// First pass: find struct types that end with "BusinessLayer"
	businessLayerTypes := make(map[string]bool)
	ast.Inspect(node, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if _, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
				if strings.HasSuffix(typeSpec.Name.Name, "BusinessLayer") {
					businessLayerTypes[typeSpec.Name.Name] = true
				}
			}
		}
		return true
	})

	// Second pass: find methods on business layer types
	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}

		// Get receiver type name
		receiverType := extractReceiverTypeName(funcDecl.Recv.List[0].Type)
		if receiverType == "" || !businessLayerTypes[receiverType] {
			continue
		}

		// Extract return types
		returnTypes := r.extractReturnTypes(funcDecl.Type)

		// Build the registry key
		key := fmt.Sprintf("%s.%s", packageName, receiverType)

		// Get or create business layer info
		info, exists := r.layers[key]
		if !exists {
			info = &BusinessLayerInfo{
				PackagePath: packageName,
				TypeName:    receiverType,
				Methods:     make(map[string]BusinessLayerMethod),
			}
			r.layers[key] = info
		}

		// Add method
		info.Methods[funcDecl.Name.Name] = BusinessLayerMethod{
			Name:        funcDecl.Name.Name,
			ReturnTypes: returnTypes,
		}
	}

	return nil
}

// extractReceiverTypeName extracts the type name from a receiver expression
func extractReceiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		// Pointer receiver: *TypeName
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// extractReturnTypes extracts return type names from a function type
func (r *BusinessLayerRegistry) extractReturnTypes(funcType *ast.FuncType) []string {
	if funcType.Results == nil || len(funcType.Results.List) == 0 {
		return nil
	}

	var returnTypes []string
	for _, field := range funcType.Results.List {
		typeName := r.typeExprToString(field.Type)
		if typeName != "" {
			// Handle multiple names in a single field (e.g., "a, b int")
			if len(field.Names) > 1 {
				for range field.Names {
					returnTypes = append(returnTypes, typeName)
				}
			} else {
				returnTypes = append(returnTypes, typeName)
			}
		}
	}

	return returnTypes
}

// typeExprToString converts an AST type expression to a string representation
func (r *BusinessLayerRegistry) typeExprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		// package.Type
		if ident, ok := t.X.(*ast.Ident); ok {
			return fmt.Sprintf("%s.%s", ident.Name, t.Sel.Name)
		}
		return t.Sel.Name
	case *ast.StarExpr:
		// *Type
		inner := r.typeExprToString(t.X)
		if inner != "" {
			return "*" + inner
		}
	case *ast.ArrayType:
		// []Type
		inner := r.typeExprToString(t.Elt)
		if inner != "" {
			return "[]" + inner
		}
	case *ast.MapType:
		// map[Key]Value
		key := r.typeExprToString(t.Key)
		value := r.typeExprToString(t.Value)
		if key != "" && value != "" {
			return fmt.Sprintf("map[%s]%s", key, value)
		}
	case *ast.IndexExpr:
		// Generic type: Type[T]
		base := r.typeExprToString(t.X)
		index := r.typeExprToString(t.Index)
		if base != "" && index != "" {
			return fmt.Sprintf("%s[%s]", base, index)
		}
	case *ast.IndexListExpr:
		// Generic type with multiple type params: Type[T, U]
		base := r.typeExprToString(t.X)
		if base != "" {
			var indices []string
			for _, idx := range t.Indices {
				indices = append(indices, r.typeExprToString(idx))
			}
			return fmt.Sprintf("%s[%s]", base, strings.Join(indices, ", "))
		}
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func"
	}
	return ""
}

// GetMethodReturnType looks up the return type for a business layer method
// Returns the first non-error return type, or empty string if not found
func (r *BusinessLayerRegistry) GetMethodReturnType(packageName, typeName, methodName string) string {
	// Try to find the business layer
	key := fmt.Sprintf("%s.%s", packageName, typeName)
	info, exists := r.layers[key]
	if !exists {
		// Try alternative key formats
		// Sometimes the package name in imports differs from directory name
		for _, v := range r.layers {
			if v.TypeName == typeName {
				info = v
				break
			}
		}
		if info == nil {
			return ""
		}
	}

	method, exists := info.Methods[methodName]
	if !exists {
		return ""
	}

	// Return the first non-error type
	for _, rt := range method.ReturnTypes {
		if rt != "error" && !strings.HasSuffix(rt, ".error") {
			return rt
		}
	}

	return ""
}

// LookupMethodByFieldType looks up a method return type given a field type expression
// fieldType could be like "*deals_business_layer.DealBusinessLayer"
func (r *BusinessLayerRegistry) LookupMethodByFieldType(fieldType, methodName string) string {
	// Remove pointer prefix
	fieldType = strings.TrimPrefix(fieldType, "*")

	// Extract package and type name
	parts := strings.Split(fieldType, ".")
	if len(parts) != 2 {
		return ""
	}

	packageName := parts[0]
	typeName := parts[1]

	return r.GetMethodReturnType(packageName, typeName, methodName)
}
