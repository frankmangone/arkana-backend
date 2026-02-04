package tests

import (
	"arkana/features/posts/models"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestToggleLikeHandler(t *testing.T) {
	db := setupTestDB(t)
	router, ts := setupRouter(t, db)
	walletID := insertTestWallet(t, db, "0xabc")
	token, _ := ts.GenerateToken(walletID, "0xabc", "ethereum")

	t.Run("likes a post", func(t *testing.T) {
		req := authedRequest("POST", "/api/posts/test-path/like", token, nil)
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
		req := authedRequest("POST", "/api/posts/test-path/like", token, nil)
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
	router, ts := setupRouter(t, db)
	walletID := insertTestWallet(t, db, "0xabc")
	token, _ := ts.GenerateToken(walletID, "0xabc", "ethereum")

	t.Run("creates a comment", func(t *testing.T) {
		body := models.CreateCommentRequest{Body: "great post"}
		req := authedRequest("POST", "/api/posts/my-post/comments", token, body)
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
		body := models.CreateCommentRequest{Body: "parent"}
		req := authedRequest("POST", "/api/posts/my-post/comments", token, body)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var parent models.Comment
		json.NewDecoder(rec.Body).Decode(&parent)

		reply := models.CreateCommentRequest{Body: "reply", ParentID: &parent.ID}
		req = authedRequest("POST", "/api/posts/my-post/comments", token, reply)
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
		body := models.CreateCommentRequest{Body: ""}
		req := authedRequest("POST", "/api/posts/my-post/comments", token, body)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/posts/my-post/comments", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}
