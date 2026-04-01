package db_shell_cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigPrecedence(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "pixie.yaml")
	envPath := filepath.Join(tmp, ".env")

	configContent := []byte("db:\n  driver: postgres\n  host: config-host\n  port: 6000\n  name: config-db\n  user: config-user\n  password: config-pass\n  sslmode: require\n")
	if err := os.WriteFile(configPath, configContent, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	envContent := []byte("PIXIE_DB_HOST=env-file-host\nPIXIE_DB_PORT=7000\nPIXIE_DB_NAME=env-file-db\nPIXIE_DB_USER=env-file-user\n")
	if err := os.WriteFile(envPath, envContent, 0644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	envLookup := func(key string) string {
		values := map[string]string{
			"PIXIE_DB_PORT":     "7100",
			"PIXIE_DB_PASSWORD": "env-password",
			"PIXIE_DB_SSLMODE":  "disable",
		}
		return values[key]
	}

	cfg, err := ResolveConfig(Options{
		Host: "flag-host",
		Name: "flag-db",
	}, configPath, envPath, envLookup)
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}

	if cfg.Host != "flag-host" {
		t.Fatalf("Host = %q, want flag-host", cfg.Host)
	}
	if cfg.Port != 7100 {
		t.Fatalf("Port = %d, want 7100", cfg.Port)
	}
	if cfg.Name != "flag-db" {
		t.Fatalf("Name = %q, want flag-db", cfg.Name)
	}
	if cfg.User != "env-file-user" {
		t.Fatalf("User = %q, want env-file-user", cfg.User)
	}
	if cfg.Password != "env-password" {
		t.Fatalf("Password = %q, want env-password", cfg.Password)
	}
	if cfg.SSLMode != "disable" {
		t.Fatalf("SSLMode = %q, want disable", cfg.SSLMode)
	}
	if cfg.DSN == "" {
		t.Fatal("DSN = empty, want generated DSN")
	}
}

func TestResolveConfigSQLiteDefaults(t *testing.T) {
	cfg, err := ResolveConfig(Options{Driver: "sqlite"}, "", "", func(string) string { return "" })
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}

	if cfg.Driver != "sqlite" {
		t.Fatalf("Driver = %q, want sqlite", cfg.Driver)
	}
	if cfg.DSN != defaultSQLiteDSN {
		t.Fatalf("DSN = %q, want %q", cfg.DSN, defaultSQLiteDSN)
	}
}
