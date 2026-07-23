package tests

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"arkana/features/posts/models"
)

func signAdminRequest(secret string, body []byte) map[string]string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))
	return map[string]string{"X-Timestamp": timestamp, "X-Signature": signature}
}

func TestPublishHandler(t *testing.T) {
	t.Run("publishes a post with a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db)

		body, _ := json.Marshal(models.PublishPostRequest{
			Path:       "cryptography-101/handler-test",
			Lang:       "en",
			RawContent: "---\ntitle: Handler Test\n---\nsome content\n",
		})
		headers := signAdminRequest(testAdminSecret, body)
		req := httptest.NewRequest("POST", "/api/admin/posts", bytes.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.PublishPostResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if !resp.Published {
			t.Error("published = false, want true")
		}

		var count int
		if err := db.QueryRow(
			"SELECT COUNT(*) FROM post_contents WHERE lang = 'en' AND path = 'cryptography-101/handler-test.md'",
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("post_contents row count = %d, want 1", count)
		}
	})

	t.Run("rejects a request without a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db)

		body, _ := json.Marshal(models.PublishPostRequest{Path: "x", Lang: "en", RawContent: "c"})
		req := httptest.NewRequest("POST", "/api/admin/posts", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("rejects a missing required field", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db)

		body, _ := json.Marshal(models.PublishPostRequest{Path: "x", Lang: "en"})
		headers := signAdminRequest(testAdminSecret, body)
		req := httptest.NewRequest("POST", "/api/admin/posts", bytes.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})
}
