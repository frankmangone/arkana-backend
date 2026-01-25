package handlers

import (
	"arkana/auth/middlewares"
	"arkana/auth/services"
	"net/http"

	"github.com/gorilla/mux"
)

// AuthHandler handles auth HTTP requests
type AuthHandler struct {
	authService *services.AuthService
	middleware  *middlewares.AuthMiddleware
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService *services.AuthService, middleware *middlewares.AuthMiddleware) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		middleware:  middleware,
	}
}

// RegisterRoutes registers auth routes to the router
func (h *AuthHandler) RegisterRoutes(router *mux.Router) {
	// Public routes
	router.HandleFunc("/api/auth/signup", h.Signup).Methods("POST")
	router.HandleFunc("/api/auth/login", h.Login).Methods("POST")
	router.HandleFunc("/api/auth/refresh", h.Refresh).Methods("POST")
	router.HandleFunc("/api/auth/logout", h.Logout).Methods("POST")

	// Protected routes
	router.Handle("/api/auth/me", h.middleware.RequireAuth(http.HandlerFunc(h.GetCurrentUser))).Methods("GET")
}
