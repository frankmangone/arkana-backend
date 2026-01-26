package handlers

import (
	"arkana/features/auth/models"
	"arkana/features/auth/services"
	"encoding/json"
	"net/http"
)

// RefreshRequest represents a refresh token request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// RefreshResponse represents a refresh token response
type RefreshResponse struct {
	AccessToken string `json:"access_token"`
}

// Refresh handles POST /auth/refresh
func Refresh(w http.ResponseWriter, r *http.Request, authService *services.AuthService) {
	var req RefreshRequest
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

	// Generate new access token
	accessToken, err := authService.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(RefreshResponse{
		AccessToken: accessToken,
	})
}

// RefreshHandler returns an http.HandlerFunc that handles token refresh
func RefreshHandler(authService *services.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		Refresh(w, r, authService)
	}
}
