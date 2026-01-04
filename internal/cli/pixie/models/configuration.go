package models

// CLIConfig holds the CLI configuration loaded from JSON
type CLIConfig struct {
	Database DatabaseConfig `json:"database"`
	Redis    RedisConfig    `json:"redis"`
	Security SecurityConfig `json:"security"`
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Host                  string `json:"host"`
	Port                  int    `json:"port"`
	Name                  string `json:"name"`
	Username              string `json:"username"`
	Password              string `json:"password"`
	SSLMode               string `json:"ssl_mode"`
	MaxOpenConnections    int    `json:"max_open_connections"`
	MaxIdleConnections    int    `json:"max_idle_connections"`
	ConnectionMaxLifetime string `json:"connection_max_lifetime"`
}

// RedisConfig holds Redis connection settings
type RedisConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	Database int    `json:"database"`
}

// SecurityConfig holds security-related settings
type SecurityConfig struct {
	JWTSecretKey string `json:"jwt_secret_key"`
	AdminAPIKey  string `json:"admin_api_key"`
}

// EnvironmentConfig holds environment-specific settings
type EnvironmentConfig struct {
	Environment string `json:"environment"`
	Debug       bool   `json:"debug"`
	LogLevel    string `json:"log_level"`
}
