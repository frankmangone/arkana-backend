package models

import "time"

// RefreshToken represents a refresh token in the system
type RefreshToken struct {
	ID         int        `db:"id" json:"id"`
	UserID     int        `db:"user_id" json:"user_id"`
	TokenHash  string     `db:"token_hash" json:"-"` // Never expose token hash
	ExpiresAt  time.Time  `db:"expires_at" json:"expires_at"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	RevokedAt  *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`
	DeviceInfo string     `db:"device_info" json:"device_info,omitempty"`
}
