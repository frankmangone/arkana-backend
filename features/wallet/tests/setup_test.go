package tests

import (
	"database/sql"
	"testing"

	"arkana/features/wallet/services"

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
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			address    TEXT UNIQUE NOT NULL,
			system     TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX idx_wallets_address ON wallets(address);
	`)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func newWalletService(db *sql.DB) *services.WalletService {
	return services.NewWalletService(db)
}
