package models

import "time"

// User represents a user in the system
type User struct {
	ID             int        `db:"id"              json:"id"`
	Email          string     `db:"email"           json:"email"`
	Username       *string    `db:"username"        json:"username,omitempty"`
	AvatarURL      *string    `db:"avatar_url"      json:"avatar_url,omitempty"`
	AuthProvider   string     `db:"auth_provider"   json:"auth_provider"`
	ProviderUserID string     `db:"provider_user_id" json:"provider_user_id"`
	EmailVerified  bool       `db:"email_verified"  json:"email_verified"`
	WalletAddress  *string    `db:"wallet_address"  json:"wallet_address,omitempty"`
	WalletSystem   *string    `db:"wallet_system"   json:"wallet_system,omitempty"`
	CreatedAt      time.Time  `db:"created_at"      json:"created_at"`
	UpdatedAt      *time.Time `db:"updated_at"      json:"updated_at,omitempty"`
}
