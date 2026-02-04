package handlers

import (
	"arkana/features/user/services"
	"database/sql"

	"github.com/gorilla/mux"
)

// RegisterRoutes registers user routes to the router
func RegisterRoutes(router *mux.Router, db *sql.DB) {
	userService := services.NewUserService(db)

	router.HandleFunc("/api/users/{id}", GetUserHandler(userService)).Methods("GET")
	router.HandleFunc("/api/users", CreateUserHandler(userService)).Methods("POST")
}
