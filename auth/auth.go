package auth

import (
	"arkana/auth/handlers"
	"arkana/auth/middlewares"
	"arkana/auth/services"
	"arkana/src/config"
	"database/sql"

	"github.com/gorilla/mux"
)

// Initialize sets up the auth module and registers its routes
func Initialize(router *mux.Router, db *sql.DB, cfg *config.Config) {
	// Initialize services
	authService := services.NewAuthService(db, cfg)

	// Initialize middleware
	authMiddleware := middlewares.NewAuthMiddleware(cfg.JWTSecret)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService, authMiddleware)

	// Register routes
	authHandler.RegisterRoutes(router)
}
