package tests

import (
	"crypto/ecdsa"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"arkana/features/posts/handlers"
	"arkana/features/posts/services"
	walletsvc "arkana/features/wallet/services"

	walletmw "arkana/features/wallet/middlewares"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE wallets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			address TEXT UNIQUE NOT NULL,
			system TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path_identifier TEXT UNIQUE NOT NULL,
			like_count INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE post_likes (
			post_id INTEGER NOT NULL,
			wallet_id INTEGER NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (post_id, wallet_id),
			FOREIGN KEY (post_id) REFERENCES posts(id),
			FOREIGN KEY (wallet_id) REFERENCES wallets(id)
		);
		CREATE TABLE comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			post_id INTEGER NOT NULL,
			wallet_id INTEGER NOT NULL,
			parent_id INTEGER,
			body TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (post_id) REFERENCES posts(id),
			FOREIGN KEY (wallet_id) REFERENCES wallets(id),
			FOREIGN KEY (parent_id) REFERENCES comments(id)
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func insertTestWallet(t *testing.T, db *sql.DB, address string) int {
	t.Helper()
	address = strings.ToLower(address)
	result, err := db.Exec(
		"INSERT INTO wallets (address, system) VALUES (?, 'ethereum')", address,
	)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	return int(id)
}

func setupRouter(t *testing.T, db *sql.DB) *mux.Router {
	t.Helper()
	router := mux.NewRouter()
	ws := walletsvc.NewWalletService(db)
	auth := walletmw.NewAuthMiddleware(ws)
	ps := services.NewPostService(db)
	cs := services.NewCommentService(db)
	handlers.RegisterRoutes(router, ps, cs, auth)
	return router
}

// generateTestKey creates a new Ethereum private key and returns it with its address.
func generateTestKey(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	address := crypto.PubkeyToAddress(key.PublicKey).Hex()
	return key, address
}

// signJWS creates a compact JWS string (header.payload.signature) signed by the given key.
func signJWS(t *testing.T, key *ecdsa.PrivateKey, payload map[string]any) string {
	t.Helper()

	headerJSON, _ := json.Marshal(map[string]string{"sys": "ethereum"})
	protectedB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Inject address and timestamp if not present
	if _, ok := payload["addr"]; !ok {
		payload["addr"] = crypto.PubkeyToAddress(key.PublicKey).Hex()
	}
	if _, ok := payload["ts"]; !ok {
		payload["ts"] = time.Now().Unix()
	}

	payloadJSON, _ := json.Marshal(payload)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	signingInput := protectedB64 + "." + payloadB64

	// EIP-191 personal_sign
	prefixed := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(signingInput), signingInput)
	hash := crypto.Keccak256Hash([]byte(prefixed))

	sig, err := crypto.Sign(hash.Bytes(), key)
	if err != nil {
		t.Fatal(err)
	}
	sig[64] += 27 // EIP-191 recovery id

	return protectedB64 + "." + payloadB64 + "." + hex.EncodeToString(sig)
}

