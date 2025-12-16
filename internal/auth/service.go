package auth

import (
	"arkana/config"
	"arkana/models"
	"database/sql"
	"errors"
	"time"
)

// Service handles authentication business logic
type Service struct {
	db  *sql.DB
	cfg *config.Config
}

// NewService creates a new auth service
func NewService(db *sql.DB, cfg *config.Config) *Service {
	return &Service{db: db, cfg: cfg}
}

// Register creates a new user with email/password authentication
func (s *Service) Register(email, username, password string) (*models.User, error) {
	// Hash the password
	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	// Create user with email auth provider
	// Note: password column is set to empty string temporarily (will be removed in future migration)
	result, err := s.db.Exec(`
		INSERT INTO users (email, username, password, password_hash, auth_provider, email_verified)
		VALUES (?, ?, '', ?, 'email', 0)
	`, email, username, passwordHash)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return s.GetByID(int(id))
}

// Login authenticates a user with email/password and returns tokens
func (s *Service) Login(email, password string) (accessToken, refreshToken string, user *models.User, err error) {
	// Get user by email and auth provider
	user, err = s.GetByEmailAndProvider(email, "email")
	if err != nil {
		return "", "", nil, err
	}
	if user == nil {
		return "", "", nil, errors.New("invalid credentials")
	}

	// Validate password
	if err := ValidatePassword(user.PasswordHash, password); err != nil {
		return "", "", nil, errors.New("invalid credentials")
	}

	// Generate access token
	accessToken, err = GenerateAccessToken(user.ID, user.Email, s.cfg.JWTSecret, s.cfg.JWTAccessExpiry)
	if err != nil {
		return "", "", nil, err
	}

	// Generate refresh token
	refreshToken, err = GenerateRefreshToken()
	if err != nil {
		return "", "", nil, err
	}

	// Store refresh token in database
	tokenHash := HashToken(refreshToken)
	expiresAt := time.Now().Add(s.cfg.JWTRefreshExpiry)
	_, err = s.db.Exec(`
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES (?, ?, ?)
	`, user.ID, tokenHash, expiresAt)
	if err != nil {
		return "", "", nil, err
	}

	return accessToken, refreshToken, user, nil
}

// RefreshAccessToken generates a new access token using a refresh token
func (s *Service) RefreshAccessToken(refreshToken string) (string, error) {
	tokenHash := HashToken(refreshToken)

	// Look up refresh token in database
	var userID int
	var expiresAt time.Time
	var revokedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT user_id, expires_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = ?
	`, tokenHash).Scan(&userID, &expiresAt, &revokedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New("invalid refresh token")
		}
		return "", err
	}

	// Check if token is revoked
	if revokedAt.Valid {
		return "", errors.New("refresh token has been revoked")
	}

	// Check if token is expired
	if time.Now().After(expiresAt) {
		return "", errors.New("refresh token has expired")
	}

	// Get user
	user, err := s.GetByID(userID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", errors.New("user not found")
	}

	// Generate new access token
	return GenerateAccessToken(user.ID, user.Email, s.cfg.JWTSecret, s.cfg.JWTAccessExpiry)
}

// RevokeRefreshToken revokes a refresh token (logout)
func (s *Service) RevokeRefreshToken(refreshToken string) error {
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

// GetByID retrieves a user by ID
func (s *Service) GetByID(id int) (*models.User, error) {
	user := &models.User{}
	err := s.db.QueryRow(`
		SELECT id, email, username, password_hash, auth_provider, provider_user_id,
		       email_verified, avatar_url, created_at, updated_at
		FROM users WHERE id = ?
	`, id).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.AuthProvider,
		&user.ProviderUserID, &user.EmailVerified, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return user, nil
}

// GetByEmailAndProvider retrieves a user by email and auth provider
func (s *Service) GetByEmailAndProvider(email, provider string) (*models.User, error) {
	user := &models.User{}
	err := s.db.QueryRow(`
		SELECT id, email, username, password_hash, auth_provider, provider_user_id,
		       email_verified, avatar_url, created_at, updated_at
		FROM users
		WHERE email = ? AND auth_provider = ?
	`, email, provider).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.AuthProvider,
		&user.ProviderUserID, &user.EmailVerified, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return user, nil
}

// GetUserByProviderID retrieves a user by provider and provider user ID
func (s *Service) GetUserByProviderID(provider, providerUserID string) (*models.User, error) {
	user := &models.User{}
	err := s.db.QueryRow(`
		SELECT id, email, username, password_hash, auth_provider, provider_user_id,
		       email_verified, avatar_url, created_at, updated_at
		FROM users
		WHERE auth_provider = ? AND provider_user_id = ?
	`, provider, providerUserID).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.AuthProvider,
		&user.ProviderUserID, &user.EmailVerified, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return user, nil
}

// CreateOIDCUser creates a new user from OIDC authentication
func (s *Service) CreateOIDCUser(email, username, provider, providerUserID, avatarURL string) (*models.User, error) {
	var avatarURLPtr *string
	if avatarURL != "" {
		avatarURLPtr = &avatarURL
	}

	// Note: password column is set to empty string temporarily (will be removed in future migration)
	result, err := s.db.Exec(`
		INSERT INTO users (email, username, password, auth_provider, provider_user_id, avatar_url, email_verified)
		VALUES (?, ?, '', ?, ?, ?, 1)
	`, email, username, provider, providerUserID, avatarURLPtr)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return s.GetByID(int(id))
}

// GenerateTokensForUser generates access and refresh tokens for a user
func (s *Service) GenerateTokensForUser(user *models.User) (accessToken, refreshToken string, err error) {
	// Generate access token
	accessToken, err = GenerateAccessToken(user.ID, user.Email, s.cfg.JWTSecret, s.cfg.JWTAccessExpiry)
	if err != nil {
		return "", "", err
	}

	// Generate refresh token
	refreshToken, err = GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}

	// Store refresh token in database
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
