package tests

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"arkana/features/posts/handlers"
	"arkana/features/posts/services"
	walletmw "arkana/features/wallet/middlewares"
	walletsvc "arkana/features/wallet/services"

	"github.com/gorilla/mux"
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
		CREATE TABLE wallets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			address TEXT UNIQUE NOT NULL,
			system TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path_identifier TEXT UNIQUE NOT NULL,
			like_count INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE post_likes (
			post_id INTEGER NOT NULL,
			wallet_id INTEGER NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (post_id, wallet_id),
			FOREIGN KEY (post_id) REFERENCES posts(id),
			FOREIGN KEY (wallet_id) REFERENCES wallets(id)
		);
		CREATE TABLE comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			post_id INTEGER NOT NULL,
			wallet_id INTEGER NOT NULL,
			parent_id INTEGER,
			body TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (post_id) REFERENCES posts(id),
			FOREIGN KEY (wallet_id) REFERENCES wallets(id),
			FOREIGN KEY (parent_id) REFERENCES comments(id)
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func insertTestWallet(t *testing.T, db *sql.DB, address string) int {
	t.Helper()
	result, err := db.Exec(
		"INSERT INTO wallets (address, system) VALUES (?, 'ethereum')", address,
	)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	return int(id)
}

func setupRouter(t *testing.T, db *sql.DB) (*mux.Router, *walletsvc.TokenService) {
	t.Helper()
	router := mux.NewRouter()
	tokenService := walletsvc.NewTokenService(
		"test-secret-must-be-at-least-32-chars!", 24*time.Hour,
	)
	auth := walletmw.NewAuthMiddleware(tokenService)
	ps := services.NewPostService(db)
	cs := services.NewCommentService(db)
	handlers.RegisterRoutes(router, ps, cs, auth)
	return router, tokenService
}

func authedRequest(method, path, token string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return req
}
