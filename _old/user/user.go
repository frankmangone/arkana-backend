package user

import (
	"arkana/features/user/handlers"
	usermodels "arkana/features/user/models"
	"database/sql"

	"github.com/gorilla/mux"
)

// Initialize sets up the user module and registers its routes
func Initialize(router *mux.Router, db *sql.DB) {
	handlers.RegisterRoutes(router, db)
}

// Re-export user model for cleaner imports
// This allows importing "arkana/features/user" and using types like user.User
type User = usermodels.User
