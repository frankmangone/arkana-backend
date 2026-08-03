package tests

import (
	"database/sql"
	"testing"
	"time"

	"arkana/config"
	"arkana/features/auth/services"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE users (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			email            TEXT UNIQUE NOT NULL,
			username         TEXT,
			avatar_url       TEXT,
			auth_provider    TEXT NOT NULL,
			provider_user_id TEXT NOT NULL,
			email_verified   INTEGER NOT NULL DEFAULT 0,
			wallet_address   TEXT,
			wallet_system    TEXT,
			created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(auth_provider, provider_user_id)
		);
		CREATE TABLE refresh_tokens (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL,
			token_hash TEXT NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			revoked_at TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

const testJWTSecret = "test-secret-key"

func testConfig() *config.Config {
	return &config.Config{
		JWTSecret:        testJWTSecret,
		JWTAccessExpiry:  15 * time.Minute,
		JWTRefreshExpiry: 168 * time.Hour,
	}
}

func newAuthService(db *sql.DB) *services.AuthService {
	return services.NewAuthService(db, testConfig())
}
