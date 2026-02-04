package handlers

import (
	"arkana/features/auth/models"
	"arkana/features/auth/services"
	"encoding/json"
	"log"
	"net/http"
)

// GoogleTokenRequest represents a Google OAuth token exchange request
type GoogleTokenRequest struct {
	Code        string `json:"code" validate:"required"`
	RedirectURI string `json:"redirect_uri" validate:"required"`
}

// GoogleToken handles POST /auth/google/token
func GoogleToken(w http.ResponseWriter, r *http.Request, authService *services.AuthService, googleOAuthService *services.GoogleOAuthService) {
	// OPTIONS requests are handled by CORS middleware, but we need to allow them here too
	if r.Method == "OPTIONS" {
		return
	}

	log.Printf("[Google OAuth] Received token exchange request")

	var req GoogleTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[Google OAuth] Error decoding request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Invalid request format"})
		return
	}

	log.Printf("[Google OAuth] Request decoded, code length: %d", len(req.Code))

	// Validate request
	if err := models.ValidateRequest(req); err != nil {
		log.Printf("[Google OAuth] Validation error: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: err.Error()})
		return
	}

	ctx := r.Context()

	// Step 1: Exchange code for tokens
	log.Printf("[Google OAuth] Exchanging code for tokens with redirect_uri: %s", req.RedirectURI)
	tokens, err := googleOAuthService.ExchangeCodeForTokens(ctx, req.Code, req.RedirectURI)
	if err != nil {
		log.Printf("[Google OAuth] Failed to exchange code: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Failed to exchange authorization code"})
		return
	}
	log.Printf("[Google OAuth] Code exchanged successfully, ID token received")

	// Step 2: Verify ID token and extract user info
	log.Printf("[Google OAuth] Verifying ID token...")
	googleUserInfo, err := googleOAuthService.VerifyIDToken(ctx, tokens.IDToken)
	if err != nil {
		log.Printf("[Google OAuth] Failed to verify ID token: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Failed to verify ID token"})
		return
	}
	log.Printf("[Google OAuth] ID token verified, user email: %s, sub: %s", googleUserInfo.Email, googleUserInfo.Sub)

	// Step 3: Find or create user
	log.Printf("[Google OAuth] Finding or creating user...")
	user, err := authService.FindOrCreateGoogleUser(googleUserInfo)
	if err != nil {
		log.Printf("[Google OAuth] Error finding/creating user: %v", err)
		// Check if it's a duplicate email error
		if err.Error() == "email already registered with a different account" {
			log.Printf("[Google OAuth] Email conflict: %s", googleUserInfo.Email)
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(models.ErrorResponse{
				Error: "This email is already registered. Please use your original login method.",
			})
			return
		}
		// Other errors (database issues, etc.)
		log.Printf("[Google OAuth] Internal error creating user: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Failed to create or retrieve user"})
		return
	}
	log.Printf("[Google OAuth] User found/created successfully, user ID: %d", user.ID)

	// Step 4: Generate our own tokens
	log.Printf("[Google OAuth] Generating tokens for user...")
	accessToken, refreshToken, err := authService.GenerateTokensForUser(user)
	if err != nil {
		log.Printf("[Google OAuth] Failed to generate tokens: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Failed to generate tokens"})
		return
	}
	log.Printf("[Google OAuth] Tokens generated successfully")

	// Step 5: Return tokens and user info
	log.Printf("[Google OAuth] Returning success response for user: %s", user.Email)
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
