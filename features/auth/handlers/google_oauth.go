package handlers

import (
	"arkana/features/auth/models"
	"arkana/features/auth/services"
	"arkana/shared/httputil"
	"encoding/json"
	"log"
	"net/http"
)

// GoogleTokenRequest represents a Google OAuth token exchange request
type GoogleTokenRequest struct {
	Code        string `json:"code"         validate:"required"`
	RedirectURI string `json:"redirect_uri" validate:"required"`
}

// GoogleTokenHandler handles POST /api/auth/google/token
func GoogleTokenHandler(authService *services.AuthService, googleOAuthService *services.GoogleOAuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[Google OAuth] Received token exchange request")

		var req GoogleTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[Google OAuth] Error decoding request: %v", err)
			httputil.WriteError(w, http.StatusBadRequest, "Invalid request format")
			return
		}

		if err := httputil.ValidateRequest(req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		ctx := r.Context()

		tokens, err := googleOAuthService.ExchangeCodeForTokens(ctx, req.Code, req.RedirectURI)
		if err != nil {
			log.Printf("[Google OAuth] Failed to exchange code: %v", err)
			httputil.WriteError(w, http.StatusUnauthorized, "Failed to exchange authorization code")
			return
		}

		googleUserInfo, err := googleOAuthService.VerifyIDToken(ctx, tokens.IDToken)
		if err != nil {
			log.Printf("[Google OAuth] Failed to verify ID token: %v", err)
			httputil.WriteError(w, http.StatusUnauthorized, "Failed to verify ID token")
			return
		}

		user, err := authService.FindOrCreateGoogleUser(googleUserInfo)
		if err != nil {
			log.Printf("[Google OAuth] Error finding/creating user: %v", err)
			httputil.WriteError(w, http.StatusInternalServerError, "Failed to create or retrieve user")
			return
		}

		accessToken, refreshToken, err := authService.GenerateTokensForUser(user)
		if err != nil {
			log.Printf("[Google OAuth] Failed to generate tokens: %v", err)
			httputil.WriteError(w, http.StatusInternalServerError, "Failed to generate tokens")
			return
		}

		log.Printf("[Google OAuth] Success for user: %s", user.Email)

		httputil.WriteJSON(w, http.StatusOK, models.AuthResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			User:         user,
		})
	}
}
