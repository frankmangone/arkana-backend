package services

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("password cannot be empty")
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}

	return string(hashedBytes), nil
}

// ValidatePassword validates a password against a bcrypt hash
// hash can be nil for OAuth users (they don't have passwords)
func ValidatePassword(hash *string, password string) error {
	if hash == nil {
		return errors.New("password hash is not set")
	}
	return bcrypt.CompareHashAndPassword([]byte(*hash), []byte(password))
}
