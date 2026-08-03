package db

import (
	"database/sql"
	"errors"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTransactTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	if _, err := sqlDB.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}

	return sqlDB
}

func TestTransactCommitsOnSuccess(t *testing.T) {
	sqlDB := setupTransactTestDB(t)

	err := Transact(sqlDB, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO items (name) VALUES (?)", "widget")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	var count int
	if err := sqlDB.QueryRow("SELECT COUNT(*) FROM items").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1 (the insert should have been committed)", count)
	}
}

func TestTransactRollsBackOnError(t *testing.T) {
	sqlDB := setupTransactTestDB(t)

	wantErr := errors.New("boom")
	err := Transact(sqlDB, func(tx *sql.Tx) error {
		if _, err := tx.Exec("INSERT INTO items (name) VALUES (?)", "widget"); err != nil {
			return err
		}
		return wantErr
	})
	if err != wantErr {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}

	var count int
	if err := sqlDB.QueryRow("SELECT COUNT(*) FROM items").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (the insert should have been rolled back)", count)
	}
}
