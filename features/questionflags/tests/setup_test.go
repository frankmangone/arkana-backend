package tests

import (
	"database/sql"
	"testing"
	"time"

	authmw "arkana/features/auth/middlewares"
	authsvc "arkana/features/auth/services"
	"arkana/features/questionflags/handlers"
	"arkana/features/questionflags/services"
	"arkana/shared/adminauth"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

const (
	testJWTSecret   = "test-secret-key"
	testAdminSecret = "test-admin-secret"
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
			auth_provider    TEXT NOT NULL,
			provider_user_id TEXT NOT NULL,
			created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE questions (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid        TEXT UNIQUE NOT NULL,
			slug        TEXT UNIQUE NOT NULL,
			type        TEXT NOT NULL,
			difficulty  INTEGER NOT NULL,
			answer_key  TEXT NOT NULL,
			created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE question_flags (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			question_id INTEGER NOT NULL REFERENCES questions(id),
			user_id     INTEGER NOT NULL REFERENCES users(id),
			reason      TEXT NOT NULL,
			created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (question_id, user_id)
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

func insertTestQuestion(t *testing.T, db *sql.DB, uuid, slug string) int {
	t.Helper()
	result, err := db.Exec(
		"INSERT INTO questions (uuid, slug, type, difficulty, answer_key) VALUES (?, ?, 'single_choice', 1, '{}')",
		uuid, slug,
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
	adminAuth := adminauth.NewAdminAuthMiddleware(testAdminSecret)
	svc := services.NewQuestionFlagService(db)
	handlers.RegisterRoutes(router, svc, auth, adminAuth)
	return router
}
