package services

import (
	"arkana/config"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// GoogleOAuthService handles Google OAuth token exchange and verification
type GoogleOAuthService struct {
	config   *oauth2.Config
	verifier *oidc.IDTokenVerifier
	clientID string
}

// GoogleTokenResponse represents the response from Google's token endpoint
type GoogleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	IDToken      string `json:"id_token"`
}

// GoogleUserInfo represents user information from Google ID token
type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Locale        string `json:"locale"`
}

// NewGoogleOAuthService creates a new Google OAuth service
func NewGoogleOAuthService(cfg *config.Config) (*GoogleOAuthService, error) {
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" || cfg.GoogleRedirectURL == "" {
		return nil, errors.New("Google OAuth configuration is incomplete")
	}

	oauthConfig := &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}

	// Create OIDC verifier
	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: cfg.GoogleClientID,
	})

	return &GoogleOAuthService{
		config:   oauthConfig,
		verifier: verifier,
		clientID: cfg.GoogleClientID,
	}, nil
}

// ExchangeCodeForTokens exchanges an authorization code for tokens
func (s *GoogleOAuthService) ExchangeCodeForTokens(ctx context.Context, code string) (*GoogleTokenResponse, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", s.config.ClientID)
	data.Set("client_secret", s.config.ClientSecret)
	data.Set("redirect_uri", s.config.RedirectURL)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, "POST", s.config.Endpoint.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp GoogleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

// VerifyIDToken verifies and extracts user information from Google ID token
func (s *GoogleOAuthService) VerifyIDToken(ctx context.Context, idToken string) (*GoogleUserInfo, error) {
	// Verify the token
	token, err := s.verifier.Verify(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	// Extract claims
	var claims struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Locale        string `json:"locale"`
	}

	if err := token.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims: %w", err)
	}

	return &GoogleUserInfo{
		Sub:           claims.Sub,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
		Picture:       claims.Picture,
		GivenName:     claims.GivenName,
		FamilyName:    claims.FamilyName,
		Locale:        claims.Locale,
	}, nil
}
