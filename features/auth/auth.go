package auth

import (
	"arkana/config"
	"arkana/features/auth/handlers"
	"arkana/features/auth/middlewares"
	"database/sql"

	"github.com/gorilla/mux"
)

// Initialize sets up the auth module and returns the auth middleware for use by other modules
func Initialize(router *mux.Router, db *sql.DB, cfg *config.Config) *middlewares.AuthMiddleware {
	return handlers.RegisterRoutes(router, db, cfg)
}
