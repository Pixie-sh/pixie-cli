package shared

import (
	"fmt"
	"strings"
	"time"
)

// TemplateData holds the data passed to scaffold templates.
type TemplateData struct {
	// Basic information
	ServiceName         string // user_management (for microservices) or custom service name
	ServiceNameCamel    string // UserManagement (for microservices) or custom service name
	DomainName          string // users
	DomainNameCamel     string // Users
	ModuleName          string // github.com/company/my-project
	RepositoryName      string // For repositories: custom name or domain name
	RepositoryNameCamel string // CamelCase version of RepositoryName
	EntityName          string // For entities: custom name or domain name
	EntityNameCamel     string // CamelCase version of EntityName

	// Features
	Features map[string]bool // Feature flags

	// Configuration
	Port               int    // HTTP server port
	MetricsPort        int    // Metrics server port
	Timestamp          string // Generation timestamp
	MigrationTimestamp string // Migration timestamp for DB migrations

	// Database configuration (with defaults)
	DatabaseHost                  string
	DatabasePort                  int
	DatabaseName                  string
	DatabaseUsername              string
	DatabasePassword              string
	DatabaseSSLMode               string
	DatabaseMaxOpenConnections    int
	DatabaseMaxIdleConnections    int
	DatabaseConnectionMaxLifetime string

	// Redis configuration (with defaults)
	RedisHost     string
	RedisPort     int
	RedisPassword string
	RedisDatabase int

	// Security configuration (with defaults)
	JWTSecretKey string
	AdminAPIKey  string
}

// NewTemplateData creates a TemplateData with timestamp defaults populated.
func NewTemplateData() TemplateData {
	return TemplateData{
		Features:           make(map[string]bool),
		Port:               8080,
		MetricsPort:        9090,
		Timestamp:          time.Now().Format(time.RFC3339),
		MigrationTimestamp: fmt.Sprintf("%d", time.Now().Unix()),

		// Database defaults
		DatabaseHost:                  "localhost",
		DatabasePort:                  5432,
		DatabaseName:                  "app_database",
		DatabaseUsername:              "postgres",
		DatabasePassword:              "password",
		DatabaseSSLMode:               "disable",
		DatabaseMaxOpenConnections:    100,
		DatabaseMaxIdleConnections:    10,
		DatabaseConnectionMaxLifetime: "1h",

		// Redis defaults
		RedisHost:     "localhost",
		RedisPort:     6379,
		RedisPassword: "",
		RedisDatabase: 0,

		// Security defaults
		JWTSecretKey: "your-secret-key",
		AdminAPIKey:  "admin-api-key",
	}
}

// IsValidSnakeCase checks whether s is a valid snake_case identifier.
func IsValidSnakeCase(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9' && i > 0) || (r == '_' && i > 0 && i < len(s)-1)) {
			return false
		}
	}
	return true
}

// IsValidIdentifier checks whether s is a valid Go-style identifier (lowercase alpha + digits).
func IsValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9' && i > 0)) {
			return false
		}
	}
	return true
}

// ParseFeatures parses a comma-separated feature string and applies template defaults.
func ParseFeatures(featureStr, templateType string) map[string]bool {
	features := make(map[string]bool)

	// Apply template defaults
	switch templateType {
	case "minimal":
		features["metrics"] = true
	case "standard":
		features["database"] = true
		features["metrics"] = true
	case "full":
		features["database"] = true
		features["metrics"] = true
		features["auth"] = true
		features["cache"] = true
		features["tokens"] = true
		features["events"] = true
		features["notifications"] = true
		features["backoffice"] = true
		features["validation"] = true
		features["adapters"] = true
		features["apis"] = true
	}

	// Parse user-specified features
	if featureStr != "" {
		for _, feature := range strings.Split(featureStr, ",") {
			feature = strings.TrimSpace(feature)
			if feature != "" {
				features[feature] = true
			}
		}
	}

	return features
}

// ResolveFeatureDependencies ensures all required feature dependencies are satisfied.
func ResolveFeatureDependencies(features map[string]bool) map[string]bool {
	dependencies := map[string][]string{
		"auth":          {"tokens"},
		"backoffice":    {"auth"},
		"events":        {"cache"},
		"notifications": {"events"},
	}

	for {
		added := false
		for feature, deps := range dependencies {
			if features[feature] {
				for _, dep := range deps {
					if !features[dep] {
						features[dep] = true
						added = true
					}
				}
			}
		}
		if !added {
			break
		}
	}

	return features
}

// EnabledFeatures returns a sorted list of enabled feature names.
func EnabledFeatures(features map[string]bool) []string {
	var enabled []string
	for feature, isEnabled := range features {
		if isEnabled {
			enabled = append(enabled, feature)
		}
	}
	return enabled
}

// FeaturesListString returns a comma-separated string of enabled features.
func FeaturesListString(features map[string]bool) string {
	enabled := EnabledFeatures(features)
	if len(enabled) == 0 {
		return "none"
	}
	return strings.Join(enabled, ", ")
}
