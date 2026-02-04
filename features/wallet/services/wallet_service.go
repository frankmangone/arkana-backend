package services

import (
	"arkana/features/wallet/models"
	"database/sql"
	"strings"
)

type WalletService struct {
	db *sql.DB
}

func NewWalletService(db *sql.DB) *WalletService {
	return &WalletService{db: db}
}

// GetOrCreate finds an existing wallet by address or creates a new one.
func (s *WalletService) GetOrCreate(address, system string) (*models.Wallet, error) {
	address = strings.ToLower(address)

	wallet, err := s.GetByAddress(address)
	if err == nil {
		return wallet, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	result, err := s.db.Exec(
		"INSERT INTO wallets (address, system) VALUES (?, ?)",
		address, system,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return s.GetByID(int(id))
}

// GetByAddress finds a wallet by its address.
func (s *WalletService) GetByAddress(address string) (*models.Wallet, error) {
	address = strings.ToLower(address)
	var w models.Wallet
	err := s.db.QueryRow(
		"SELECT id, address, system, created_at, updated_at FROM wallets WHERE address = ?",
		address,
	).Scan(&w.ID, &w.Address, &w.System, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// GetByID finds a wallet by its ID.
func (s *WalletService) GetByID(id int) (*models.Wallet, error) {
	var w models.Wallet
	err := s.db.QueryRow(
		"SELECT id, address, system, created_at, updated_at FROM wallets WHERE id = ?",
		id,
	).Scan(&w.ID, &w.Address, &w.System, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}
