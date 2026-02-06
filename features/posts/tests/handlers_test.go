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
		jws := signJWS(t, key, map[string]any{"action": "LIKE_POST", "path": "non-existent-post"})
		req := httptest.NewRequest("POST", "/api/posts/non-existent-post/like", strings.NewReader(jws))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("likes a post", func(t *testing.T) {
		insertTestPost(t, db, "test-path")

		jws := signJWS(t, key, map[string]any{"action": "LIKE_POST", "path": "test-path"})
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
		jws := signJWS(t, key, map[string]any{"action": "UNLIKE_POST", "path": "test-path"})
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
		jws := signJWS(t, key, map[string]any{"action": "CREATE_COMMENT", "body": "test comment"})
		req := httptest.NewRequest("POST", "/api/posts/non-existent-post/comments", strings.NewReader(jws))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("creates a comment", func(t *testing.T) {
		insertTestPost(t, db, "my-post")

		jws := signJWS(t, key, map[string]any{"action": "CREATE_COMMENT", "body": "great post"})
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
		jws := signJWS(t, key, map[string]any{"action": "CREATE_COMMENT", "body": "parent"})
		req := httptest.NewRequest("POST", "/api/posts/my-post/comments", strings.NewReader(jws))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var parent models.Comment
		json.NewDecoder(rec.Body).Decode(&parent)

		// Reply
		jws = signJWS(t, key, map[string]any{"action": "CREATE_COMMENT", "body": "reply", "parent_id": parent.ID})
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
		jws := signJWS(t, key, map[string]any{"action": "CREATE_COMMENT", "body": ""})
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

	t.Run("rejects comment exceeding max length", func(t *testing.T) {
		// Create a comment body that exceeds 1000 characters
		longBody := strings.Repeat("x", 1001)
		jws := signJWS(t, key, map[string]any{"action": "CREATE_COMMENT", "body": longBody})
		req := httptest.NewRequest("POST", "/api/posts/my-post/comments", strings.NewReader(jws))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestGetCommentsHandler(t *testing.T) {
	db := setupTestDB(t)
	router := setupRouter(t, db)
	key, addr := generateTestKey(t)
	insertTestWallet(t, db, addr)

	t.Run("returns 404 for non-existent post", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts/non-existent/comments", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("returns empty list for post with no comments", func(t *testing.T) {
		insertTestPost(t, db, "no-comments-post")

		req := httptest.NewRequest("GET", "/api/posts/no-comments-post/comments", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		var resp models.CommentsResponse
		json.NewDecoder(rec.Body).Decode(&resp)

		if resp.Total != 0 {
			t.Errorf("total = %d, want 0", resp.Total)
		}
		if len(resp.Comments) != 0 {
			t.Errorf("comments length = %d, want 0", len(resp.Comments))
		}
	})

	t.Run("returns comments with author address", func(t *testing.T) {
		insertTestPost(t, db, "commented-post")

		// Create a comment
		jws := signJWS(t, key, map[string]any{"action": "CREATE_COMMENT", "body": "test comment"})
		req := httptest.NewRequest("POST", "/api/posts/commented-post/comments", strings.NewReader(jws))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("create comment failed: status = %d; body: %s", rec.Code, rec.Body.String())
		}

		// Fetch comments
		req = httptest.NewRequest("GET", "/api/posts/commented-post/comments", nil)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		var resp models.CommentsResponse
		json.NewDecoder(rec.Body).Decode(&resp)

		if resp.Total != 1 {
			t.Errorf("total = %d, want 1", resp.Total)
		}
		if len(resp.Comments) != 1 {
			t.Fatalf("comments length = %d, want 1", len(resp.Comments))
		}
		if resp.Comments[0].Body != "test comment" {
			t.Errorf("body = %q, want %q", resp.Comments[0].Body, "test comment")
		}
		if resp.Comments[0].AuthorAddress == "" {
			t.Error("author_address is empty")
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
		jws := signJWS(t, key, map[string]any{"action": "LIKE_POST", "path": "liked-post"})
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
