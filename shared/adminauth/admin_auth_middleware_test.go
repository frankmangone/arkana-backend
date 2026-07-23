package adminauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func sign(secret, timestamp, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + body))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestAdminAuthMiddleware(t *testing.T) {
	const secret = "admin-secret"
	body := `{"post_id":1}`

	newRequest := func(timestamp, signature, body string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/subscriptions/broadcast", strings.NewReader(body))
		if timestamp != "" {
			req.Header.Set("X-Timestamp", timestamp)
		}
		if signature != "" {
			req.Header.Set("X-Signature", signature)
		}
		return req
	}

	called := func() (http.Handler, *bool) {
		wasCalled := false
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wasCalled = true
			w.WriteHeader(http.StatusOK)
		}), &wasCalled
	}

	t.Run("accepts a valid signature and fresh timestamp", func(t *testing.T) {
		m := NewAdminAuthMiddleware(secret)
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		signature := sign(secret, timestamp, body)

		next, wasCalled := called()
		rec := httptest.NewRecorder()
		m.RequireHMAC(next).ServeHTTP(rec, newRequest(timestamp, signature, body))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if !*wasCalled {
			t.Error("expected next handler to be called")
		}
	})

	t.Run("rejects a missing signature", func(t *testing.T) {
		m := NewAdminAuthMiddleware(secret)
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)

		next, wasCalled := called()
		rec := httptest.NewRecorder()
		m.RequireHMAC(next).ServeHTTP(rec, newRequest(timestamp, "", body))

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if *wasCalled {
			t.Error("expected next handler NOT to be called")
		}
	})

	t.Run("rejects a signature computed with the wrong secret", func(t *testing.T) {
		m := NewAdminAuthMiddleware(secret)
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		signature := sign("wrong-secret", timestamp, body)

		rec := httptest.NewRecorder()
		next, wasCalled := called()
		m.RequireHMAC(next).ServeHTTP(rec, newRequest(timestamp, signature, body))

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if *wasCalled {
			t.Error("expected next handler NOT to be called")
		}
	})

	t.Run("rejects a stale timestamp", func(t *testing.T) {
		m := NewAdminAuthMiddleware(secret)
		timestamp := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
		signature := sign(secret, timestamp, body)

		rec := httptest.NewRecorder()
		next, wasCalled := called()
		m.RequireHMAC(next).ServeHTTP(rec, newRequest(timestamp, signature, body))

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if *wasCalled {
			t.Error("expected next handler NOT to be called")
		}
	})

	t.Run("rejects a tampered body", func(t *testing.T) {
		m := NewAdminAuthMiddleware(secret)
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		signature := sign(secret, timestamp, body)
		tamperedBody := fmt.Sprintf(`{"post_id":2}`)

		rec := httptest.NewRecorder()
		next, wasCalled := called()
		m.RequireHMAC(next).ServeHTTP(rec, newRequest(timestamp, signature, tamperedBody))

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if *wasCalled {
			t.Error("expected next handler NOT to be called")
		}
	})

	t.Run("rejects an unparseable timestamp", func(t *testing.T) {
		m := NewAdminAuthMiddleware(secret)
		signature := sign(secret, "not-a-number", body)

		rec := httptest.NewRecorder()
		next, wasCalled := called()
		m.RequireHMAC(next).ServeHTTP(rec, newRequest("not-a-number", signature, body))

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if *wasCalled {
			t.Error("expected next handler NOT to be called")
		}
	})
}
