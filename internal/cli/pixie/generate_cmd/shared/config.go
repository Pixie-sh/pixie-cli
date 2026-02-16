package shared

import (
	"os"
	"strings"

	"github.com/pixie-sh/errors-go"
	"gopkg.in/yaml.v3"
)

// GeneratorConfig holds all configurable paths and conventions for code generation.
// Paths are relative to the project root. Loaded from .pixie.yaml or pixie.yaml if present,
// otherwise sensible defaults are used.
type GeneratorConfig struct {
	// Directory conventions (relative to project root)
	MicroserviceDir string `yaml:"microservice_dir"` // e.g. "internal/ms"
	DomainDir       string `yaml:"domain_dir"`       // e.g. "internal/domain"
	ModelsDir       string `yaml:"models_dir"`       // e.g. "pkg/models"
	ConfigsDir      string `yaml:"configs_dir"`      // e.g. "misc/configs"
	CmdDir          string `yaml:"cmd_dir"`          // e.g. "cmd/ms"

	// Naming conventions
	MicroservicePrefix  string `yaml:"microservice_prefix"`   // e.g. "ms_"
	BusinessLayerSuffix string `yaml:"business_layer_suffix"` // e.g. "_business_layer"

	// OpenAPI defaults
	OpenAPITitle   string   `yaml:"openapi_title"`   // default title for OpenAPI spec
	OpenAPIServers []string `yaml:"openapi_servers"` // default server URLs
	OAuthAuthorize string   `yaml:"oauth_authorize"` // OAuth authorize URL
	OAuthToken     string   `yaml:"oauth_token"`     // OAuth token URL

	// Module name (auto-detected from go.mod if empty)
	ModuleName string `yaml:"module_name"`
}

// DefaultConfig returns a GeneratorConfig with sensible defaults.
func DefaultConfig() GeneratorConfig {
	return GeneratorConfig{
		MicroserviceDir:     "internal/ms",
		DomainDir:           "internal/domain",
		ModelsDir:           "pkg/models",
		ConfigsDir:          "misc/configs",
		CmdDir:              "cmd/ms",
		MicroservicePrefix:  "ms_",
		BusinessLayerSuffix: "_business_layer",
		OpenAPITitle:        "API",
		OpenAPIServers:      []string{},
		OAuthAuthorize:      "",
		OAuthToken:          "",
	}
}

// LoadConfig loads configuration from .pixie.yaml or pixie.yaml in the current directory.
// If no config file is found, returns DefaultConfig with no error.
func LoadConfig() (GeneratorConfig, error) {
	cfg := DefaultConfig()

	// Try .pixie.yaml first, then pixie.yaml
	configPaths := []string{".pixie.yaml", "pixie.yaml"}

	var data []byte
	var found bool
	for _, path := range configPaths {
		content, err := os.ReadFile(path)
		if err == nil {
			data = content
			found = true
			break
		}
	}

	if !found {
		return cfg, nil
	}

	// Parse YAML into a wrapper struct that has a "generate" key
	var wrapper struct {
		Generate GeneratorConfig `yaml:"generate"`
	}
	wrapper.Generate = cfg // preserve defaults

	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return cfg, errors.Wrap(err, "failed to parse pixie config file")
	}

	return wrapper.Generate, nil
}

// DetectModule reads go.mod from the current directory and returns the module path.
func DetectModule() (string, error) {
	content, err := os.ReadFile("go.mod")
	if err != nil {
		return "", errors.Wrap(err, "could not read go.mod file")
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}

	return "", errors.New("module name not found in go.mod")
}

// ResolveModule returns moduleName if non-empty, otherwise auto-detects from go.mod.
func ResolveModule(moduleName string) (string, error) {
	if moduleName != "" {
		return moduleName, nil
	}
	return DetectModule()
}
