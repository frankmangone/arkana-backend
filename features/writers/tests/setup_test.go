package tests

import (
	"database/sql"
	"testing"

	"arkana/features/writers/handlers"
	"arkana/features/writers/services"
	"arkana/shared/adminauth"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

const testAdminSecret = "test-admin-secret"

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE writers (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			slug           TEXT UNIQUE,
			name           TEXT NOT NULL,
			user_id        INTEGER,
			image_url      TEXT,
			avatar_url     TEXT,
			visible        BOOLEAN NOT NULL DEFAULT 1,
			organization   TEXT,
			bio            TEXT,
			social         TEXT,
			wallet_address TEXT,
			created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func setupRouter(t *testing.T, db *sql.DB) *mux.Router {
	t.Helper()
	router := mux.NewRouter()
	adminAuth := adminauth.NewAdminAuthMiddleware(testAdminSecret)
	adminSvc := services.NewAdminWriterService(db)
	writerSvc := services.NewWriterService(db)
	handlers.RegisterRoutes(router, adminSvc, writerSvc, adminAuth)
	return router
}

func insertTestWriter(t *testing.T, db *sql.DB, slug, name string, visible bool) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO writers (slug, name, image_url, avatar_url, visible, organization, bio, social, wallet_address)
		 VALUES (?, ?, '/img.png', '/avatar.png', ?, ?, ?, ?, ?)`,
		slug, name, visible,
		`{"name":"SpaceDev","url":"https://spacedev.io"}`,
		`{"en":"An English bio."}`,
		`{"twitter":"https://x.com/example"}`,
		"0xWALLET",
	)
	if err != nil {
		t.Fatal(err)
	}
}
