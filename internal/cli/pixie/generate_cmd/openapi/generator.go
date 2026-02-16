package openapi

import (
	"fmt"
	"strings"
	"unicode"
)

// OpenAPISpec represents the complete OpenAPI 3.0 specification
type OpenAPISpec struct {
	OpenAPI    string                  `json:"openapi" yaml:"openapi"`
	Info       InfoSpec                `json:"info" yaml:"info"`
	Servers    []ServerSpec            `json:"servers,omitempty" yaml:"servers,omitempty"`
	Paths      map[string]PathItemSpec `json:"paths" yaml:"paths"`
	Components ComponentsSpec          `json:"components" yaml:"components"`
	Security   []map[string][]string   `json:"security,omitempty" yaml:"security,omitempty"`
	Tags       []TagSpec               `json:"tags,omitempty" yaml:"tags,omitempty"`

	// collectedScopes tracks all permission scopes used across endpoints (not serialized)
	collectedScopes map[string]struct{} `json:"-" yaml:"-"`
}

// InfoSpec represents API information
type InfoSpec struct {
	Title          string       `json:"title" yaml:"title"`
	Description    string       `json:"description,omitempty" yaml:"description,omitempty"`
	Version        string       `json:"version" yaml:"version"`
	TermsOfService string       `json:"termsOfService,omitempty" yaml:"termsOfService,omitempty"`
	Contact        *ContactSpec `json:"contact,omitempty" yaml:"contact,omitempty"`
	License        *LicenseSpec `json:"license,omitempty" yaml:"license,omitempty"`
}

// ContactSpec represents contact information
type ContactSpec struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	URL   string `json:"url,omitempty" yaml:"url,omitempty"`
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
}

// LicenseSpec represents license information
type LicenseSpec struct {
	Name string `json:"name" yaml:"name"`
	URL  string `json:"url,omitempty" yaml:"url,omitempty"`
}

// ServerSpec represents a server
type ServerSpec struct {
	URL         string                    `json:"url" yaml:"url"`
	Description string                    `json:"description,omitempty" yaml:"description,omitempty"`
	Variables   map[string]ServerVariable `json:"variables,omitempty" yaml:"variables,omitempty"`
}

// ServerVariable represents a server variable
type ServerVariable struct {
	Default     string   `json:"default" yaml:"default"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Enum        []string `json:"enum,omitempty" yaml:"enum,omitempty"`
}

// PathItemSpec represents a path item with operations
type PathItemSpec struct {
	Summary     string         `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Get         *OperationSpec `json:"get,omitempty" yaml:"get,omitempty"`
	Post        *OperationSpec `json:"post,omitempty" yaml:"post,omitempty"`
	Put         *OperationSpec `json:"put,omitempty" yaml:"put,omitempty"`
	Delete      *OperationSpec `json:"delete,omitempty" yaml:"delete,omitempty"`
	Patch       *OperationSpec `json:"patch,omitempty" yaml:"patch,omitempty"`
	Options     *OperationSpec `json:"options,omitempty" yaml:"options,omitempty"`
	Head        *OperationSpec `json:"head,omitempty" yaml:"head,omitempty"`
	Parameters  []ParameterRef `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// OperationSpec represents an HTTP operation
type OperationSpec struct {
	Tags        []string               `json:"tags,omitempty" yaml:"tags,omitempty"`
	Summary     string                 `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	OperationID string                 `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Parameters  []ParameterRef         `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	RequestBody *RequestBodyRef        `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
	Responses   map[string]ResponseRef `json:"responses" yaml:"responses"`
	Security    []map[string][]string  `json:"security,omitempty" yaml:"security,omitempty"`
	Deprecated  bool                   `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
}

// ParameterRef represents a parameter (inline or reference)
type ParameterRef struct {
	Name        string      `json:"name,omitempty" yaml:"name,omitempty"`
	In          string      `json:"in,omitempty" yaml:"in,omitempty"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool        `json:"required,omitempty" yaml:"required,omitempty"`
	Schema      *SchemaSpec `json:"schema,omitempty" yaml:"schema,omitempty"`
	Ref         string      `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

// RequestBodyRef represents a request body (inline or reference)
type RequestBodyRef struct {
	Description string                   `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool                     `json:"required,omitempty" yaml:"required,omitempty"`
	Content     map[string]MediaTypeSpec `json:"content,omitempty" yaml:"content,omitempty"`
	Ref         string                   `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

// ResponseRef represents a response (inline or reference)
type ResponseRef struct {
	Description string                   `json:"description,omitempty" yaml:"description,omitempty"`
	Content     map[string]MediaTypeSpec `json:"content,omitempty" yaml:"content,omitempty"`
	Ref         string                   `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

// MediaTypeSpec represents a media type
type MediaTypeSpec struct {
	Schema   *SchemaSpec            `json:"schema,omitempty" yaml:"schema,omitempty"`
	Example  interface{}            `json:"example,omitempty" yaml:"example,omitempty"`
	Examples map[string]ExampleSpec `json:"examples,omitempty" yaml:"examples,omitempty"`
}

// ExampleSpec represents an example
type ExampleSpec struct {
	Summary     string      `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	Value       interface{} `json:"value,omitempty" yaml:"value,omitempty"`
}

// ComponentsSpec represents reusable components
type ComponentsSpec struct {
	Schemas         map[string]SchemaSpec         `json:"schemas,omitempty" yaml:"schemas,omitempty"`
	Responses       map[string]ResponseRef        `json:"responses,omitempty" yaml:"responses,omitempty"`
	Parameters      map[string]ParameterRef       `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	RequestBodies   map[string]RequestBodyRef     `json:"requestBodies,omitempty" yaml:"requestBodies,omitempty"`
	SecuritySchemes map[string]SecuritySchemeSpec `json:"securitySchemes,omitempty" yaml:"securitySchemes,omitempty"`
}

// SecuritySchemeSpec represents a security scheme
type SecuritySchemeSpec struct {
	Type             string      `json:"type" yaml:"type"`
	Description      string      `json:"description,omitempty" yaml:"description,omitempty"`
	Name             string      `json:"name,omitempty" yaml:"name,omitempty"`
	In               string      `json:"in,omitempty" yaml:"in,omitempty"`
	Scheme           string      `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	BearerFormat     string      `json:"bearerFormat,omitempty" yaml:"bearerFormat,omitempty"`
	OpenIdConnectUrl string      `json:"openIdConnectUrl,omitempty" yaml:"openIdConnectUrl,omitempty"`
	Flows            *OAuthFlows `json:"flows,omitempty" yaml:"flows,omitempty"`
}

// OAuthFlows represents OAuth flows
type OAuthFlows struct {
	Implicit          *OAuthFlow `json:"implicit,omitempty" yaml:"implicit,omitempty"`
	Password          *OAuthFlow `json:"password,omitempty" yaml:"password,omitempty"`
	ClientCredentials *OAuthFlow `json:"clientCredentials,omitempty" yaml:"clientCredentials,omitempty"`
	AuthorizationCode *OAuthFlow `json:"authorizationCode,omitempty" yaml:"authorizationCode,omitempty"`
}

// OAuthFlow represents an OAuth flow
type OAuthFlow struct {
	AuthorizationURL string            `json:"authorizationUrl,omitempty" yaml:"authorizationUrl,omitempty"`
	TokenURL         string            `json:"tokenUrl,omitempty" yaml:"tokenUrl,omitempty"`
	RefreshURL       string            `json:"refreshUrl,omitempty" yaml:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes" yaml:"scopes"`
}

// TagSpec represents a tag
type TagSpec struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// NewOpenAPISpec creates a new OpenAPI specification.
// servers is the list of server entries; if nil/empty, the Servers field will be empty.
func NewOpenAPISpec(title, version, description string, servers []ServerSpec) *OpenAPISpec {
	s := &OpenAPISpec{
		OpenAPI: "3.0.0",
		Info: InfoSpec{
			Title:       title,
			Version:     version,
			Description: description,
		},
		Servers: servers,
		Paths:   make(map[string]PathItemSpec),
		Components: ComponentsSpec{
			Schemas: make(map[string]SchemaSpec),
			SecuritySchemes: map[string]SecuritySchemeSpec{
				"bearerAuth": {
					Type:         "http",
					Scheme:       "bearer",
					BearerFormat: "JWT",
					Description:  "JWT Bearer token authentication",
				},
			},
		},
		Tags:            []TagSpec{},
		collectedScopes: make(map[string]struct{}),
	}

	if s.Servers == nil {
		s.Servers = []ServerSpec{}
	}

	return s
}

// AddEndpoint adds an endpoint to the specification
func (s *OpenAPISpec) AddEndpoint(endpoint *EndpointSpec) {
	// Normalize path (convert :param to {param})
	path := s.normalizePath(endpoint.Path)

	// Get or create path item
	pathItem, exists := s.Paths[path]
	if !exists {
		pathItem = PathItemSpec{}
	}

	// Create operation
	operation := s.createOperation(endpoint)

	// Add operation to path item based on method
	method := strings.ToLower(endpoint.Method)
	switch method {
	case "get":
		pathItem.Get = operation
	case "post":
		pathItem.Post = operation
	case "put":
		pathItem.Put = operation
	case "delete":
		pathItem.Delete = operation
	case "patch":
		pathItem.Patch = operation
	case "options":
		pathItem.Options = operation
	case "head":
		pathItem.Head = operation
	}

	s.Paths[path] = pathItem

	// Add tag if not exists
	s.addTag(endpoint.Tags, endpoint.MSName)

	// Resolve and add schemas
	s.resolveSchemas(endpoint)

	// Collect security scopes for later finalization
	for _, sec := range endpoint.Security {
		for _, scope := range sec.Scopes {
			s.collectedScopes[scope] = struct{}{}
		}
	}
}

// FinalizeSecuritySchemes adds an OAuth2 security scheme with all collected permission scopes.
// This should be called after all endpoints have been added.
// oauthAuthorize and oauthToken are the OAuth2 authorization and token URLs respectively.
// If both are empty and there are scopes, the scheme is still created with empty URLs.
func (s *OpenAPISpec) FinalizeSecuritySchemes(oauthAuthorize, oauthToken string) {
	if len(s.collectedScopes) == 0 {
		return // No scopes collected, keep only bearerAuth
	}

	// Build scopes map with descriptions
	scopes := make(map[string]string)
	for scope := range s.collectedScopes {
		// Generate a description from the scope name
		scopes[scope] = fmt.Sprintf("Permission: %s", scope)
	}

	// Add OAuth2 security scheme with all collected scopes
	s.Components.SecuritySchemes["permissions"] = SecuritySchemeSpec{
		Type:        "oauth2",
		Description: "OAuth2 with permission scopes extracted from middleware",
		Flows: &OAuthFlows{
			AuthorizationCode: &OAuthFlow{
				AuthorizationURL: oauthAuthorize,
				TokenURL:         oauthToken,
				Scopes:           scopes,
			},
		},
	}
}

// normalizePath converts :param syntax to {param} syntax for OpenAPI
func (s *OpenAPISpec) normalizePath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + strings.TrimPrefix(part, ":") + "}"
		}
	}
	return strings.Join(parts, "/")
}

// createOperation creates an operation spec from an endpoint
func (s *OpenAPISpec) createOperation(endpoint *EndpointSpec) *OperationSpec {
	operation := &OperationSpec{
		Tags:        endpoint.Tags,
		Summary:     endpoint.Summary,
		Description: endpoint.Description,
		OperationID: s.generateOperationID(endpoint),
		Parameters:  []ParameterRef{},
		Responses:   make(map[string]ResponseRef),
	}

	// Add parameters
	for _, param := range endpoint.Parameters {
		operation.Parameters = append(operation.Parameters, ParameterRef{
			Name:        param.Name,
			In:          param.In,
			Description: param.Description,
			Required:    param.Required,
			Schema:      &param.Schema,
		})
	}

	// Add request body
	if endpoint.RequestBody != nil {
		operation.RequestBody = &RequestBodyRef{
			Description: endpoint.RequestBody.Description,
			Required:    endpoint.RequestBody.Required,
			Content: map[string]MediaTypeSpec{
				endpoint.RequestBody.ContentType: {
					Schema: &endpoint.RequestBody.Schema,
				},
			},
		}
	}

	// Add responses
	for code, response := range endpoint.Responses {
		responseRef := ResponseRef{
			Description: response.Description,
		}

		if response.ContentType != "" {
			// Wrap the response schema appropriately
			// Success responses (2xx) are wrapped with "data" field
			// Error responses (4xx, 5xx) are wrapped with "error" field
			wrappedSchema := wrapResponseSchema(&response.Schema, code)
			responseRef.Content = map[string]MediaTypeSpec{
				response.ContentType: {
					Schema: wrappedSchema,
				},
			}
		}

		operation.Responses[code] = responseRef
	}

	// Add default error response if not present
	if _, exists := operation.Responses["400"]; !exists {
		operation.Responses["400"] = ResponseRef{
			Description: "Bad Request",
			Content: map[string]MediaTypeSpec{
				"application/json": {
					Schema: &SchemaSpec{
						Type: "object",
						Properties: map[string]SchemaSpec{
							"error": {Type: "string"},
						},
					},
				},
			},
		}
	}

	// Add security
	if len(endpoint.Security) > 0 {
		operation.Security = []map[string][]string{}
		for _, sec := range endpoint.Security {
			operation.Security = append(operation.Security, map[string][]string{
				sec.Name: sec.Scopes,
			})
		}
	}

	return operation
}

// wrapResponseSchema wraps a response schema with the appropriate wrapper property
// Success responses (2xx) are wrapped with "data" field: { "data": <original_schema> }
// Error responses (4xx, 5xx) are NOT wrapped since they already have the error format from the analyzer
func wrapResponseSchema(schema *SchemaSpec, statusCode string) *SchemaSpec {
	// Don't wrap error responses - they already have the correct format with "error" property
	if strings.HasPrefix(statusCode, "4") || strings.HasPrefix(statusCode, "5") {
		return schema
	}

	// Wrap success responses with "data" property
	return &SchemaSpec{
		Type: "object",
		Properties: map[string]SchemaSpec{
			"data": *schema,
		},
	}
}

// generateOperationID generates a unique operation ID
func (s *OpenAPISpec) generateOperationID(endpoint *EndpointSpec) string {
	// Generate based on method and handler name
	method := strings.ToLower(endpoint.Method)
	handler := endpoint.Handler

	// Clean handler name
	handler = strings.TrimPrefix(handler, "*")
	handler = strings.TrimSuffix(handler, "Controller")

	return fmt.Sprintf("%s_%s", method, handler)
}

// addTag adds a tag if it doesn't exist with dynamic description
func (s *OpenAPISpec) addTag(tags []string, msName string) {
	for _, newTag := range tags {
		found := false
		for _, existingTag := range s.Tags {
			if existingTag.Name == newTag {
				found = true
				break
			}
		}
		if !found {
			description := s.generateTagDescription(newTag, msName)
			s.Tags = append(s.Tags, TagSpec{
				Name:        newTag,
				Description: description,
			})
		}
	}
}

// generateTagDescription generates a description for a tag based on its name and microservice
func (s *OpenAPISpec) generateTagDescription(tagName, _ string) string {
	// For path-based tags, generate a description based on the tag name
	// The tag name is already formatted (e.g., "Orders", "Onboarding", "User Profiles")
	return fmt.Sprintf("%s related endpoints", tagName)
}

// titleCase capitalizes the first letter of each word in the string.
// Replaces the deprecated strings.Title.
func titleCase(s string) string {
	prev := ' '
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(rune(prev)) || prev == ' ' {
			prev = r
			return unicode.ToTitle(r)
		}
		prev = r
		return r
	}, s)
}

// resolveSchemas resolves and adds schemas from endpoint
func (s *OpenAPISpec) resolveSchemas(endpoint *EndpointSpec) {
	// For now, we'll rely on the schema references
	// The actual schema resolution will be done by TypeResolver
	// which will be integrated in the main generation flow
}

// resolveAllSchemas resolves all type references in the spec and populates the schemas
func (s *OpenAPISpec) resolveAllSchemas(resolver *TypeResolver, verbose bool) error {
	// Collect all type references from the spec
	typeRefs := s.collectTypeReferences()

	if verbose {
		fmt.Printf("  Found %d unique type references to resolve\n", len(typeRefs))
	}

	// Resolve each type
	for typeName := range typeRefs {
		if verbose {
			fmt.Printf("  Resolving type: %s\n", typeName)
		}

		_, err := resolver.ResolveType(typeName)
		if err != nil {
			if verbose {
				fmt.Printf("    Warning: Failed to resolve %s: %v\n", typeName, err)
			}
			// Continue with other types even if one fails
			continue
		}
	}

	// Get all resolved schemas and add them to components
	schemas := resolver.GetSchemas()
	if s.Components.Schemas == nil {
		s.Components.Schemas = make(map[string]SchemaSpec)
	}

	for name, schema := range schemas {
		s.Components.Schemas[name] = schema
	}

	return nil
}

// collectTypeReferences collects all type references from the spec
func (s *OpenAPISpec) collectTypeReferences() map[string]bool {
	typeRefs := make(map[string]bool)

	// Iterate through all paths and operations
	for _, pathItem := range s.Paths {
		operations := []*OperationSpec{
			pathItem.Get,
			pathItem.Post,
			pathItem.Put,
			pathItem.Delete,
			pathItem.Patch,
			pathItem.Options,
			pathItem.Head,
		}

		for _, op := range operations {
			if op == nil {
				continue
			}

			// Collect from request body
			if op.RequestBody != nil {
				s.collectTypeRefsFromSchema(op.RequestBody.Content, typeRefs)
			}

			// Collect from responses
			for _, response := range op.Responses {
				s.collectTypeRefsFromSchema(response.Content, typeRefs)
			}

			// Collect from parameters
			for _, param := range op.Parameters {
				if param.Schema != nil {
					s.collectTypeRefFromSingleSchema(param.Schema, typeRefs)
				}
			}
		}
	}

	return typeRefs
}

// collectTypeRefsFromSchema collects type references from content
func (s *OpenAPISpec) collectTypeRefsFromSchema(content map[string]MediaTypeSpec, typeRefs map[string]bool) {
	for _, mediaType := range content {
		if mediaType.Schema != nil {
			s.collectTypeRefFromSingleSchema(mediaType.Schema, typeRefs)
		}
	}
}

// collectTypeRefFromSingleSchema collects a type reference from a schema
func (s *OpenAPISpec) collectTypeRefFromSingleSchema(schema *SchemaSpec, typeRefs map[string]bool) {
	if schema.Ref != "" {
		// Extract type name from $ref
		// Format: #/components/schemas/TypeName
		parts := strings.Split(schema.Ref, "/")
		if len(parts) > 0 {
			typeName := parts[len(parts)-1]
			typeRefs[typeName] = true
		}
	}

	// Recursively collect from properties
	for _, propSchema := range schema.Properties {
		s.collectTypeRefFromSingleSchema(&propSchema, typeRefs)
	}

	// Recursively collect from array items
	if schema.Items != nil {
		s.collectTypeRefFromSingleSchema(schema.Items, typeRefs)
	}
}
