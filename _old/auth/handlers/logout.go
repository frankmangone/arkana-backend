package handlers

import (
	"arkana/features/auth/models"
	"arkana/features/auth/services"
	"encoding/json"
	"net/http"
)

// LogoutRequest represents a logout request
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// MessageResponse represents a simple message response
type MessageResponse struct {
	Message string `json:"message"`
}

// Logout handles POST /auth/logout
func Logout(w http.ResponseWriter, r *http.Request, authService *services.AuthService) {
	var req LogoutRequest
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

	// Revoke refresh token
	if err := authService.RevokeRefreshToken(req.RefreshToken); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(MessageResponse{Message: "Logged out successfully"})
}

// LogoutHandler returns an http.HandlerFunc that handles user logout
func LogoutHandler(authService *services.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		Logout(w, r, authService)
	}
}
