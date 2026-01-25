package config

import (
	"log"
	"time"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	JWTSecret         string        `validate:"required,min=32" env:"JWT_SECRET"`
	JWTAccessExpiry   time.Duration `validate:"required,duration_min=1s,duration_max=24h" env:"JWT_ACCESS_EXPIRY"`
	JWTRefreshExpiry  time.Duration `validate:"required,duration_min=24h" env:"JWT_REFRESH_EXPIRY"`
	GoogleClientID    string        `validate:"" env:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string        `validate:"" env:"GOOGLE_CLIENT_SECRET"`
	GoogleRedirectURL string        `validate:"" env:"GOOGLE_REDIRECT_URL"`
	DatabasePath      string        `validate:"required" env:"DATABASE_PATH"`
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		// JWT
		JWTSecret:         getEnv("JWT_SECRET", ""),
		JWTAccessExpiry:   getDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
		JWTRefreshExpiry:  getDuration("JWT_REFRESH_EXPIRY", 168*time.Hour),

		// Google
		GoogleClientID:    getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL: getEnv("GOOGLE_REDIRECT_URL", ""),

		// Database
		DatabasePath:      getEnv("DATABASE_PATH", "blog.db"),
	}
}

// LoadAndValidate loads environment variables, loads configuration, and validates critical settings
// Returns the config and an error if validation fails
func LoadAndValidate() (*Config, error) {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Load configuration
	cfg := Load()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	log.Println("Configuration loaded successfully")

	return cfg, nil
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
