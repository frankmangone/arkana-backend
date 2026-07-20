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
	userID := insertTestUser(t, db, "liker@example.com")
	token := generateTestJWT(t, userID, "liker@example.com")
	insertTestPost(t, db, "test-path")

	t.Run("likes a post", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/posts/test-path/like", nil)
		req.Header.Set("Authorization", "Bearer "+token)
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
		req := httptest.NewRequest("POST", "/api/posts/test-path/like", nil)
		req.Header.Set("Authorization", "Bearer "+token)
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
		req := httptest.NewRequest("POST", "/api/posts/test-path/like", nil)
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
	userID := insertTestUser(t, db, "commenter@example.com")
	token := generateTestJWT(t, userID, "commenter@example.com")
	insertTestPost(t, db, "my-post")

	t.Run("creates a comment", func(t *testing.T) {
		body := strings.NewReader(`{"body":"great post"}`)
		req := httptest.NewRequest("POST", "/api/posts/my-post/comments", body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
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
		parentBody := strings.NewReader(`{"body":"parent"}`)
		req := httptest.NewRequest("POST", "/api/posts/my-post/comments", parentBody)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var parent models.Comment
		json.NewDecoder(rec.Body).Decode(&parent)

		replyBody := strings.NewReader(fmt.Sprintf(`{"body":"reply","parent_id":%d}`, parent.ID))
		req = httptest.NewRequest("POST", "/api/posts/my-post/comments", replyBody)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
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
		body := strings.NewReader(`{"body":""}`)
		req := httptest.NewRequest("POST", "/api/posts/my-post/comments", body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		body := strings.NewReader(`{"body":"test"}`)
		req := httptest.NewRequest("POST", "/api/posts/my-post/comments", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("rejects comment exceeding max length", func(t *testing.T) {
		longBody := strings.Repeat("x", 1001)
		body := strings.NewReader(fmt.Sprintf(`{"body":%q}`, longBody))
		req := httptest.NewRequest("POST", "/api/posts/my-post/comments", body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
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
	userID := insertTestUser(t, db, "author@example.com")
	token := generateTestJWT(t, userID, "author@example.com")

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

	t.Run("returns comments with author username", func(t *testing.T) {
		insertTestPost(t, db, "commented-post")

		body := strings.NewReader(`{"body":"test comment"}`)
		req := httptest.NewRequest("POST", "/api/posts/commented-post/comments", body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("create comment failed: status = %d; body: %s", rec.Code, rec.Body.String())
		}

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
		if resp.Comments[0].AuthorUsername == "" {
			t.Error("author_username is empty")
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
		uid := insertTestUser(t, db, "likerinfo@example.com")
		tok := generateTestJWT(t, uid, "likerinfo@example.com")

		req := httptest.NewRequest("POST", "/api/posts/liked-post/like", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("like failed: status = %d; body: %s", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest("GET", fmt.Sprintf("/api/posts/liked-post/info?user=%d", uid), nil)
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

	t.Run("returns read status for authenticated user", func(t *testing.T) {
		insertTestPost(t, db, "read-info-post")
		uid := insertTestUser(t, db, "readinfo@example.com")
		tok := generateTestJWT(t, uid, "readinfo@example.com")

		req := httptest.NewRequest("POST", "/api/posts/read-info-post/read", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("mark read failed: status = %d; body: %s", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest("GET", fmt.Sprintf("/api/posts/read-info-post/info?user=%d", uid), nil)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		var resp models.PostInfoResponse
		json.NewDecoder(rec.Body).Decode(&resp)

		if !resp.Read {
			t.Error("read = false, want true")
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

func TestToggleReadHandler(t *testing.T) {
	db := setupTestDB(t)
	router := setupRouter(t, db)
	userID := insertTestUser(t, db, "readhandler@example.com")
	token := generateTestJWT(t, userID, "readhandler@example.com")
	insertTestPost(t, db, "read-path")

	t.Run("marks a post read", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/posts/read-path/read", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		var resp models.ToggleReadResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if !resp.Read {
			t.Error("read = false, want true")
		}
	})

	t.Run("marks unread on second call", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/posts/read-path/read", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var resp models.ToggleReadResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.Read {
			t.Error("read = true, want false")
		}
	})

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/posts/read-path/read", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}
