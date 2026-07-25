package tests

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"arkana/features/tags/handlers"
	"arkana/features/tags/models"
	"arkana/features/tags/services"
	"arkana/shared/adminauth"

	"github.com/gorilla/mux"
)

func signAdminRequest(secret string, body []byte) map[string]string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))
	return map[string]string{"X-Timestamp": timestamp, "X-Signature": signature}
}

func setupTagRouter(t *testing.T, db *sql.DB) *mux.Router {
	t.Helper()
	router := mux.NewRouter()
	adminAuth := adminauth.NewAdminAuthMiddleware(testAdminSecret)
	svc := services.NewTagService(db)
	handlers.RegisterRoutes(router, svc, adminAuth)
	return router
}

func TestSyncTagsHandler(t *testing.T) {
	t.Run("syncs tags with a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupTagRouter(t, db)

		body, _ := json.Marshal(models.TagSyncRequest{
			Tags: []models.TagPayload{
				{Slug: "cryptography", Translations: map[string]string{"en": "Cryptography", "es": "Criptografía"}},
			},
		})
		headers := signAdminRequest(testAdminSecret, body)
		req := httptest.NewRequest("POST", "/api/admin/tags/sync", bytes.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.TagSyncResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.Synced != 1 {
			t.Errorf("synced = %d, want 1", resp.Synced)
		}

		var count int
		db.QueryRow("SELECT COUNT(*) FROM tags").Scan(&count)
		if count != 1 {
			t.Errorf("tags row count = %d, want 1", count)
		}
	})

	t.Run("rejects a request without a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupTagRouter(t, db)

		body, _ := json.Marshal(models.TagSyncRequest{Tags: []models.TagPayload{{Slug: "x"}}})
		req := httptest.NewRequest("POST", "/api/admin/tags/sync", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("rejects a tag with an empty slug", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupTagRouter(t, db)

		body, _ := json.Marshal(models.TagSyncRequest{Tags: []models.TagPayload{{Slug: "", Translations: map[string]string{"en": "X"}}}})
		headers := signAdminRequest(testAdminSecret, body)
		req := httptest.NewRequest("POST", "/api/admin/tags/sync", bytes.NewReader(body))
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
