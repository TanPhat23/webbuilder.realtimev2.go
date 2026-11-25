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

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	env := strings.ToLower(strings.TrimSpace(getEnv("ENV", "development")))
	allowedOrigins := parseCSV(getEnv("ALLOWED_ORIGINS", ""))
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
		ServerPort:     getEnv("SERVER_PORT", "8080"),
		Database: DatabaseConfig{
			URL:      getEnv("DATABASE_URL", ""),
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			Name:     getEnv("DB_NAME", "webbuilder"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			URL:      getEnv("REDIS_URL", ""),
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		DevMode:        getEnvAsBool("DEV_MODE", false),
		ClerkSecretKey: getEnv("CLERK_SECRET_KEY", ""),
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

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		i, err := strconv.Atoi(value)
		if err == nil {
			return i
		}
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		b, err := strconv.ParseBool(value)
		if err == nil {
			return b
		}
	}
	return fallback
}
