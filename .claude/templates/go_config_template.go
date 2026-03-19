// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	App      AppConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port    string
	Host    string
	Timeout time.Duration
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	URL            string
	MaxConnections int
	MinConnections int
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	URL      string
	Enabled  bool
	TTL      time.Duration
}

// AppConfig holds application configuration
type AppConfig struct {
	LogLevel string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:    getEnv("PORT", "8080"),
			Host:    getEnv("HOST", "0.0.0.0"),
			Timeout: parseDuration(getEnv("REQUEST_TIMEOUT", "30s")),
		},
		Database: DatabaseConfig{
			URL:            getEnv("DATABASE_URL", "postgres://sensor_user:sensor_password@localhost:5432/sensor_db"),
			MaxConnections: parseInt(getEnv("DB_MAX_CONNECTIONS", "25")),
			MinConnections: parseInt(getEnv("DB_MIN_CONNECTIONS", "5")),
		},
		Redis: RedisConfig{
			URL:     getEnv("REDIS_URL", "redis://localhost:6379"),
			Enabled: parseBool(getEnv("REDIS_ENABLED", "true")),
			TTL:     parseDuration(getEnv("REDIS_TTL", "30s")),
		},
		App: AppConfig{
			LogLevel: getEnv("LOG_LEVEL", "info"),
		},
	}

	// Validate required fields
	if cfg.Database.URL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func parseInt(s string) int {
	val, _ := strconv.Atoi(s)
	return val
}

func parseBool(s string) bool {
	val, _ := strconv.ParseBool(s)
	return val
}

func parseDuration(s string) time.Duration {
	val, _ := time.ParseDuration(s)
	return val
}
