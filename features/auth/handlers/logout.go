package handlers

import (
	"arkana/features/auth/services"
	"arkana/shared/httputil"
	"encoding/json"
	"net/http"
)

// LogoutRequest represents a logout request
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LogoutHandler handles POST /api/auth/logout
func LogoutHandler(authService *services.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LogoutRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "Invalid request format")
			return
		}

		if err := httputil.ValidateRequest(req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := authService.RevokeRefreshToken(req.RefreshToken); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
	}
}
