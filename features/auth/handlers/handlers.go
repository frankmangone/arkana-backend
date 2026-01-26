package handlers

import (
	"arkana/config"
	"arkana/features/auth/middlewares"
	"arkana/features/auth/services"
	"database/sql"
	"net/http"

	"github.com/gorilla/mux"
)

// RegisterRoutes registers auth routes to the router
func RegisterRoutes(router *mux.Router, db *sql.DB, cfg *config.Config) {
	authService := services.NewAuthService(db, cfg)
	authMiddleware := middlewares.NewAuthMiddleware(cfg.JWTSecret)

	// Public routes
	router.HandleFunc("/api/auth/signup", SignupHandler(authService)).Methods("POST")
	router.HandleFunc("/api/auth/login", LoginHandler(authService)).Methods("POST")
	router.HandleFunc("/api/auth/refresh", RefreshHandler(authService)).Methods("POST")
	router.HandleFunc("/api/auth/logout", LogoutHandler(authService)).Methods("POST")

	// Protected routes
	router.Handle("/api/auth/me", authMiddleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		GetCurrentUser(w, r, authService)
	}))).Methods("GET")
}
