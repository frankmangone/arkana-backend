package models

import "time"

// User represents a user in the system
type User struct {
	ID        int       `db:"id" json:"id"`
	Email     string    `db:"email" json:"email"`
	Username  string    `db:"username" json:"username"`
	Password  string    `db:"password" json:"-"` // Never expose password in JSON
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
