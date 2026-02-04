package models

import "time"

type Wallet struct {
	ID        int       `json:"id"`
	Address   string    `json:"address"`
	System    string    `json:"system"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LoginRequest struct {
	Address   string `json:"address" validate:"required"`
	System    string `json:"system" validate:"required,oneof=ethereum"`
	Message   string `json:"message" validate:"required"`
	Signature string `json:"signature" validate:"required"`
}

type LoginResponse struct {
	Token  string `json:"token"`
	Wallet Wallet `json:"wallet"`
}
