package config

import (
	"os"
	"time"
)

// Config holds application configuration
type Config struct {
	JWTSecret         string
	JWTAccessExpiry   time.Duration
	JWTRefreshExpiry  time.Duration
	GoogleClientID    string
	GoogleClientSecret string
	GoogleRedirectURL string
	DatabasePath      string
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		JWTSecret:         getEnv("JWT_SECRET", ""),
		JWTAccessExpiry:   getDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
		JWTRefreshExpiry:  getDuration("JWT_REFRESH_EXPIRY", 168*time.Hour),
		GoogleClientID:    getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL: getEnv("GOOGLE_REDIRECT_URL", ""),
		DatabasePath:      getEnv("DATABASE_PATH", "blog.db"),
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getDuration parses a duration from environment variable or returns default
func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
