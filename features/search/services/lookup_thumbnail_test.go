package services

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupThumbnailTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE post_contents (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			post_id   INTEGER NOT NULL,
			lang      TEXT NOT NULL,
			path      TEXT NOT NULL,
			content   TEXT NOT NULL,
			thumbnail TEXT,
			UNIQUE (lang, path)
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

// TestSearchServiceLookupThumbnail exercises lookupThumbnail's DB round
// trip directly - unlike Search()/IndexPost(), which go through Meilisearch
// and are covered elsewhere in this package with an httptest server,
// lookupThumbnail is the only DB-touching path in SearchService, and
// nothing else in this package's existing tests reaches it.
func TestSearchServiceLookupThumbnail(t *testing.T) {
	db := setupThumbnailTestDB(t)
	svc := NewSearchService(db, "http://unused", "unused")

	t.Run("returns the thumbnail for a matching lang/path", func(t *testing.T) {
		if _, err := db.Exec(
			"INSERT INTO post_contents (post_id, lang, path, content, thumbnail) VALUES (1, 'en', 'cryptography-101.md', 'body', 'https://example.com/thumb.png')",
		); err != nil {
			t.Fatal(err)
		}

		got := svc.lookupThumbnail("en", "cryptography-101")
		if got != "https://example.com/thumb.png" {
			t.Errorf("lookupThumbnail = %q, want %q", got, "https://example.com/thumb.png")
		}
	})

	t.Run("returns empty string when no row matches", func(t *testing.T) {
		got := svc.lookupThumbnail("en", "does-not-exist")
		if got != "" {
			t.Errorf("lookupThumbnail = %q, want empty string", got)
		}
	})

	t.Run("returns empty string when thumbnail column is NULL", func(t *testing.T) {
		if _, err := db.Exec(
			"INSERT INTO post_contents (post_id, lang, path, content, thumbnail) VALUES (2, 'es', 'sin-miniatura.md', 'body', NULL)",
		); err != nil {
			t.Fatal(err)
		}

		got := svc.lookupThumbnail("es", "sin-miniatura")
		if got != "" {
			t.Errorf("lookupThumbnail = %q, want empty string for a NULL thumbnail", got)
		}
	})
}
