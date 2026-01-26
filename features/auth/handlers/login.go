package handlers

import (
	"arkana/features/auth/models"
	"arkana/features/auth/services"
	"encoding/json"
	"net/http"
)

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// Login handles POST /auth/login
func Login(w http.ResponseWriter, r *http.Request, authService *services.AuthService) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Invalid request format"})
		return
	}

	// Validate request
	if err := models.ValidateRequest(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: err.Error()})
		return
	}

	// Authenticate user
	accessToken, refreshToken, user, err := authService.Login(req.Email, req.Password)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Invalid credentials"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

// LoginHandler returns an http.HandlerFunc that handles user login
func LoginHandler(authService *services.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		Login(w, r, authService)
	}
}
