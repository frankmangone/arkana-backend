package auth

import (
	"arkana/config"
	"arkana/features/auth/handlers"
	authmodels "arkana/features/auth/models"
	"database/sql"

	"github.com/gorilla/mux"
)

// Initialize sets up the auth module and registers its routes
func Initialize(router *mux.Router, db *sql.DB, cfg *config.Config) {
	handlers.RegisterRoutes(router, db, cfg)
}

// Re-export all types from the auth models package for cleaner imports
// This allows importing "arkana/features/auth" and using types like auth.ErrorResponse

// Re-export request types
type (
	SignupRequest  = authmodels.SignupRequest
	LoginRequest   = authmodels.LoginRequest
	RefreshRequest = authmodels.RefreshRequest
	LogoutRequest  = authmodels.LogoutRequest
)

// Re-export response types
type (
	AuthResponse    = authmodels.AuthResponse
	RefreshResponse = authmodels.RefreshResponse
	MessageResponse = authmodels.MessageResponse
	ErrorResponse   = authmodels.ErrorResponse
)

// Re-export other types
type (
	RefreshToken = authmodels.RefreshToken
)
