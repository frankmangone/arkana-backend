package handlers

import (
	"arkana/features/auth/models"
	"arkana/features/auth/services"
	"encoding/json"
	"net/http"
)

// GoogleTokenRequest represents a Google OAuth token exchange request
type GoogleTokenRequest struct {
	Code string `json:"code" validate:"required"`
}

// GoogleToken handles POST /auth/google/token
func GoogleToken(w http.ResponseWriter, r *http.Request, authService *services.AuthService, googleOAuthService *services.GoogleOAuthService) {
	var req GoogleTokenRequest
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

	ctx := r.Context()

	// Step 1: Exchange code for tokens
	tokens, err := googleOAuthService.ExchangeCodeForTokens(ctx, req.Code)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Failed to exchange authorization code"})
		return
	}

	// Step 2: Verify ID token and extract user info
	googleUserInfo, err := googleOAuthService.VerifyIDToken(ctx, tokens.IDToken)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Failed to verify ID token"})
		return
	}

	// Step 3: Find or create user
	user, err := authService.FindOrCreateGoogleUser(googleUserInfo)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Failed to create or retrieve user"})
		return
	}

	// Step 4: Generate our own tokens
	accessToken, refreshToken, err := authService.GenerateTokensForUser(user)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Failed to generate tokens"})
		return
	}

	// Step 5: Return tokens and user info
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

// GoogleTokenHandler returns an http.HandlerFunc that handles Google OAuth token exchange
func GoogleTokenHandler(authService *services.AuthService, googleOAuthService *services.GoogleOAuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		GoogleToken(w, r, authService, googleOAuthService)
	}
}
