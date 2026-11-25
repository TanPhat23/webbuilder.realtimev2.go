package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Env            string
	AllowedOrigins []string
	ServerPort     string
	Database       DatabaseConfig
	Redis          RedisConfig
	DevMode        bool
	ClerkSecretKey string
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	URL      string
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	URL      string
	Host     string
	Port     string
	Password string
	DB       int
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	env := strings.ToLower(strings.TrimSpace(envOrDefault("ENV", "development")))
	allowedOrigins := parseCSV(envOrDefault("ALLOWED_ORIGINS", ""))
	if env == "production" || env == "prod" {
		if len(allowedOrigins) == 0 {
			return nil, fmt.Errorf("ALLOWED_ORIGINS must not be empty when ENV=%s", env)
		}
		for _, o := range allowedOrigins {
			if o == "*" {
				return nil, fmt.Errorf("ALLOWED_ORIGINS must not contain wildcard '*' when ENV=%s", env)
			}
		}
	}

	return &Config{
		Env:            env,
		AllowedOrigins: allowedOrigins,
		ServerPort:     envOrDefault("SERVER_PORT", "8080"),
		Database: DatabaseConfig{
			URL:      envOrDefault("DATABASE_URL", ""),
			Host:     envOrDefault("DB_HOST", "localhost"),
			Port:     envOrDefault("DB_PORT", "5432"),
			User:     envOrDefault("DB_USER", "postgres"),
			Password: envOrDefault("DB_PASSWORD", "postgres"),
			Name:     envOrDefault("DB_NAME", "webbuilder"),
			SSLMode:  envOrDefault("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			URL:      envOrDefault("REDIS_URL", ""),
			Host:     envOrDefault("REDIS_HOST", "localhost"),
			Port:     envOrDefault("REDIS_PORT", "6379"),
			Password: envOrDefault("REDIS_PASSWORD", ""),
			DB:       envIntOrDefault("REDIS_DB", 0),
		},
		DevMode:        envBoolOrDefault("DEV_MODE", false),
		ClerkSecretKey: envOrDefault("CLERK_SECRET_KEY", ""),
	}, nil
}

// IsUnsafeDevMode reports whether DevMode is enabled under production, staging, or a live Clerk key.
func (c *Config) IsUnsafeDevMode() bool {
	if c == nil || !c.DevMode {
		return false
	}
	env := strings.ToLower(strings.TrimSpace(c.Env))
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	}
	if env == "production" || env == "prod" || env == "staging" {
		return true
	}
	return strings.HasPrefix(strings.TrimSpace(c.ClerkSecretKey), "sk_live_")
}

func parseCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func envOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		i, err := strconv.Atoi(value)
		if err == nil {
			return i
		}
	}
	return fallback
}

func envBoolOrDefault(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		b, err := strconv.ParseBool(value)
		if err == nil {
			return b
		}
	}
	return fallback
}
