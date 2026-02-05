package config

import (
	"log"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	DatabasePath      string `validate:"required" env:"DATABASE_PATH"`
	CORSAllowedOrigin string `env:"CORS_ALLOWED_ORIGIN"`
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		DatabasePath:      getEnv("DATABASE_PATH", "blog.db"),
		CORSAllowedOrigin: getEnv("CORS_ALLOWED_ORIGIN", "*"),
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
