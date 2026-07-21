package tests

import (
	"database/sql"
	"testing"
	"time"

	authmw "arkana/features/auth/middlewares"
	authsvc "arkana/features/auth/services"
	"arkana/features/notifications/handlers"
	"arkana/features/notifications/services"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

const testJWTSecret = "test-secret-key"

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
			auth_provider    TEXT NOT NULL,
			provider_user_id TEXT NOT NULL,
			created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE notifications (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			recipient_user_id INTEGER NOT NULL,
			actor_user_id     INTEGER NOT NULL,
			type              TEXT NOT NULL,
			post_id           INTEGER,
			comment_id        INTEGER,
			read_at           TIMESTAMP,
			created_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func insertTestUser(t *testing.T, db *sql.DB, email string) int {
	t.Helper()
	result, err := db.Exec(
		"INSERT INTO users (email, username, auth_provider, provider_user_id) VALUES (?, ?, 'google', ?)",
		email, email, email,
	)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	return int(id)
}

func generateTestJWT(t *testing.T, userID int, email string) string {
	t.Helper()
	token, err := authsvc.GenerateAccessToken(userID, email, testJWTSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func setupRouter(t *testing.T, db *sql.DB) *mux.Router {
	t.Helper()
	router := mux.NewRouter()
	auth := authmw.NewAuthMiddleware(testJWTSecret)
	svc := services.NewNotificationService(db)
	handlers.RegisterRoutes(router, svc, auth)
	return router
}
