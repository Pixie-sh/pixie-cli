package db_shell_cmd

import (
	"os"
	"path/filepath"
	"strings"
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

func TestResolveConfigSupportsBackendSystemEnvNames(t *testing.T) {
	tmp := t.TempDir()
	envPath := filepath.Join(tmp, "cli_pixie.local.env")

	envContent := []byte("DB_HOST=localhost\nDB_PORT=5432\nDB_NAME=grupoegor\nDB_USERNAME=postgres\nDB_PASSWORD=postgres\nDB_SSL_MODE=disable\n")
	if err := os.WriteFile(envPath, envContent, 0644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	cfg, err := ResolveConfig(Options{}, "", envPath, func(string) string { return "" })
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}

	if cfg.Driver != defaultPostgresDriver {
		t.Fatalf("Driver = %q, want %q", cfg.Driver, defaultPostgresDriver)
	}
	if cfg.Host != "localhost" {
		t.Fatalf("Host = %q, want localhost", cfg.Host)
	}
	if cfg.Port != 5432 {
		t.Fatalf("Port = %d, want 5432", cfg.Port)
	}
	if cfg.Name != "grupoegor" {
		t.Fatalf("Name = %q, want grupoegor", cfg.Name)
	}
	if cfg.User != "postgres" {
		t.Fatalf("User = %q, want postgres", cfg.User)
	}
	if cfg.Password != "postgres" {
		t.Fatalf("Password = %q, want postgres", cfg.Password)
	}
	if cfg.SSLMode != "disable" {
		t.Fatalf("SSLMode = %q, want disable", cfg.SSLMode)
	}
	if !strings.Contains(cfg.DSN, "dbname=grupoegor") {
		t.Fatalf("DSN = %q, want generated postgres DSN", cfg.DSN)
	}
}

func TestResolvedConfigIsSQLite(t *testing.T) {
	if !(ResolvedConfig{Driver: "sqlite"}).IsSQLite() {
		t.Fatal("IsSQLite() = false, want true")
	}

	if (ResolvedConfig{Driver: "postgres"}).IsSQLite() {
		t.Fatal("IsSQLite() = true, want false")
	}
}

func TestResolveConfigRejectsUnsupportedDriver(t *testing.T) {
	_, err := ResolveConfig(Options{Driver: "mysql"}, "", "", func(string) string { return "" })
	if err == nil {
		t.Fatal("ResolveConfig() error = nil, want unsupported driver error")
	}

	message := err.Error()
	if !strings.Contains(message, "unsupported driver: mysql") {
		t.Fatalf("error = %q, want unsupported driver details", message)
	}
	if !strings.Contains(message, "helper-backed postgres") {
		t.Fatalf("error = %q, want postgres scope guidance", message)
	}
}
