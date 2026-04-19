package tests

import (
	"arkana/features/posts/services"
	"testing"
)

func TestCreateComment(t *testing.T) {
	db := setupTestDB(t)
	postSvc := services.NewPostService(db)
	commentSvc := services.NewCommentService(db)
	userID := insertTestUser(t, db, "test@example.com")
	post, _ := postSvc.GetOrCreateByPath("test-post")

	t.Run("creates a top-level comment", func(t *testing.T) {
		comment, err := commentSvc.Create(post.ID, userID, "hello world", nil)
		if err != nil {
			t.Fatal(err)
		}
		if comment.Body != "hello world" {
			t.Errorf("body = %q, want %q", comment.Body, "hello world")
		}
		if comment.PostID != post.ID {
			t.Errorf("post_id = %d, want %d", comment.PostID, post.ID)
		}
		if comment.UserID != userID {
			t.Errorf("user_id = %d, want %d", comment.UserID, userID)
		}
		if comment.ParentID != nil {
			t.Errorf("parent_id = %d, want nil", *comment.ParentID)
		}
	})

	t.Run("creates a reply", func(t *testing.T) {
		parent, _ := commentSvc.Create(post.ID, userID, "parent", nil)

		reply, err := commentSvc.Create(post.ID, userID, "reply", &parent.ID)
		if err != nil {
			t.Fatal(err)
		}
		if reply.ParentID == nil || *reply.ParentID != parent.ID {
			t.Errorf("parent_id = %v, want %d", reply.ParentID, parent.ID)
		}
	})

	t.Run("rejects reply to nonexistent parent", func(t *testing.T) {
		badID := 9999
		_, err := commentSvc.Create(post.ID, userID, "orphan reply", &badID)
		if err == nil {
			t.Error("expected error for nonexistent parent")
		}
	})

	t.Run("rejects reply to comment on different post", func(t *testing.T) {
		otherPost, _ := postSvc.GetOrCreateByPath("other-post")
		otherComment, _ := commentSvc.Create(otherPost.ID, userID, "other", nil)

		_, err := commentSvc.Create(post.ID, userID, "cross-post reply", &otherComment.ID)
		if err == nil {
			t.Error("expected error for cross-post reply")
		}
	})
}
