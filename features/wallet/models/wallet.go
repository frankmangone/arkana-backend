package models

import "time"

type Wallet struct {
	ID        int       `json:"id"`
	Address   string    `json:"address"`
	System    string    `json:"system"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LoginResponse struct {
	Wallet Wallet `json:"wallet"`
}
