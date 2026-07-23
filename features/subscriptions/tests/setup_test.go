package tests

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	authmw "arkana/features/auth/middlewares"
	authsvc "arkana/features/auth/services"
	"arkana/features/subscriptions/handlers"
	"arkana/features/subscriptions/services"
	"arkana/shared/adminauth"
	"arkana/shared/email"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

const (
	testJWTSecret   = "test-jwt-secret"
	testTokenSecret = "test-token-secret"
	testAdminSecret = "test-admin-secret"
	testFrontendURL = "https://arkana.test"
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
		CREATE TABLE subscribers (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id         INTEGER REFERENCES users(id),
			email           TEXT NOT NULL UNIQUE,
			status          TEXT NOT NULL DEFAULT 'pending',
			created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			confirmed_at    TIMESTAMP,
			unsubscribed_at TIMESTAMP
		);
		CREATE TABLE posts (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			path_identifier TEXT UNIQUE NOT NULL
		);
		CREATE TABLE post_contents (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			post_id    INTEGER NOT NULL,
			lang       TEXT NOT NULL,
			path       TEXT NOT NULL,
			content    TEXT NOT NULL,
			title      TEXT,
			visible    INTEGER NOT NULL DEFAULT 1
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

func insertTestPost(t *testing.T, db *sql.DB, path, title, content string) int {
	t.Helper()
	result, err := db.Exec("INSERT INTO posts (path_identifier) VALUES (?)", path)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	postID := int(id)

	_, err = db.Exec(
		"INSERT INTO post_contents (post_id, lang, path, content, title, visible) VALUES (?, 'en', ?, ?, ?, 1)",
		postID, path+".md", content, title,
	)
	if err != nil {
		t.Fatal(err)
	}

	return postID
}

func generateTestJWT(t *testing.T, userID int, email string) string {
	t.Helper()
	token, err := authsvc.GenerateAccessToken(userID, email, testJWTSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

// fakeSender is an in-memory email.Sender used to assert what would have
// been sent, without making real network calls.
type fakeSender struct {
	mu      sync.Mutex
	sent    []email.Message
	failFor map[string]bool
}

func newFakeSender() *fakeSender {
	return &fakeSender{failFor: map[string]bool{}}
}

func (f *fakeSender) Send(ctx context.Context, msg email.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failFor[msg.To] {
		return errFakeSendFailure
	}
	f.sent = append(f.sent, msg)
	return nil
}

func (f *fakeSender) messages() []email.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]email.Message, len(f.sent))
	copy(out, f.sent)
	return out
}

var errFakeSendFailure = &fakeSendError{}

type fakeSendError struct{}

func (e *fakeSendError) Error() string { return "simulated send failure" }

func setupService(t *testing.T, db *sql.DB, sender email.Sender) *services.SubscriptionService {
	t.Helper()
	return services.NewSubscriptionService(db, sender, testTokenSecret, testFrontendURL)
}

func setupRouter(t *testing.T, db *sql.DB, sender email.Sender) *mux.Router {
	t.Helper()
	router := mux.NewRouter()
	auth := authmw.NewAuthMiddleware(testJWTSecret)
	adminAuth := adminauth.NewAdminAuthMiddleware(testAdminSecret)
	svc := services.NewSubscriptionService(db, sender, testTokenSecret, testFrontendURL)
	handlers.RegisterRoutes(router, svc, auth, adminAuth)
	return router
}
