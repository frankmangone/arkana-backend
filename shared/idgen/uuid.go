package idgen

import (
	"crypto/rand"
	"fmt"
)

// NewV4 generates a random RFC 4122 version 4 UUID using crypto/rand,
// avoiding an external dependency for something this small. Used for the
// only public-facing identifiers on questions and quiz_attempts, so a
// client can never enumerate either table via sequential ids.
func NewV4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
