package config

import (
	"log"
	"time"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	SigningSecret string        `validate:"required,min=32" env:"SIGNING_SECRET"`
	TokenExpiry   time.Duration `validate:"required,duration_min=1h" env:"TOKEN_EXPIRY"`
	DatabasePath  string        `validate:"required" env:"DATABASE_PATH"`
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		SigningSecret: getEnv("SIGNING_SECRET", ""),
		TokenExpiry:   getDuration("TOKEN_EXPIRY", 720*time.Hour),
		DatabasePath:  getEnv("DATABASE_PATH", "blog.db"),
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
