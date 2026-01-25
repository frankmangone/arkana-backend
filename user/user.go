package user

import (
	"arkana/user/handlers"
	"arkana/user/services"
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
