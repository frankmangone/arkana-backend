package user

import (
	"arkana/features/user/handlers"
	usermodels "arkana/features/user/models"
	"arkana/features/user/services"
	"database/sql"

	"github.com/gorilla/mux"
)

// Initialize sets up the user module and registers its routes
func Initialize(router *mux.Router, db *sql.DB) {
	// Initialize services
	userService := services.NewUserService(db)

	// Initialize handlers
	userHandler := handlers.NewUserHandler(userService)

	// Register routes
	userHandler.RegisterRoutes(router)
}

// Re-export user model for cleaner imports
// This allows importing "arkana/features/user" and using types like user.User
type User = usermodels.User
