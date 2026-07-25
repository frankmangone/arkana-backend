package tests

import (
	"database/sql"
	"testing"

	"arkana/features/tags/services"

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
		CREATE TABLE tags (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			slug       TEXT UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE tag_translations (
			id     INTEGER PRIMARY KEY AUTOINCREMENT,
			tag_id INTEGER NOT NULL REFERENCES tags(id),
			lang   TEXT NOT NULL,
			name   TEXT NOT NULL,
			UNIQUE (tag_id, lang)
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

const testAdminSecret = "test-admin-secret"

func newTagService(db *sql.DB) *services.TagService {
	return services.NewTagService(db)
}
