package queries

import (
	dbpkg "arkana/shared/db"
	"arkana/features/wallet/models"
	"strings"
)

type WalletQueries interface {
	InsertWallet(address, system string) (int64, error)
	GetByAddress(address string) (*models.Wallet, error)
	GetByID(id int) (*models.Wallet, error)
}

type SQLWalletQueries struct {
	db dbpkg.DBTX
}

func NewSQLWalletQueries(db dbpkg.DBTX) *SQLWalletQueries {
	return &SQLWalletQueries{db: db}
}

// InsertWallet creates a new wallet row and returns its id.
func (q *SQLWalletQueries) InsertWallet(address, system string) (int64, error) {
	result, err := q.db.Exec(
		"INSERT INTO wallets (address, system) VALUES (?, ?)",
		address, system,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetByAddress finds a wallet by its address.
func (q *SQLWalletQueries) GetByAddress(address string) (*models.Wallet, error) {
	address = strings.ToLower(address)
	var w models.Wallet
	err := q.db.QueryRow(
		"SELECT id, address, system, created_at, updated_at FROM wallets WHERE address = ?",
		address,
	).Scan(&w.ID, &w.Address, &w.System, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// GetByID finds a wallet by its ID.
func (q *SQLWalletQueries) GetByID(id int) (*models.Wallet, error) {
	var w models.Wallet
	err := q.db.QueryRow(
		"SELECT id, address, system, created_at, updated_at FROM wallets WHERE id = ?",
		id,
	).Scan(&w.ID, &w.Address, &w.System, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}
