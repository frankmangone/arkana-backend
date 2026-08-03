package services

import (
	"arkana/config"
	"arkana/features/auth/models"
	"arkana/features/auth/queries"
	"database/sql"
	"errors"
	"log"
	"time"
)

// AuthService handles authentication business logic
type AuthService struct {
	db      *sql.DB
	cfg     *config.Config
	queries queries.AuthQueries
}

// NewAuthService creates a new auth service
func NewAuthService(db *sql.DB, cfg *config.Config) *AuthService {
	return &AuthService{db: db, cfg: cfg, queries: queries.NewSQLAuthQueries(db)}
}

// GetByID retrieves a user by ID
func (s *AuthService) GetByID(id int) (*models.User, error) {
	return s.queries.GetByID(id)
}

// GetUserByProviderID retrieves a user by OIDC provider and provider user ID
func (s *AuthService) GetUserByProviderID(provider, providerUserID string) (*models.User, error) {
	return s.queries.GetUserByProviderID(provider, providerUserID)
}

// CreateOIDCUser creates a new user from OIDC authentication
func (s *AuthService) CreateOIDCUser(email, username, provider, providerUserID, avatarURL string) (*models.User, error) {
	var avatarPtr *string
	if avatarURL != "" {
		avatarPtr = &avatarURL
	}
	var usernamePtr *string
	if username != "" {
		usernamePtr = &username
	}

	id, err := s.queries.CreateUser(email, usernamePtr, provider, providerUserID, avatarPtr)
	if err != nil {
		return nil, err
	}

	return s.GetByID(int(id))
}

// FindOrCreateGoogleUser finds an existing user by Google sub or creates a new one
func (s *AuthService) FindOrCreateGoogleUser(googleUserInfo *GoogleUserInfo) (*models.User, error) {
	log.Printf("[AuthService] FindOrCreateGoogleUser: email=%s, sub=%s", googleUserInfo.Email, googleUserInfo.Sub)

	user, err := s.GetUserByProviderID("google", googleUserInfo.Sub)
	if err != nil {
		return nil, err
	}

	if user != nil {
		log.Printf("[AuthService] Found existing user ID=%d", user.ID)
		if err := s.queries.TouchUser(user.ID); err != nil {
			return nil, err
		}
		return user, nil
	}

	log.Printf("[AuthService] Creating new user for email=%s", googleUserInfo.Email)

	username := googleUserInfo.Email
	if googleUserInfo.GivenName != "" {
		username = googleUserInfo.GivenName
	}

	user, err = s.CreateOIDCUser(
		googleUserInfo.Email,
		username,
		"google",
		googleUserInfo.Sub,
		googleUserInfo.Picture,
	)
	if err != nil {
		log.Printf("[AuthService] Error creating user: %v", err)
		return nil, err
	}

	log.Printf("[AuthService] User created ID=%d", user.ID)
	return user, nil
}

// GenerateTokensForUser generates access and refresh tokens for a user
func (s *AuthService) GenerateTokensForUser(user *models.User) (accessToken, refreshToken string, err error) {
	accessToken, err = GenerateAccessToken(user.ID, user.Email, s.cfg.JWTSecret, s.cfg.JWTAccessExpiry)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}

	tokenHash := HashToken(refreshToken)
	expiresAt := time.Now().Add(s.cfg.JWTRefreshExpiry)
	if err := s.queries.InsertRefreshToken(user.ID, tokenHash, expiresAt); err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// RefreshAccessToken generates a new access token using a valid refresh token
func (s *AuthService) RefreshAccessToken(refreshToken string) (string, error) {
	tokenHash := HashToken(refreshToken)

	userID, expiresAt, revokedAt, err := s.queries.GetRefreshToken(tokenHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("invalid refresh token")
		}
		return "", err
	}

	if revokedAt.Valid {
		return "", errors.New("refresh token has been revoked")
	}

	if time.Now().After(expiresAt) {
		return "", errors.New("refresh token has expired")
	}

	user, err := s.GetByID(userID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", errors.New("user not found")
	}

	return GenerateAccessToken(user.ID, user.Email, s.cfg.JWTSecret, s.cfg.JWTAccessExpiry)
}

// RevokeRefreshToken revokes a refresh token (logout)
func (s *AuthService) RevokeRefreshToken(refreshToken string) error {
	tokenHash := HashToken(refreshToken)

	rowsAffected, err := s.queries.RevokeRefreshToken(tokenHash)
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("refresh token not found or already revoked")
	}

	return nil
}
