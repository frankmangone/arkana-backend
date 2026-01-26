package handlers

import (
	"arkana/config"
	"arkana/features/auth/middlewares"
	"arkana/features/auth/services"
	"database/sql"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// RegisterRoutes registers auth routes to the router
func RegisterRoutes(router *mux.Router, db *sql.DB, cfg *config.Config) {
	authService := services.NewAuthService(db, cfg)
	authMiddleware := middlewares.NewAuthMiddleware(cfg.JWTSecret)

	// Initialize Google OAuth service (optional, only if configured)
	var googleOAuthService *services.GoogleOAuthService
	if cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "" && cfg.GoogleRedirectURL != "" {
		var err error
		googleOAuthService, err = services.NewGoogleOAuthService(cfg)
		if err != nil {
			log.Printf("Warning: Failed to initialize Google OAuth service: %v", err)
		}
	}

	// Public routes
	router.HandleFunc("/api/auth/signup", SignupHandler(authService)).Methods("POST")
	router.HandleFunc("/api/auth/login", LoginHandler(authService)).Methods("POST")
	router.HandleFunc("/api/auth/refresh", RefreshHandler(authService)).Methods("POST")
	router.HandleFunc("/api/auth/logout", LogoutHandler(authService)).Methods("POST")

	// Google OAuth route (only if service is initialized)
	if googleOAuthService != nil {
		router.HandleFunc("/api/auth/google/token", GoogleTokenHandler(authService, googleOAuthService)).Methods("POST")
	}

	// Protected routes
	router.Handle("/api/auth/me", authMiddleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		GetCurrentUser(w, r, authService)
	}))).Methods("GET")
}
