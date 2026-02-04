package services

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type WalletClaims struct {
	WalletID int    `json:"wid"`
	Address  string `json:"addr"`
	System   string `json:"sys"`
	jwt.RegisteredClaims
}

type TokenService struct {
	secret []byte
	expiry time.Duration
}

func NewTokenService(secret string, expiry time.Duration) *TokenService {
	return &TokenService{
		secret: []byte(secret),
		expiry: expiry,
	}
}

// GenerateToken creates a signed JWT for the given wallet.
func (s *TokenService) GenerateToken(walletID int, address, system string) (string, error) {
	claims := WalletClaims{
		WalletID: walletID,
		Address:  address,
		System:   system,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.expiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ValidateToken parses and validates a token string, returning the claims.
func (s *TokenService) ValidateToken(tokenString string) (*WalletClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &WalletClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*WalletClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}
