package config

import (
	"log"
	"time"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	ApiPort           string `env:"API_PORT"`
	DatabasePath      string `validate:"required" env:"DATABASE_PATH"`
	CORSAllowedOrigin string `env:"CORS_ALLOWED_ORIGIN"`

	// JWT
	JWTSecret        string        `validate:"required" env:"JWT_SECRET"`
	JWTAccessExpiry  time.Duration `env:"JWT_ACCESS_EXPIRY"`
	JWTRefreshExpiry time.Duration `env:"JWT_REFRESH_EXPIRY"`

	// Google OAuth
	GoogleClientID     string `env:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `env:"GOOGLE_CLIENT_SECRET"`
	GoogleRedirectURL  string `env:"GOOGLE_REDIRECT_URL"`

	// Meilisearch
	MeiliHost      string `env:"MEILI_HOST"`
	MeiliMasterKey string `env:"MEILI_MASTER_KEY"`

	// Redis - quiz attempt/session state (ephemeral, TTL-bound)
	RedisAddr string `env:"REDIS_ADDR"`

	// Email subscriptions
	FrontendURL             string `env:"FRONTEND_URL"`
	ResendAPIKey            string `env:"RESEND_API_KEY"`
	ResendFromEmail         string `env:"RESEND_FROM_EMAIL"`
	SubscriptionTokenSecret string `env:"SUBSCRIPTION_TOKEN_SECRET"`
	AdminHMACSecret         string `env:"ADMIN_HMAC_SECRET"`
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		ApiPort:           getEnv("API_PORT", "8082"),
		DatabasePath:      getEnv("DATABASE_PATH", "blog.db"),
		CORSAllowedOrigin: getEnv("CORS_ALLOWED_ORIGIN", "*"),

		JWTSecret:        getEnv("JWT_SECRET", ""),
		JWTAccessExpiry:  getDurationEnv("JWT_ACCESS_EXPIRY", 15*time.Minute),
		JWTRefreshExpiry: getDurationEnv("JWT_REFRESH_EXPIRY", 168*time.Hour),

		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", ""),

		MeiliHost:      getEnv("MEILI_HOST", "http://localhost:7700"),
		MeiliMasterKey: getEnv("MEILI_MASTER_KEY", ""),

		RedisAddr: getEnv("REDIS_ADDR", "localhost:6379"),

		FrontendURL:             getEnv("FRONTEND_URL", "http://localhost:5173"),
		ResendAPIKey:            getEnv("RESEND_API_KEY", ""),
		ResendFromEmail:         getEnv("RESEND_FROM_EMAIL", ""),
		SubscriptionTokenSecret: getEnv("SUBSCRIPTION_TOKEN_SECRET", ""),
		AdminHMACSecret:         getEnv("ADMIN_HMAC_SECRET", ""),
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
