package shared

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"MicroserviceDir", cfg.MicroserviceDir, "internal/ms"},
		{"DomainDir", cfg.DomainDir, "internal/domain"},
		{"ModelsDir", cfg.ModelsDir, "pkg/models"},
		{"ConfigsDir", cfg.ConfigsDir, "misc/configs"},
		{"CmdDir", cfg.CmdDir, "cmd/ms"},
		{"MicroservicePrefix", cfg.MicroservicePrefix, "ms_"},
		{"BusinessLayerSuffix", cfg.BusinessLayerSuffix, "_business_layer"},
		{"OpenAPITitle", cfg.OpenAPITitle, "API"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("DefaultConfig().%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}

	if len(cfg.OpenAPIServers) != 0 {
		t.Errorf("DefaultConfig().OpenAPIServers = %v, want empty", cfg.OpenAPIServers)
	}
}

func TestLoadConfig_NoFile(t *testing.T) {
	// Run in a temp directory with no config files
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}

	// Should return defaults
	want := DefaultConfig()
	if cfg.MicroserviceDir != want.MicroserviceDir {
		t.Errorf("MicroserviceDir = %q, want %q", cfg.MicroserviceDir, want.MicroserviceDir)
	}
	if cfg.DomainDir != want.DomainDir {
		t.Errorf("DomainDir = %q, want %q", cfg.DomainDir, want.DomainDir)
	}
}

func TestLoadConfig_PixieYaml(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	content := `generate:
  microservice_dir: "custom/ms"
  domain_dir: "custom/domain"
  openapi_title: "My API"
  openapi_servers:
    - "https://api.example.com"
  oauth_authorize: "https://auth.example.com/authorize"
  oauth_token: "https://auth.example.com/token"
`
	if err := os.WriteFile("pixie.yaml", []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write pixie.yaml: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"MicroserviceDir", cfg.MicroserviceDir, "custom/ms"},
		{"DomainDir", cfg.DomainDir, "custom/domain"},
		{"OpenAPITitle", cfg.OpenAPITitle, "My API"},
		{"OAuthAuthorize", cfg.OAuthAuthorize, "https://auth.example.com/authorize"},
		{"OAuthToken", cfg.OAuthToken, "https://auth.example.com/token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("LoadConfig().%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}

	if len(cfg.OpenAPIServers) != 1 || cfg.OpenAPIServers[0] != "https://api.example.com" {
		t.Errorf("OpenAPIServers = %v, want [https://api.example.com]", cfg.OpenAPIServers)
	}
}

func TestLoadConfig_DotPixieYamlPriority(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Both files exist; .pixie.yaml should win
	if err := os.WriteFile(".pixie.yaml", []byte("generate:\n  microservice_dir: \"from-dot-pixie\"\n"), 0644); err != nil {
		t.Fatalf("Failed to write .pixie.yaml: %v", err)
	}
	if err := os.WriteFile("pixie.yaml", []byte("generate:\n  microservice_dir: \"from-pixie\"\n"), 0644); err != nil {
		t.Fatalf("Failed to write pixie.yaml: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}

	if cfg.MicroserviceDir != "from-dot-pixie" {
		t.Errorf("MicroserviceDir = %q, want %q (should prefer .pixie.yaml)", cfg.MicroserviceDir, "from-dot-pixie")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	if err := os.WriteFile(".pixie.yaml", []byte("{{invalid yaml}}"), 0644); err != nil {
		t.Fatalf("Failed to write .pixie.yaml: %v", err)
	}

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want error for invalid YAML")
	}
}

func TestLoadConfig_PartialOverride(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Only override one field; others should keep defaults
	if err := os.WriteFile("pixie.yaml", []byte("generate:\n  domain_dir: \"custom/domain\"\n"), 0644); err != nil {
		t.Fatalf("Failed to write pixie.yaml: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}

	if cfg.DomainDir != "custom/domain" {
		t.Errorf("DomainDir = %q, want %q", cfg.DomainDir, "custom/domain")
	}
	// Verify other fields retain defaults
	if cfg.MicroserviceDir != "internal/ms" {
		t.Errorf("MicroserviceDir = %q, want default %q", cfg.MicroserviceDir, "internal/ms")
	}
}

func TestDetectModule(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	goMod := `module github.com/example/myproject

go 1.21

require (
	github.com/some/dep v1.0.0
)
`
	if err := os.WriteFile("go.mod", []byte(goMod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	mod, err := DetectModule()
	if err != nil {
		t.Fatalf("DetectModule() error = %v, want nil", err)
	}
	if mod != "github.com/example/myproject" {
		t.Errorf("DetectModule() = %q, want %q", mod, "github.com/example/myproject")
	}
}

func TestDetectModule_NoGoMod(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	_, err := DetectModule()
	if err == nil {
		t.Fatal("DetectModule() error = nil, want error when go.mod missing")
	}
}

func TestDetectModule_NoModuleLine(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	if err := os.WriteFile("go.mod", []byte("go 1.21\n"), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	_, err := DetectModule()
	if err == nil {
		t.Fatal("DetectModule() error = nil, want error when module line missing")
	}
}

func TestResolveModule_Provided(t *testing.T) {
	mod, err := ResolveModule("github.com/custom/mod")
	if err != nil {
		t.Fatalf("ResolveModule() error = %v", err)
	}
	if mod != "github.com/custom/mod" {
		t.Errorf("ResolveModule() = %q, want %q", mod, "github.com/custom/mod")
	}
}

func TestResolveModule_AutoDetect(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	if err := os.WriteFile("go.mod", []byte("module github.com/auto/detected\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	mod, err := ResolveModule("")
	if err != nil {
		t.Fatalf("ResolveModule() error = %v", err)
	}
	if mod != "github.com/auto/detected" {
		t.Errorf("ResolveModule() = %q, want %q", mod, "github.com/auto/detected")
	}
}

// Prevent a regression: ensure we use filepath conventions correctly
func TestDefaultConfig_PathSeparators(t *testing.T) {
	cfg := DefaultConfig()
	// All paths should use forward slashes (Go convention, platform-agnostic)
	paths := []string{cfg.MicroserviceDir, cfg.DomainDir, cfg.ModelsDir, cfg.ConfigsDir, cfg.CmdDir}
	for _, p := range paths {
		if p != filepath.ToSlash(p) {
			t.Errorf("path %q contains non-forward-slash separators", p)
		}
	}
}
