package queries

import (
	dbpkg "arkana/shared/db"
	"arkana/features/auth/models"
	"database/sql"
	"time"
)

type AuthQueries interface {
	GetByID(id int) (*models.User, error)
	GetUserByProviderID(provider, providerUserID string) (*models.User, error)
	CreateUser(email string, username *string, provider, providerUserID string, avatarURL *string) (int64, error)
	TouchUser(id int) error
	InsertRefreshToken(userID int, tokenHash string, expiresAt time.Time) error
	GetRefreshToken(tokenHash string) (userID int, expiresAt time.Time, revokedAt sql.NullTime, err error)
	RevokeRefreshToken(tokenHash string) (int64, error)
}

type SQLAuthQueries struct {
	db dbpkg.DBTX
}

func NewSQLAuthQueries(db dbpkg.DBTX) *SQLAuthQueries {
	return &SQLAuthQueries{db: db}
}

// GetByID retrieves a user by ID. Returns (nil, nil) — not an error — if no
// such user exists, matching the pre-refactor behavior callers depend on.
func (q *SQLAuthQueries) GetByID(id int) (*models.User, error) {
	user := &models.User{}
	var username, avatarURL, walletAddress, walletSystem sql.NullString
	var updatedAt sql.NullTime

	err := q.db.QueryRow(`
		SELECT id, email, username, avatar_url, auth_provider, provider_user_id,
		       email_verified, wallet_address, wallet_system, created_at, updated_at
		FROM users WHERE id = ?
	`, id).Scan(
		&user.ID, &user.Email, &username, &avatarURL,
		&user.AuthProvider, &user.ProviderUserID, &user.EmailVerified,
		&walletAddress, &walletSystem, &user.CreatedAt, &updatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
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

// GetUserByProviderID retrieves a user by OIDC provider and provider user
// ID. Returns (nil, nil) if no such user exists.
func (q *SQLAuthQueries) GetUserByProviderID(provider, providerUserID string) (*models.User, error) {
	user := &models.User{}
	var username, avatarURL, walletAddress, walletSystem sql.NullString
	var updatedAt sql.NullTime

	err := q.db.QueryRow(`
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
		if err == sql.ErrNoRows {
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

// CreateUser inserts a new user row from OIDC authentication and returns
// its id.
func (q *SQLAuthQueries) CreateUser(email string, username *string, provider, providerUserID string, avatarURL *string) (int64, error) {
	result, err := q.db.Exec(`
		INSERT INTO users (email, username, auth_provider, provider_user_id, avatar_url, email_verified)
		VALUES (?, ?, ?, ?, ?, 1)
	`, email, username, provider, providerUserID, avatarURL)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// TouchUser bumps updated_at on an existing user row.
func (q *SQLAuthQueries) TouchUser(id int) error {
	_, err := q.db.Exec(`UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// InsertRefreshToken stores a new refresh token's hash.
func (q *SQLAuthQueries) InsertRefreshToken(userID int, tokenHash string, expiresAt time.Time) error {
	_, err := q.db.Exec(`
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES (?, ?, ?)
	`, userID, tokenHash, expiresAt)
	return err
}

// GetRefreshToken looks up a refresh token by its hash.
func (q *SQLAuthQueries) GetRefreshToken(tokenHash string) (userID int, expiresAt time.Time, revokedAt sql.NullTime, err error) {
	err = q.db.QueryRow(`
		SELECT user_id, expires_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = ?
	`, tokenHash).Scan(&userID, &expiresAt, &revokedAt)
	return
}

// RevokeRefreshToken marks a refresh token revoked, returning how many rows
// the UPDATE affected.
func (q *SQLAuthQueries) RevokeRefreshToken(tokenHash string) (int64, error) {
	result, err := q.db.Exec(`
		UPDATE refresh_tokens
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE token_hash = ? AND revoked_at IS NULL
	`, tokenHash)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
