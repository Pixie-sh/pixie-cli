package db_shell_cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pixie-sh/errors-go"
	"gopkg.in/yaml.v3"
)

const (
	defaultPostgresDriver = "postgres"
	defaultSQLiteDriver   = "sqlite"
	defaultSQLiteDSN      = "file:pixie-shell.db"
)

type Options struct {
	Driver   string
	DSN      string
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	SSLMode  string
}

type DBConfig struct {
	Driver   string `yaml:"driver"`
	DSN      string `yaml:"dsn"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	SSLMode  string `yaml:"sslmode"`
}

type runtimeConfigFile struct {
	DB DBConfig `yaml:"db"`
}

type EnvironmentLookup func(string) string

type ResolvedConfig struct {
	Driver   string
	DSN      string
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	SSLMode  string
}

func defaultConfig() ResolvedConfig {
	return ResolvedConfig{
		Driver:  defaultPostgresDriver,
		Host:    "localhost",
		Port:    5432,
		Name:    "postgres",
		User:    "postgres",
		SSLMode: "disable",
	}
}

func ResolveConfig(opts Options, configPath, envPath string, envLookup EnvironmentLookup) (ResolvedConfig, error) {
	if envLookup == nil {
		envLookup = os.Getenv
	}

	cfg := defaultConfig()

	fileCfg, err := loadRuntimeConfig(configPath)
	if err != nil {
		return ResolvedConfig{}, err
	}
	applyDBConfig(&cfg, fileCfg)

	envFileValues, err := loadEnvFile(envPath)
	if err != nil {
		return ResolvedConfig{}, err
	}
	applyEnvValues(&cfg, func(key string) string {
		return envFileValues[key]
	})
	applyEnvValues(&cfg, envLookup)
	applyOptions(&cfg, opts)

	if err := finalizeConfig(&cfg); err != nil {
		return ResolvedConfig{}, err
	}

	return cfg, nil
}

func (c ResolvedConfig) SQLDriverName() string {
	switch c.Driver {
	case defaultSQLiteDriver:
		return defaultSQLiteDriver
	default:
		return "pgx"
	}
}

func (c ResolvedConfig) SafeSummary() string {
	if c.Driver == defaultSQLiteDriver {
		return fmt.Sprintf("sqlite (%s)", c.DSN)
	}

	return fmt.Sprintf("postgres %s:%d/%s as %s (sslmode=%s)", c.Host, c.Port, c.Name, c.User, c.SSLMode)
}

func loadRuntimeConfig(configPath string) (DBConfig, error) {
	paths := []string{}
	if configPath != "" {
		paths = append(paths, configPath)
	} else {
		paths = append(paths, ".pixie.yaml", "pixie.yaml")
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return DBConfig{}, errors.Wrap(err, "failed to read config file: %s", path)
		}

		var cfg runtimeConfigFile
		if err := yaml.Unmarshal(content, &cfg); err != nil {
			return DBConfig{}, errors.Wrap(err, "failed to parse config file: %s", path)
		}

		return cfg.DB, nil
	}

	return DBConfig{}, nil
}

func loadEnvFile(envPath string) (map[string]string, error) {
	if envPath == "" {
		return map[string]string{}, nil
	}

	content, err := os.ReadFile(envPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read env file: %s", envPath)
	}

	values := make(map[string]string)
	for index, rawLine := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, errors.New("invalid env file line %d in %s", index+1, envPath)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		values[key] = value
	}

	return values, nil
}

func applyDBConfig(target *ResolvedConfig, source DBConfig) {
	if source.Driver != "" {
		target.Driver = normalizeDriver(source.Driver)
	}
	if source.DSN != "" {
		target.DSN = source.DSN
	}
	if source.Host != "" {
		target.Host = source.Host
	}
	if source.Port != 0 {
		target.Port = source.Port
	}
	if source.Name != "" {
		target.Name = source.Name
	}
	if source.User != "" {
		target.User = source.User
	}
	if source.Password != "" {
		target.Password = source.Password
	}
	if source.SSLMode != "" {
		target.SSLMode = source.SSLMode
	}
}

func applyOptions(target *ResolvedConfig, opts Options) {
	if opts.Driver != "" {
		target.Driver = normalizeDriver(opts.Driver)
	}
	if opts.DSN != "" {
		target.DSN = opts.DSN
	}
	if opts.Host != "" {
		target.Host = opts.Host
	}
	if opts.Port != 0 {
		target.Port = opts.Port
	}
	if opts.Name != "" {
		target.Name = opts.Name
	}
	if opts.User != "" {
		target.User = opts.User
	}
	if opts.Password != "" {
		target.Password = opts.Password
	}
	if opts.SSLMode != "" {
		target.SSLMode = opts.SSLMode
	}
}

func applyEnvValues(target *ResolvedConfig, lookup EnvironmentLookup) {
	if lookup == nil {
		return
	}

	if value := lookup("PIXIE_DB_DRIVER"); value != "" {
		target.Driver = normalizeDriver(value)
	}
	if value := lookup("PIXIE_DB_DSN"); value != "" {
		target.DSN = value
	} else if value := lookup("DATABASE_URL"); value != "" {
		target.DSN = value
	}
	if value := lookup("PIXIE_DB_HOST"); value != "" {
		target.Host = value
	} else if value := lookup("PGHOST"); value != "" {
		target.Host = value
	}
	if value := lookup("PIXIE_DB_PORT"); value != "" {
		if port, err := strconv.Atoi(value); err == nil {
			target.Port = port
		}
	} else if value := lookup("PGPORT"); value != "" {
		if port, err := strconv.Atoi(value); err == nil {
			target.Port = port
		}
	}
	if value := lookup("PIXIE_DB_NAME"); value != "" {
		target.Name = value
	} else if value := lookup("PGDATABASE"); value != "" {
		target.Name = value
	}
	if value := lookup("PIXIE_DB_USER"); value != "" {
		target.User = value
	} else if value := lookup("PGUSER"); value != "" {
		target.User = value
	}
	if value := lookup("PIXIE_DB_PASSWORD"); value != "" {
		target.Password = value
	} else if value := lookup("PGPASSWORD"); value != "" {
		target.Password = value
	}
	if value := lookup("PIXIE_DB_SSLMODE"); value != "" {
		target.SSLMode = value
	} else if value := lookup("PGSSLMODE"); value != "" {
		target.SSLMode = value
	}
}

func finalizeConfig(target *ResolvedConfig) error {
	target.Driver = normalizeDriver(target.Driver)
	if target.Driver != defaultPostgresDriver && target.Driver != defaultSQLiteDriver {
		return errors.New("unsupported driver: %s", target.Driver)
	}

	if target.Driver == defaultSQLiteDriver {
		if target.DSN == "" {
			target.DSN = defaultSQLiteDSN
		}
		return nil
	}

	if target.DSN == "" {
		if target.Name == "" {
			return errors.New("database name is required when dsn is not provided")
		}
		target.DSN = fmt.Sprintf(
			"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
			target.Host,
			target.Port,
			target.Name,
			target.User,
			target.Password,
			target.SSLMode,
		)
	}

	return nil
}

func normalizeDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "postgres", "postgresql", "pgx":
		return defaultPostgresDriver
	case "sqlite", "sqlite3":
		return defaultSQLiteDriver
	default:
		return strings.ToLower(strings.TrimSpace(driver))
	}
}

func resolveConfigPath(configPath string) string {
	if configPath == "" {
		return ""
	}
	return filepath.Clean(configPath)
}
