package handlers

import (
	"arkana/features/auth"
	"arkana/features/user/services"
	"encoding/json"
	"net/http"
)

// CreateUserRequest represents a user creation request
type CreateUserRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// CreateUser handles POST /api/users
func CreateUser(w http.ResponseWriter, r *http.Request, service *services.UserService) {
	var req CreateUserRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(auth.ErrorResponse{Error: "Invalid request"})
		return
	}

	if req.Email == "" || req.Username == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(auth.ErrorResponse{Error: "Email, username, and password are required"})
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
