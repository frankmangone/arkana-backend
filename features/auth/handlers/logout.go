package handlers

import (
	"arkana/features/auth/models"
	"arkana/features/auth/services"
	"encoding/json"
	"net/http"
)

// Logout handles POST /api/auth/logout
func Logout(w http.ResponseWriter, r *http.Request, authService *services.AuthService) {
	var req models.LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Invalid request format"})
		return
	}

	if req.RefreshToken == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Refresh token is required"})
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
	json.NewEncoder(w).Encode(models.MessageResponse{Message: "Logged out successfully"})
}

// LogoutHandler returns an http.HandlerFunc that handles user logout
func LogoutHandler(authService *services.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		Logout(w, r, authService)
	}
}
