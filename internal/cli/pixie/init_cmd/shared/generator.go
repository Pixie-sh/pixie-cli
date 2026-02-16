package shared

import (
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"embed"

	"github.com/pixie-sh/errors-go"
)

// ProjectConfig holds common configuration for all project types
type ProjectConfig struct {
	// Project identification
	Name   string // Project name (e.g., "my-backend")
	Output string // Output directory

	// Go-specific (optional for other stacks)
	Module string // Go module path (e.g., "github.com/company/my-backend")

	// Generation options
	Force bool // Overwrite existing files

	// Metadata
	Timestamp string // Generation timestamp
}

// RenderTemplate renders a template from an embed.FS and returns the result as bytes
func RenderTemplate(fs embed.FS, templateName string, data interface{}) ([]byte, error) {
	// Read template content
	content, err := fs.ReadFile(templateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read template: %s", templateName)
	}

	// Parse template
	tmpl, err := template.New(templateName).Parse(string(content))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse template: %s", templateName)
	}

	// Execute template to buffer
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, errors.Wrap(err, "failed to execute template: %s", templateName)
	}

	return []byte(buf.String()), nil
}

// WriteFile writes content to a file, creating directories as needed
func WriteFile(path string, content []byte, force bool) error {
	// Check if file exists
	if !force && FileExists(path) {
		return errors.New("file already exists (use --force to overwrite): %s", path)
	}

	// Create directory structure
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrap(err, "failed to create directory: %s", dir)
	}

	// Write file
	if err := os.WriteFile(path, content, 0644); err != nil {
		return errors.Wrap(err, "failed to write file: %s", path)
	}

	return nil
}

// CreateDirStructure creates a list of directories
func CreateDirStructure(basePath string, dirs []string) error {
	for _, dir := range dirs {
		fullPath := filepath.Join(basePath, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return errors.Wrap(err, "failed to create directory: %s", fullPath)
		}
	}
	return nil
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// ToCamelCase converts snake_case to CamelCase
func ToCamelCase(s string) string {
	words := strings.Split(s, "_")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, "")
}

// ToKebabCase converts snake_case or CamelCase to kebab-case
func ToKebabCase(s string) string {
	// First convert CamelCase to snake_case
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	// Then replace underscores with hyphens and lowercase
	return strings.ToLower(strings.ReplaceAll(result.String(), "_", "-"))
}
