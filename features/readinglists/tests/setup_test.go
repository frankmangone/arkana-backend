package tests

import (
	"database/sql"
	"testing"

	"arkana/features/readinglists/services"

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
		CREATE TABLE reading_lists (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			slug        TEXT UNIQUE NOT NULL,
			cover_image TEXT,
			ongoing     BOOLEAN NOT NULL DEFAULT 0,
			created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE reading_list_translations (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			reading_list_id INTEGER NOT NULL REFERENCES reading_lists(id),
			lang            TEXT NOT NULL,
			title           TEXT NOT NULL,
			description     TEXT NOT NULL,
			UNIQUE (reading_list_id, lang)
		);
		CREATE TABLE reading_list_modules (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			reading_list_id INTEGER NOT NULL REFERENCES reading_lists(id),
			slug            TEXT NOT NULL,
			position        INTEGER NOT NULL,
			UNIQUE (reading_list_id, slug)
		);
		CREATE TABLE reading_list_module_translations (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			module_id   INTEGER NOT NULL REFERENCES reading_list_modules(id),
			lang        TEXT NOT NULL,
			title       TEXT NOT NULL,
			description TEXT NOT NULL,
			UNIQUE (module_id, lang)
		);
		CREATE TABLE reading_list_items (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			module_id INTEGER NOT NULL REFERENCES reading_list_modules(id),
			slug      TEXT NOT NULL,
			post_path TEXT NOT NULL,
			position  INTEGER NOT NULL,
			UNIQUE (module_id, slug)
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

const testAdminSecret = "test-admin-secret"

// fakePostChecker reports a path as missing only if it's explicitly listed
// in missing, so tests that don't exercise path validation don't need to
// stand up a real posts table — every path they use is treated as already
// registered by default (missing is nil).
type fakePostChecker struct {
	missing []string
}

func (f *fakePostChecker) MissingPaths(paths []string) ([]string, error) {
	return f.missing, nil
}

func newReadingListService(db *sql.DB) *services.ReadingListService {
	return services.NewReadingListService(db, &fakePostChecker{})
}
