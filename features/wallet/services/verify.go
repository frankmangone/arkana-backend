package services

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// VerifySignature dispatches signature verification based on the system.
func VerifySignature(system, address, message, signature string) error {
	switch system {
	case "ethereum":
		return verifyEthereum(address, message, signature)
	default:
		return fmt.Errorf("unsupported system: %s", system)
	}
}

// verifyEthereum verifies an EIP-191 personal_sign signature.
func verifyEthereum(address, message, signature string) error {
	// Decode the hex signature
	sig, err := hex.DecodeString(strings.TrimPrefix(signature, "0x"))
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	if len(sig) != 65 {
		return fmt.Errorf("invalid signature length: %d", len(sig))
	}

	// EIP-191: Ethereum uses recovery id 27/28, normalize to 0/1
	if sig[64] >= 27 {
		sig[64] -= 27
	}

	// Hash the message with the Ethereum prefix (EIP-191 personal_sign)
	prefixedMessage := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := crypto.Keccak256Hash([]byte(prefixedMessage))

	// Recover the public key from the signature
	pubKey, err := crypto.Ecrecover(hash.Bytes(), sig)
	if err != nil {
		return fmt.Errorf("failed to recover public key: %w", err)
	}

	// Derive address from the recovered public key
	recoveredPub, err := crypto.UnmarshalPubkey(pubKey)
	if err != nil {
		return fmt.Errorf("failed to unmarshal public key: %w", err)
	}
	recoveredAddr := crypto.PubkeyToAddress(*recoveredPub)

	// Compare addresses (case-insensitive)
	if !strings.EqualFold(recoveredAddr.Hex(), address) {
		return fmt.Errorf("signature does not match address")
	}

	return nil
}
