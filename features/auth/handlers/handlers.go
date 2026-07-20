package handlers

import (
	"arkana/config"
	"arkana/features/auth/middlewares"
	"arkana/features/auth/services"
	"database/sql"
	"log"

	"github.com/gorilla/mux"
)

// RegisterRoutes registers auth routes and returns the auth middleware for use by other modules
func RegisterRoutes(router *mux.Router, db *sql.DB, cfg *config.Config) *middlewares.AuthMiddleware {
	authService := services.NewAuthService(db, cfg)
	authMiddleware := middlewares.NewAuthMiddleware(cfg.JWTSecret)

	var googleOAuthService *services.GoogleOAuthService
	if cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "" {
		var err error
		googleOAuthService, err = services.NewGoogleOAuthService(cfg)
		if err != nil {
			log.Printf("Warning: Failed to initialize Google OAuth service: %v", err)
		}
	}

	if googleOAuthService != nil {
		router.HandleFunc("/api/auth/google/token", GoogleTokenHandler(authService, googleOAuthService)).Methods("POST", "OPTIONS")
	}

	router.HandleFunc("/api/auth/refresh", RefreshHandler(authService)).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/auth/logout", LogoutHandler(authService)).Methods("POST", "OPTIONS")

	router.Handle("/api/auth/me", authMiddleware.RequireAuth(MeHandler(authService))).Methods("GET", "OPTIONS")

	return authMiddleware
}
