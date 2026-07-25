package tests

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

func TestListContentHandler(t *testing.T) {
	t.Run("returns visible post content with a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db)

		postID := insertTestPost(t, db, "cryptography-101/list-content")
		insertPostContent(t, db, postID, "en", "cryptography-101/list-content.md", "---\ntitle: T\n---\nbody\n", true)
		insertPostContent(t, db, postID, "en", "cryptography-101/hidden-content.md", "---\ntitle: H\n---\nbody\n", false)

		headers := signAdminRequest(testAdminSecret, []byte{})
		req := httptest.NewRequest("GET", "/api/admin/posts", nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.AdminPostContentListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if resp.Total != 1 {
			t.Fatalf("total = %d, want 1 (hidden row excluded)", resp.Total)
		}
		if len(resp.Data) != 1 || resp.Data[0].Path != "cryptography-101/list-content.md" {
			t.Fatalf("data = %+v, want the one visible row", resp.Data)
		}
	})

	t.Run("applies limit and offset query params", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db)
		postID := insertTestPost(t, db, "cryptography-101/paged-handler")
		for i := 0; i < 3; i++ {
			path := fmt.Sprintf("cryptography-101/paged-handler-%d.md", i)
			insertPostContent(t, db, postID, "en", path, "content", true)
		}

		headers := signAdminRequest(testAdminSecret, []byte{})
		req := httptest.NewRequest("GET", "/api/admin/posts?limit=2&offset=1", nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.AdminPostContentListResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.Total != 3 {
			t.Errorf("total = %d, want 3", resp.Total)
		}
		if len(resp.Data) != 2 {
			t.Errorf("len(data) = %d, want 2", len(resp.Data))
		}
	})

	t.Run("rejects an invalid limit", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db)

		headers := signAdminRequest(testAdminSecret, []byte{})
		req := httptest.NewRequest("GET", "/api/admin/posts?limit=not-a-number", nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("rejects a request without a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db)

		req := httptest.NewRequest("GET", "/api/admin/posts", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}

func TestPublishHandlerTagValidation(t *testing.T) {
	t.Run("rejects a post referencing an unregistered tag", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouterWithTagChecker(t, db, &fakeTagChecker{missing: []string{"unknownTag"}})

		body, _ := json.Marshal(models.PublishPostRequest{
			Path:       "cryptography-101/handler-bad-tag",
			Lang:       "en",
			RawContent: "---\ntitle: T\ntags:\n  - unknownTag\n---\nbody\n",
		})
		headers := signAdminRequest(testAdminSecret, body)
		req := httptest.NewRequest("POST", "/api/admin/posts", bytes.NewReader(body))
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
