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

	"arkana/features/writers/models"
)

func signAdminRequest(secret string, body []byte) map[string]string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))
	return map[string]string{"X-Timestamp": timestamp, "X-Signature": signature}
}

func TestPublishWriterHandler(t *testing.T) {
	t.Run("publishes a writer with a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db)

		body, _ := json.Marshal(models.WriterPayload{
			Slug:      "handler-test",
			Name:      "Handler Test",
			ImageURL:  "/img.png",
			AvatarURL: "/avatar.png",
		})
		headers := signAdminRequest(testAdminSecret, body)
		req := httptest.NewRequest("POST", "/api/admin/writers", bytes.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.PublishWriterResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if !resp.Published {
			t.Error("published = false, want true")
		}

		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM writers WHERE slug = 'handler-test'").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("writers row count = %d, want 1", count)
		}
	})

	t.Run("rejects a request without a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db)

		body, _ := json.Marshal(models.WriterPayload{Slug: "x", Name: "X", ImageURL: "/i.png", AvatarURL: "/a.png"})
		req := httptest.NewRequest("POST", "/api/admin/writers", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("rejects a missing required field", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db)

		body, _ := json.Marshal(models.WriterPayload{Slug: "x", Name: "X"})
		headers := signAdminRequest(testAdminSecret, body)
		req := httptest.NewRequest("POST", "/api/admin/writers", bytes.NewReader(body))
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

func TestListAllWritersHandler(t *testing.T) {
	t.Run("returns every writer, including hidden ones, with a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		insertTestWriter(t, db, "visible-writer", "Visible Writer", true)
		insertTestWriter(t, db, "hidden-writer", "Hidden Writer", false)
		router := setupRouter(t, db)

		headers := signAdminRequest(testAdminSecret, []byte{})
		req := httptest.NewRequest("GET", "/api/admin/writers", nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.AdminWriterListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Data) != 2 {
			t.Fatalf("len(data) = %d, want 2 (both visible and hidden)", len(resp.Data))
		}
	})

	t.Run("rejects a request without a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db)

		req := httptest.NewRequest("GET", "/api/admin/writers", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}
