package services

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

const MaxMessageAge = 5 * time.Minute

// BuildSigningMessage constructs a human-readable message from the payload.
// This must match exactly with the frontend implementation.
func BuildSigningMessage(payloadBytes []byte) string {
	var payload struct {
		Address   string `json:"addr"`
		Timestamp int64  `json:"ts"`
		Action    string `json:"action,omitempty"`
		Path      string `json:"path,omitempty"`
		Liked     *bool  `json:"liked,omitempty"`
	}
	json.Unmarshal(payloadBytes, &payload)

	// Determine title based on action
	title := "Arkana Login"
	if payload.Action == "like" {
		// Check if this is an unlike action (current liked state is true)
		if payload.Liked != nil && *payload.Liked {
			title = "Arkana - Unlike Post"
		} else {
			title = "Arkana - Like Post"
		}
	}

	msg := fmt.Sprintf("%s\n\nAddress: %s\nTimestamp: %d", title, payload.Address, payload.Timestamp)
	if payload.Path != "" {
		msg += fmt.Sprintf("\nPath: %s", payload.Path)
	}
	return msg
}

// JWSEnvelope represents the three dot-separated parts of a compact JWS.
type JWSEnvelope struct {
	Protected string // base64url-encoded header
	Payload   string // base64url-encoded payload
	Signature string // hex-encoded wallet signature
}

// JWSHeader is the decoded protected header.
type JWSHeader struct {
	System string `json:"sys"`
}

// VerifiedJWS is the result of a successful JWS verification.
type VerifiedJWS struct {
	Header  JWSHeader
	Address string
	Payload json.RawMessage
}

// ParseCompactJWS splits a compact JWS string (header.payload.signature) into its parts.
func ParseCompactJWS(raw string) (*JWSEnvelope, error) {
	parts := strings.SplitN(strings.TrimSpace(raw), ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWS: expected 3 dot-separated parts")
	}
	return &JWSEnvelope{
		Protected: parts[0],
		Payload:   parts[1],
		Signature: parts[2],
	}, nil
}

// VerifyJWS cryptographically verifies a JWS envelope. It decodes the header
// and payload, checks the timestamp, and verifies the signature against the
// claimed address. Returns the verified result with the recovered address.
func VerifyJWS(envelope *JWSEnvelope) (*VerifiedJWS, error) {
	// Decode header
	headerBytes, err := base64.RawURLEncoding.DecodeString(envelope.Protected)
	if err != nil {
		return nil, fmt.Errorf("invalid protected header encoding")
	}
	var header JWSHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("invalid protected header")
	}
	if header.System == "" {
		return nil, fmt.Errorf("missing system in header")
	}

	// Decode payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(envelope.Payload)
	if err != nil {
		return nil, fmt.Errorf("invalid payload encoding")
	}

	// Extract common fields
	var base struct {
		Address   string `json:"addr"`
		Timestamp int64  `json:"ts"`
	}
	if err := json.Unmarshal(payloadBytes, &base); err != nil {
		return nil, fmt.Errorf("invalid payload")
	}
	if base.Address == "" {
		return nil, fmt.Errorf("missing address in payload")
	}
	if base.Timestamp == 0 {
		return nil, fmt.Errorf("missing timestamp in payload")
	}

	// Check timestamp freshness
	age := time.Since(time.Unix(base.Timestamp, 0))
	if age > MaxMessageAge || age < -MaxMessageAge {
		return nil, fmt.Errorf("message expired")
	}

	// Build human-readable signing message from payload
	signingInput := BuildSigningMessage(payloadBytes)
	log.Printf("[JWS] Signing message to verify:\n%s", signingInput)

	// Verify signature (recovers address and compares with claimed address)
	if err := VerifySignature(header.System, base.Address, signingInput, envelope.Signature); err != nil {
		log.Printf("[JWS] Signature verification failed: %v", err)
		return nil, err
	}

	return &VerifiedJWS{
		Header:  header,
		Address: strings.ToLower(base.Address),
		Payload: json.RawMessage(payloadBytes),
	}, nil
}
