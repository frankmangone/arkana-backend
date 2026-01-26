package handlers

import (
	"arkana/features/user/services"

	"github.com/gorilla/mux"
)

// UserHandler handles user HTTP requests
type UserHandler struct {
	service *services.UserService
}

// NewUserHandler creates a new user handler
func NewUserHandler(service *services.UserService) *UserHandler {
	return &UserHandler{service: service}
}

// RegisterRoutes registers user routes to the router
func (h *UserHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/users/{id}", h.GetUser).Methods("GET")
	router.HandleFunc("/api/users", h.CreateUser).Methods("POST")
}
