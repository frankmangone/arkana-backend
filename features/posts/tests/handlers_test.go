package tests

import (
	"arkana/features/posts/models"
	"encoding/json"
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

	t.Run("likes a post", func(t *testing.T) {
		jws := signJWS(t, key, map[string]any{"path": "test-path"})
		req := httptest.NewRequest("POST", "/api/posts/like", strings.NewReader(jws))
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
		jws := signJWS(t, key, map[string]any{"path": "test-path"})
		req := httptest.NewRequest("POST", "/api/posts/like", strings.NewReader(jws))
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
		req := httptest.NewRequest("POST", "/api/posts/like", strings.NewReader("not.a.jws"))
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

	t.Run("creates a comment", func(t *testing.T) {
		jws := signJWS(t, key, map[string]any{"path": "my-post", "body": "great post"})
		req := httptest.NewRequest("POST", "/api/posts/comment", strings.NewReader(jws))
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
		// Create parent
		jws := signJWS(t, key, map[string]any{"path": "my-post", "body": "parent"})
		req := httptest.NewRequest("POST", "/api/posts/comment", strings.NewReader(jws))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var parent models.Comment
		json.NewDecoder(rec.Body).Decode(&parent)

		// Reply
		jws = signJWS(t, key, map[string]any{"path": "my-post", "body": "reply", "parent_id": parent.ID})
		req = httptest.NewRequest("POST", "/api/posts/comment", strings.NewReader(jws))
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
		jws := signJWS(t, key, map[string]any{"path": "my-post", "body": ""})
		req := httptest.NewRequest("POST", "/api/posts/comment", strings.NewReader(jws))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/posts/comment", strings.NewReader("bad.jws.data"))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}
