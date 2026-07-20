package tests

import (
	"database/sql"
	"testing"
	"time"

	authmw "arkana/features/auth/middlewares"
	authsvc "arkana/features/auth/services"
	"arkana/features/posts/handlers"
	"arkana/features/posts/services"

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
			avatar_url       TEXT,
			auth_provider    TEXT NOT NULL CHECK(auth_provider IN ('google', 'github', 'apple', 'discord')),
			provider_user_id TEXT NOT NULL,
			email_verified   INTEGER NOT NULL DEFAULT 0,
			wallet_address   TEXT,
			wallet_system    TEXT,
			created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(auth_provider, provider_user_id)
		);
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path_identifier TEXT UNIQUE NOT NULL,
			like_count INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE post_likes (
			post_id  INTEGER NOT NULL,
			user_id  INTEGER NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (post_id, user_id),
			FOREIGN KEY (post_id) REFERENCES posts(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
		CREATE TABLE post_reads (
			post_id  INTEGER NOT NULL,
			user_id  INTEGER NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (post_id, user_id),
			FOREIGN KEY (post_id) REFERENCES posts(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
		CREATE TABLE comments (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			post_id   INTEGER NOT NULL,
			user_id   INTEGER NOT NULL,
			parent_id INTEGER,
			body      TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (post_id)   REFERENCES posts(id),
			FOREIGN KEY (user_id)   REFERENCES users(id),
			FOREIGN KEY (parent_id) REFERENCES comments(id)
		);
		CREATE INDEX idx_comments_post ON comments(post_id);
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

func insertTestPost(t *testing.T, db *sql.DB, path string) int {
	t.Helper()
	result, err := db.Exec(
		"INSERT INTO posts (path_identifier, like_count) VALUES (?, 0)", path,
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
	ps := services.NewPostService(db)
	cs := services.NewCommentService(db)
	handlers.RegisterRoutes(router, ps, cs, auth)
	return router
}
