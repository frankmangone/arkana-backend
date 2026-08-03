package handlers

import (
	"arkana/features/auth/services"
	"arkana/shared/httputil"
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

// RefreshHandler handles POST /api/auth/refresh
func RefreshHandler(authService *services.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "Invalid request format")
			return
		}

		if err := httputil.ValidateRequest(req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		accessToken, err := authService.RefreshAccessToken(req.RefreshToken)
		if err != nil {
			httputil.WriteError(w, http.StatusUnauthorized, err.Error())
			return
		}

		httputil.WriteJSON(w, http.StatusOK, RefreshResponse{AccessToken: accessToken})
	}
}
