package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	PurposeConfirm     = "confirm"
	PurposeUnsubscribe = "unsubscribe"
)

// GenerateSubscriptionToken produces a stateless, HMAC-signed token for a
// subscriber id and purpose. The purpose is part of the signed payload so a
// confirm token can't be replayed as an unsubscribe token or vice versa.
func GenerateSubscriptionToken(secret string, subscriberID int, purpose string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.%s", subscriberID, purpose) //nolint:errcheck // hash.Hash.Write never returns an error
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySubscriptionToken checks a token against the expected id/purpose
// using a constant-time comparison.
func VerifySubscriptionToken(secret string, subscriberID int, purpose, token string) bool {
	expected := GenerateSubscriptionToken(secret, subscriberID, purpose)
	return hmac.Equal([]byte(expected), []byte(token))
}
