package services

import (
	"arkana/features/wallet/models"
	"arkana/features/wallet/queries"
	"database/sql"
	"strings"
)

type WalletService struct {
	db      *sql.DB
	queries queries.WalletQueries
}

func NewWalletService(db *sql.DB) *WalletService {
	return &WalletService{db: db, queries: queries.NewSQLWalletQueries(db)}
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

	id, err := s.queries.InsertWallet(address, system)
	if err != nil {
		return nil, err
	}

	return s.GetByID(int(id))
}

// GetByAddress finds a wallet by its address.
func (s *WalletService) GetByAddress(address string) (*models.Wallet, error) {
	return s.queries.GetByAddress(address)
}

// GetByID finds a wallet by its ID.
func (s *WalletService) GetByID(id int) (*models.Wallet, error) {
	return s.queries.GetByID(id)
}
