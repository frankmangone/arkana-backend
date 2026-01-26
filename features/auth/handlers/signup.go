package handlers

import (
	"arkana/features/auth/models"
	"arkana/features/auth/services"
	"encoding/json"
	"net/http"
)

// SignupRequest represents a signup request
type SignupRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Username string `json:"username" validate:"required,min=3,max=30"`
	Password string `json:"password" validate:"required,min=8"`
}

// Signup handles POST /auth/signup
func Signup(w http.ResponseWriter, r *http.Request, authService *services.AuthService) {
	var req SignupRequest
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

	// Register user
	user, err := authService.Register(req.Email, req.Username, req.Password)
	if err != nil {
		// Check for duplicate email
		if err.Error() == "UNIQUE constraint failed: users.email" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Email already registered"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Failed to create user"})
		return
	}

	// Generate tokens
	accessToken, refreshToken, err := authService.GenerateTokensForUser(user)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Failed to generate tokens"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

// SignupHandler returns an http.HandlerFunc that handles user signup
func SignupHandler(authService *services.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		Signup(w, r, authService)
	}
}
