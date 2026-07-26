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

	"arkana/features/readinglists/handlers"
	"arkana/features/readinglists/models"
	"arkana/features/readinglists/services"
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

func setupReadingListRouter(t *testing.T, db *sql.DB, posts services.PostChecker) *mux.Router {
	t.Helper()
	router := mux.NewRouter()
	adminAuth := adminauth.NewAdminAuthMiddleware(testAdminSecret)
	svc := services.NewReadingListService(db, posts)
	handlers.RegisterRoutes(router, svc, adminAuth)
	return router
}

func TestPublishReadingListHandler(t *testing.T) {
	t.Run("publishes with a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupReadingListRouter(t, db, &fakePostChecker{})

		body, _ := json.Marshal(samplePayload())
		headers := signAdminRequest(testAdminSecret, body)
		req := httptest.NewRequest("POST", "/api/admin/reading-lists", bytes.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.PublishReadingListResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if !resp.Published {
			t.Error("published = false, want true")
		}

		var count int
		db.QueryRow("SELECT COUNT(*) FROM reading_lists").Scan(&count)
		if count != 1 {
			t.Errorf("reading_lists row count = %d, want 1", count)
		}
	})

	t.Run("rejects a request without a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupReadingListRouter(t, db, &fakePostChecker{})

		body, _ := json.Marshal(samplePayload())
		req := httptest.NewRequest("POST", "/api/admin/reading-lists", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("rejects a payload with an empty slug", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupReadingListRouter(t, db, &fakePostChecker{})

		payload := samplePayload()
		payload.Slug = ""
		body, _ := json.Marshal(payload)
		headers := signAdminRequest(testAdminSecret, body)
		req := httptest.NewRequest("POST", "/api/admin/reading-lists", bytes.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("rejects a payload referencing an unregistered post path", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupReadingListRouter(t, db, &fakePostChecker{missing: []string{"blockchain-101/how-it-all-began"}})

		body, _ := json.Marshal(samplePayload())
		headers := signAdminRequest(testAdminSecret, body)
		req := httptest.NewRequest("POST", "/api/admin/reading-lists", bytes.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestListAllReadingListsHandler(t *testing.T) {
	t.Run("returns every reading list with a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewReadingListService(db, &fakePostChecker{})
		svc.Publish(samplePayload())
		router := setupReadingListRouter(t, db, &fakePostChecker{})

		req := httptest.NewRequest("GET", "/api/admin/reading-lists", nil)
		headers := signAdminRequest(testAdminSecret, nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.AdminReadingListListResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if len(resp.Data) != 1 || resp.Data[0].Slug != "blockchain-101" {
			t.Fatalf("data = %+v, want one blockchain-101 entry", resp.Data)
		}
	})

	t.Run("rejects a request without a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupReadingListRouter(t, db, &fakePostChecker{})

		req := httptest.NewRequest("GET", "/api/admin/reading-lists", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("returns an empty data array when there are no reading lists", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupReadingListRouter(t, db, &fakePostChecker{})

		req := httptest.NewRequest("GET", "/api/admin/reading-lists", nil)
		headers := signAdminRequest(testAdminSecret, nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var resp models.AdminReadingListListResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.Data == nil {
			t.Error("data = nil, want an empty (non-nil) array")
		}
	})
}
