// Package config handles configuration loading from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application.
type Config struct {
	// Server configuration
	Port    string
	Host    string
	BaseURL string

	// Database configuration
	DatabaseURL          string
	DBMaxConnections     int
	DBMinConnections     int
	DBMaxConnLifetime    time.Duration
	DBMaxConnIdleTime    time.Duration
	DBHealthCheckPeriod  time.Duration

	// Redis configuration
	RedisURL          string
	RedisEnabled      bool
	RedisTTL          time.Duration
	RedisPoolSize     int
	RedisMinIdleConns int
	RedisMaxIdleConns int
	RedisConnMaxIdleTime time.Duration

	// Cache configuration
	CacheEnabled bool

	// Application configuration
	LogLevel       string
	RequestTimeout time.Duration
	Environment    string
}

// Load reads configuration from environment variables and returns a Config struct.
// It returns an error if required environment variables are missing or invalid.
func Load() (*Config, error) {
	// Try to load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	cfg := &Config{
		// Server configuration
		Port:    getEnv("PORT", "8080"),
		Host:    getEnv("HOST", "0.0.0.0"),
		BaseURL: fmt.Sprintf("http://%s:%s", getEnv("HOST", "localhost"), getEnv("PORT", "8080")),

		// Database configuration
		DatabaseURL:          getEnv("DATABASE_URL", ""),
		DBMaxConnections:     getEnvAsInt("DB_MAX_CONNECTIONS", 50),
		DBMinConnections:     getEnvAsInt("DB_MIN_CONNECTIONS", 10),
		DBMaxConnLifetime:    getEnvAsDuration("DB_MAX_CONN_LIFETIME", 1*time.Hour),
		DBMaxConnIdleTime:    getEnvAsDuration("DB_MAX_CONN_IDLE_TIME", 10*time.Minute),
		DBHealthCheckPeriod:  getEnvAsDuration("DB_HEALTH_CHECK_PERIOD", 30*time.Second),

		// Redis configuration
		RedisURL:            getEnv("REDIS_URL", "redis://localhost:6380"),
		RedisEnabled:        getEnvAsBool("REDIS_ENABLED", true),
		RedisTTL:            getEnvAsDuration("REDIS_TTL", 30*time.Second),
		RedisPoolSize:       getEnvAsInt("REDIS_POOL_SIZE", 50),
		RedisMinIdleConns:   getEnvAsInt("REDIS_MIN_IDLE_CONNS", 10),
		RedisMaxIdleConns:   getEnvAsInt("REDIS_MAX_IDLE_CONNS", 50),
		RedisConnMaxIdleTime: getEnvAsDuration("REDIS_CONN_MAX_IDLE_TIME", 5*time.Minute),

		// Cache configuration
		CacheEnabled: getEnvAsBool("CACHE_ENABLED", true),

		// Application configuration
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		RequestTimeout: getEnvAsDuration("REQUEST_TIMEOUT", 30*time.Second),
		Environment:    getEnv("ENVIRONMENT", "production"),
	}

	// Validate required configuration
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt retrieves an environment variable as an integer or returns a default value.
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvAsBool retrieves an environment variable as a boolean or returns a default value.
func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

// getEnvAsDuration retrieves an environment variable as a duration or returns a default value.
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
