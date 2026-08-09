package tests

import (
	"database/sql"
	"testing"
	"time"

	authmw "arkana/features/auth/middlewares"
	authsvc "arkana/features/auth/services"
	notifservices "arkana/features/notifications/services"
	"arkana/features/posts/handlers"
	"arkana/features/posts/services"
	"arkana/shared/adminauth"

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
			writer_id INTEGER,
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
		CREATE TABLE writers (
			id      INTEGER PRIMARY KEY AUTOINCREMENT,
			name    TEXT NOT NULL,
			user_id INTEGER
		);
		CREATE TABLE post_contents (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			post_id    INTEGER NOT NULL,
			lang       TEXT NOT NULL,
			path       TEXT NOT NULL,
			content    TEXT NOT NULL,
			title      TEXT,
			thumbnail  TEXT,
			visible    INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (lang, path)
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

func insertTestWriter(t *testing.T, db *sql.DB, name string, userID *int) int {
	t.Helper()
	result, err := db.Exec("INSERT INTO writers (name, user_id) VALUES (?, ?)", name, userID)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	return int(id)
}

func setPostWriter(t *testing.T, db *sql.DB, postID, writerID int) {
	t.Helper()
	if _, err := db.Exec("UPDATE posts SET writer_id = ? WHERE id = ?", writerID, postID); err != nil {
		t.Fatal(err)
	}
}

func insertPostContent(t *testing.T, db *sql.DB, postID int, lang, path, content string, visible bool) {
	t.Helper()
	v := 1
	if !visible {
		v = 0
	}
	_, err := db.Exec(
		"INSERT INTO post_contents (post_id, lang, path, content, visible) VALUES (?, ?, ?, ?, ?)",
		postID, lang, path, content, v,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func countNotifications(t *testing.T, db *sql.DB, recipientUserID int, notifType string) int {
	t.Helper()
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM notifications WHERE recipient_user_id = ? AND type = ?",
		recipientUserID, notifType,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func generateTestJWT(t *testing.T, userID int, email string) string {
	t.Helper()
	token, err := authsvc.GenerateAccessToken(userID, email, testJWTSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

const testAdminSecret = "test-admin-secret"

func setupRouter(t *testing.T, db *sql.DB) *mux.Router {
	t.Helper()
	return setupRouterWithTagChecker(t, db, &fakeTagChecker{})
}

// setupRouterWithTagChecker lets a test configure specific missing tags at
// the HTTP layer, without changing setupRouter's signature for the many
// existing callers that don't care about tag validation.
func setupRouterWithTagChecker(t *testing.T, db *sql.DB, tagChecker services.TagChecker) *mux.Router {
	t.Helper()
	router := mux.NewRouter()
	auth := authmw.NewAuthMiddleware(testJWTSecret)
	adminAuth := adminauth.NewAdminAuthMiddleware(testAdminSecret)
	notifSvc := notifservices.NewNotificationService(db)
	ps := services.NewPostService(db, notifSvc)
	cs := services.NewCommentService(db, notifSvc)
	adminSvc := services.NewAdminPostService(db, ps, &fakeIndexer{}, tagChecker, newFakeWriterResolver())
	handlers.RegisterRoutes(router, ps, cs, adminSvc, auth, adminAuth)
	return router
}
