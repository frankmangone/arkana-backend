package services

import (
	"arkana/config"
	usermodels "arkana/features/user/models"
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

// Register creates a new user with email/password authentication
func (s *AuthService) Register(email, username, password string) (*usermodels.User, error) {
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
func (s *AuthService) Login(email, password string) (accessToken, refreshToken string, user *usermodels.User, err error) {
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
func (s *AuthService) RefreshAccessToken(refreshToken string) (string, error) {
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

// GetByID retrieves a user by ID
func (s *AuthService) GetByID(id int) (*usermodels.User, error) {
	user := &usermodels.User{}
	var passwordHash sql.NullString
	err := s.db.QueryRow(`
		SELECT id, email, username, password_hash, auth_provider, provider_user_id,
		       email_verified, avatar_url, created_at, updated_at
		FROM users WHERE id = ?
	`, id).Scan(
		&user.ID, &user.Email, &user.Username, &passwordHash, &user.AuthProvider,
		&user.ProviderUserID, &user.EmailVerified, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Convert sql.NullString to *string
	if passwordHash.Valid {
		user.PasswordHash = &passwordHash.String
	}

	return user, nil
}

// GetByEmailAndProvider retrieves a user by email and auth provider
func (s *AuthService) GetByEmailAndProvider(email, provider string) (*usermodels.User, error) {
	user := &usermodels.User{}
	var passwordHash sql.NullString
	err := s.db.QueryRow(`
		SELECT id, email, username, password_hash, auth_provider, provider_user_id,
		       email_verified, avatar_url, created_at, updated_at
		FROM users
		WHERE email = ? AND auth_provider = ?
	`, email, provider).Scan(
		&user.ID, &user.Email, &user.Username, &passwordHash, &user.AuthProvider,
		&user.ProviderUserID, &user.EmailVerified, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Convert sql.NullString to *string
	if passwordHash.Valid {
		user.PasswordHash = &passwordHash.String
	}

	return user, nil
}

// GetUserByProviderID retrieves a user by provider and provider user ID
func (s *AuthService) GetUserByProviderID(provider, providerUserID string) (*usermodels.User, error) {
	user := &usermodels.User{}
	var passwordHash sql.NullString
	err := s.db.QueryRow(`
		SELECT id, email, username, password_hash, auth_provider, provider_user_id,
		       email_verified, avatar_url, created_at, updated_at
		FROM users
		WHERE auth_provider = ? AND provider_user_id = ?
	`, provider, providerUserID).Scan(
		&user.ID, &user.Email, &user.Username, &passwordHash, &user.AuthProvider,
		&user.ProviderUserID, &user.EmailVerified, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Convert sql.NullString to *string
	if passwordHash.Valid {
		user.PasswordHash = &passwordHash.String
	}

	return user, nil
}

// CreateOIDCUser creates a new user from OIDC authentication
func (s *AuthService) CreateOIDCUser(email, username, provider, providerUserID, avatarURL string) (*usermodels.User, error) {
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
func (s *AuthService) GenerateTokensForUser(user *usermodels.User) (accessToken, refreshToken string, err error) {
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

// FindOrCreateGoogleUser finds an existing user by Google ID or creates a new one
// Returns an error if the email already exists with a different auth provider
func (s *AuthService) FindOrCreateGoogleUser(googleUserInfo *GoogleUserInfo) (*usermodels.User, error) {
	log.Printf("[AuthService] FindOrCreateGoogleUser: email=%s, sub=%s", googleUserInfo.Email, googleUserInfo.Sub)

	// Try to find existing user by Google ID (idempotent - same Google ID = same account)
	user, err := s.GetUserByProviderID("google", googleUserInfo.Sub)
	if err != nil {
		log.Printf("[AuthService] Error getting user by provider ID: %v", err)
		return nil, err
	}

	if user != nil {
		log.Printf("[AuthService] User found by Google ID, user ID: %d", user.ID)
		// User already exists with this Google ID - update last login time
		_, err = s.db.Exec(`
			UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = ?
		`, user.ID)
		if err != nil {
			log.Printf("[AuthService] Error updating user timestamp: %v", err)
			return nil, err
		}
		return user, nil
	}

	log.Printf("[AuthService] User not found by Google ID, checking for existing email...")

	// Check if email already exists with a different provider
	existingUser, err := s.GetByEmailAndProvider(googleUserInfo.Email, "email")
	if err != nil {
		log.Printf("[AuthService] Error checking email: %v", err)
		return nil, err
	}

	if existingUser != nil {
		log.Printf("[AuthService] Email already exists with different provider (email auth), user ID: %d", existingUser.ID)
		return nil, errors.New("email already registered with a different account")
	}

	log.Printf("[AuthService] Email not found, creating new user...")

	// User doesn't exist with this Google ID - create new user
	// Use email as username if name is not available
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

	// Check if creation failed due to duplicate email
	if err != nil {
		log.Printf("[AuthService] Error creating user: %v", err)
		// SQLite returns "UNIQUE constraint failed: users.email" for duplicate emails
		if err.Error() == "UNIQUE constraint failed: users.email" {
			log.Printf("[AuthService] Duplicate email detected during creation")
			return nil, errors.New("email already registered with a different account")
		}
		return nil, err
	}

	log.Printf("[AuthService] User created successfully, user ID: %d", user.ID)
	return user, nil
}
