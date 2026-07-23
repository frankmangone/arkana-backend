package adminauth

import (
	"arkana/shared/httputil"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"time"
)

const maxTimestampSkew = 5 * time.Minute

// AdminAuthMiddleware authenticates admin requests via an HMAC signature
// over the request timestamp and raw body, instead of a bearer token.
type AdminAuthMiddleware struct {
	secret string
}

func NewAdminAuthMiddleware(secret string) *AdminAuthMiddleware {
	return &AdminAuthMiddleware{secret: secret}
}

// RequireHMAC requires valid X-Timestamp and X-Signature headers, where
// signature = hex(HMAC-SHA256(secret, timestamp + "." + rawBody)). Rejects
// with a generic 401 if the signature is invalid or the timestamp is more
// than 5 minutes old, without revealing which check failed.
func (m *AdminAuthMiddleware) RequireHMAC(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		timestamp := r.Header.Get("X-Timestamp")
		signature := r.Header.Get("X-Signature")
		if timestamp == "" || signature == "" || !m.valid(timestamp, signature, body) {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *AdminAuthMiddleware) valid(timestamp, signature string, body []byte) bool {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}

	skew := time.Since(time.Unix(ts, 0))
	if skew < 0 {
		skew = -skew
	}
	if skew > maxTimestampSkew {
		return false
	}

	mac := hmac.New(sha256.New, []byte(m.secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}
