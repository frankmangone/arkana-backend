package services

import (
	"arkana/config"
	"arkana/features/auth/models"
	"database/sql"
	"errors"
	"log"
	"time"
)

// AuthService handles authentication business logic
type AuthService struct {
	db  *sql.DB
	cfg *config.Config
}

// NewAuthService creates a new auth service
func NewAuthService(db *sql.DB, cfg *config.Config) *AuthService {
	return &AuthService{db: db, cfg: cfg}
}

// GetByID retrieves a user by ID
func (s *AuthService) GetByID(id int) (*models.User, error) {
	user := &models.User{}
	var username, avatarURL, walletAddress, walletSystem sql.NullString
	var updatedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, email, username, avatar_url, auth_provider, provider_user_id,
		       email_verified, wallet_address, wallet_system, created_at, updated_at
		FROM users WHERE id = ?
	`, id).Scan(
		&user.ID, &user.Email, &username, &avatarURL,
		&user.AuthProvider, &user.ProviderUserID, &user.EmailVerified,
		&walletAddress, &walletSystem, &user.CreatedAt, &updatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if username.Valid {
		user.Username = &username.String
	}
	if avatarURL.Valid {
		user.AvatarURL = &avatarURL.String
	}
	if walletAddress.Valid {
		user.WalletAddress = &walletAddress.String
	}
	if walletSystem.Valid {
		user.WalletSystem = &walletSystem.String
	}
	if updatedAt.Valid {
		user.UpdatedAt = &updatedAt.Time
	}

	return user, nil
}

// GetUserByProviderID retrieves a user by OIDC provider and provider user ID
func (s *AuthService) GetUserByProviderID(provider, providerUserID string) (*models.User, error) {
	user := &models.User{}
	var username, avatarURL, walletAddress, walletSystem sql.NullString
	var updatedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, email, username, avatar_url, auth_provider, provider_user_id,
		       email_verified, wallet_address, wallet_system, created_at, updated_at
		FROM users
		WHERE auth_provider = ? AND provider_user_id = ?
	`, provider, providerUserID).Scan(
		&user.ID, &user.Email, &username, &avatarURL,
		&user.AuthProvider, &user.ProviderUserID, &user.EmailVerified,
		&walletAddress, &walletSystem, &user.CreatedAt, &updatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if username.Valid {
		user.Username = &username.String
	}
	if avatarURL.Valid {
		user.AvatarURL = &avatarURL.String
	}
	if walletAddress.Valid {
		user.WalletAddress = &walletAddress.String
	}
	if walletSystem.Valid {
		user.WalletSystem = &walletSystem.String
	}
	if updatedAt.Valid {
		user.UpdatedAt = &updatedAt.Time
	}

	return user, nil
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

	result, err := s.db.Exec(`
		INSERT INTO users (email, username, auth_provider, provider_user_id, avatar_url, email_verified)
		VALUES (?, ?, ?, ?, ?, 1)
	`, email, usernamePtr, provider, providerUserID, avatarPtr)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
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
		_, err = s.db.Exec(`UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, user.ID)
		if err != nil {
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
	_, err = s.db.Exec(`
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES (?, ?, ?)
	`, user.ID, tokenHash, expiresAt)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// RefreshAccessToken generates a new access token using a valid refresh token
func (s *AuthService) RefreshAccessToken(refreshToken string) (string, error) {
	tokenHash := HashToken(refreshToken)

	var userID int
	var expiresAt time.Time
	var revokedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT user_id, expires_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = ?
	`, tokenHash).Scan(&userID, &expiresAt, &revokedAt)

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

	result, err := s.db.Exec(`
		UPDATE refresh_tokens
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE token_hash = ? AND revoked_at IS NULL
	`, tokenHash)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("refresh token not found or already revoked")
	}

	return nil
}
