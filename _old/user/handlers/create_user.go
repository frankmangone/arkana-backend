package handlers

import (
	"arkana/features/auth"
	authmodels "arkana/features/auth/models"
	"arkana/features/user/services"
	"encoding/json"
	"net/http"
)

// CreateUserRequest represents a user creation request
type CreateUserRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Username string `json:"username" validate:"required,min=3,max=30"`
	Password string `json:"password" validate:"required,min=8"`
}

// CreateUser handles POST /users
func CreateUser(w http.ResponseWriter, r *http.Request, service *services.UserService) {
	var req CreateUserRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(auth.ErrorResponse{Error: "Invalid request format"})
		return
	}

	// Validate request
	if err := authmodels.ValidateRequest(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(auth.ErrorResponse{Error: err.Error()})
		return
	}

	user, err := service.Create(req.Email, req.Username, req.Password)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(auth.ErrorResponse{Error: "Failed to create user"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// CreateUserHandler returns an http.HandlerFunc that handles user creation
func CreateUserHandler(service *services.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		CreateUser(w, r, service)
	}
}
