package tests

import (
	"arkana/features/posts/models"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestToggleLikeHandler(t *testing.T) {
	db := setupTestDB(t)
	router := setupRouter(t, db)
	key, addr := generateTestKey(t)
	insertTestWallet(t, db, addr)

	t.Run("returns 404 for non-existent post", func(t *testing.T) {
		jws := signJWS(t, key, map[string]any{"action": "like", "path": "non-existent-post"})
		req := httptest.NewRequest("POST", "/api/posts/non-existent-post/like", strings.NewReader(jws))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("likes a post", func(t *testing.T) {
		insertTestPost(t, db, "test-path")

		jws := signJWS(t, key, map[string]any{"action": "like", "path": "test-path"})
		req := httptest.NewRequest("POST", "/api/posts/test-path/like", strings.NewReader(jws))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		var resp models.ToggleLikeResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if !resp.Liked {
			t.Error("liked = false, want true")
		}
		if resp.LikeCount != 1 {
			t.Errorf("like_count = %d, want 1", resp.LikeCount)
		}
	})

	t.Run("unlikes on second call", func(t *testing.T) {
		// Pass liked: true to indicate this is an unlike action
		jws := signJWS(t, key, map[string]any{"action": "like", "path": "test-path", "liked": true})
		req := httptest.NewRequest("POST", "/api/posts/test-path/like", strings.NewReader(jws))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var resp models.ToggleLikeResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.Liked {
			t.Error("liked = true, want false")
		}
		if resp.LikeCount != 0 {
			t.Errorf("like_count = %d, want 0", resp.LikeCount)
		}
	})

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/posts/test-path/like", strings.NewReader("not.a.jws"))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}

func TestCreateCommentHandler(t *testing.T) {
	db := setupTestDB(t)
	router := setupRouter(t, db)
	key, addr := generateTestKey(t)
	insertTestWallet(t, db, addr)

	t.Run("returns 404 for non-existent post", func(t *testing.T) {
		jws := signJWS(t, key, map[string]any{"body": "test comment"})
		req := httptest.NewRequest("POST", "/api/posts/non-existent-post/comments", strings.NewReader(jws))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("creates a comment", func(t *testing.T) {
		insertTestPost(t, db, "my-post")

		jws := signJWS(t, key, map[string]any{"body": "great post"})
		req := httptest.NewRequest("POST", "/api/posts/my-post/comments", strings.NewReader(jws))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
		}

		var comment models.Comment
		json.NewDecoder(rec.Body).Decode(&comment)
		if comment.Body != "great post" {
			t.Errorf("body = %q, want %q", comment.Body, "great post")
		}
		if comment.ParentID != nil {
			t.Errorf("parent_id = %v, want nil", comment.ParentID)
		}
	})

	t.Run("creates a reply", func(t *testing.T) {
		// Create parent comment (post already exists from previous test)
		jws := signJWS(t, key, map[string]any{"body": "parent"})
		req := httptest.NewRequest("POST", "/api/posts/my-post/comments", strings.NewReader(jws))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var parent models.Comment
		json.NewDecoder(rec.Body).Decode(&parent)

		// Reply
		jws = signJWS(t, key, map[string]any{"body": "reply", "parent_id": parent.ID})
		req = httptest.NewRequest("POST", "/api/posts/my-post/comments", strings.NewReader(jws))
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
		}

		var child models.Comment
		json.NewDecoder(rec.Body).Decode(&child)
		if child.ParentID == nil || *child.ParentID != parent.ID {
			t.Errorf("parent_id = %v, want %d", child.ParentID, parent.ID)
		}
	})

	t.Run("rejects empty body", func(t *testing.T) {
		jws := signJWS(t, key, map[string]any{"body": ""})
		req := httptest.NewRequest("POST", "/api/posts/my-post/comments", strings.NewReader(jws))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/posts/my-post/comments", strings.NewReader("bad.jws.data"))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}

func TestGetPostInfoHandler(t *testing.T) {
	db := setupTestDB(t)
	router := setupRouter(t, db)

	t.Run("returns 404 for non-existent post", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts/non-existent-post/info", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("returns post info for existing post", func(t *testing.T) {
		insertTestPost(t, db, "existing-post")

		req := httptest.NewRequest("GET", "/api/posts/existing-post/info", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		var resp models.PostInfoResponse
		json.NewDecoder(rec.Body).Decode(&resp)

		if resp.Path != "existing-post" {
			t.Errorf("path = %q, want %q", resp.Path, "existing-post")
		}
		if resp.LikeCount != 0 {
			t.Errorf("like_count = %d, want 0", resp.LikeCount)
		}
		if resp.Liked {
			t.Error("liked = true, want false")
		}
	})

	t.Run("returns liked status for authenticated user", func(t *testing.T) {
		insertTestPost(t, db, "liked-post")
		key, addr := generateTestKey(t)
		insertTestWallet(t, db, addr)

		// Like the post first
		jws := signJWS(t, key, map[string]any{"action": "like", "path": "liked-post"})
		req := httptest.NewRequest("POST", "/api/posts/liked-post/like", strings.NewReader(jws))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("like failed: status = %d; body: %s", rec.Code, rec.Body.String())
		}

		// Now check post info with wallet
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/posts/liked-post/info?wallet=%s", addr), nil)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		var resp models.PostInfoResponse
		json.NewDecoder(rec.Body).Decode(&resp)

		if resp.LikeCount != 1 {
			t.Errorf("like_count = %d, want 1", resp.LikeCount)
		}
		if !resp.Liked {
			t.Error("liked = false, want true")
		}
	})

	t.Run("handles paths with slashes", func(t *testing.T) {
		insertTestPost(t, db, "category/my-post")

		req := httptest.NewRequest("GET", "/api/posts/category/my-post/info", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		var resp models.PostInfoResponse
		json.NewDecoder(rec.Body).Decode(&resp)

		if resp.Path != "category/my-post" {
			t.Errorf("path = %q, want %q", resp.Path, "category/my-post")
		}
	})
}
