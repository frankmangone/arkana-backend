package tests

import (
	"arkana/features/posts/services"
	"testing"
)

func TestCreateComment(t *testing.T) {
	db := setupTestDB(t)
	postSvc := services.NewPostService(db)
	commentSvc := services.NewCommentService(db)
	walletID := insertTestWallet(t, db, "0xabc")
	post, _ := postSvc.GetOrCreateByPath("test-post")

	t.Run("creates a top-level comment", func(t *testing.T) {
		comment, err := commentSvc.Create(post.ID, walletID, "hello world", nil)
		if err != nil {
			t.Fatal(err)
		}
		if comment.Body != "hello world" {
			t.Errorf("body = %q, want %q", comment.Body, "hello world")
		}
		if comment.PostID != post.ID {
			t.Errorf("post_id = %d, want %d", comment.PostID, post.ID)
		}
		if comment.WalletID != walletID {
			t.Errorf("wallet_id = %d, want %d", comment.WalletID, walletID)
		}
		if comment.ParentID != nil {
			t.Errorf("parent_id = %d, want nil", *comment.ParentID)
		}
	})

	t.Run("creates a reply", func(t *testing.T) {
		parent, _ := commentSvc.Create(post.ID, walletID, "parent", nil)

		reply, err := commentSvc.Create(post.ID, walletID, "reply", &parent.ID)
		if err != nil {
			t.Fatal(err)
		}
		if reply.ParentID == nil || *reply.ParentID != parent.ID {
			t.Errorf("parent_id = %v, want %d", reply.ParentID, parent.ID)
		}
	})

	t.Run("rejects reply to nonexistent parent", func(t *testing.T) {
		badID := 9999
		_, err := commentSvc.Create(post.ID, walletID, "orphan reply", &badID)
		if err == nil {
			t.Error("expected error for nonexistent parent")
		}
	})

	t.Run("rejects reply to comment on different post", func(t *testing.T) {
		otherPost, _ := postSvc.GetOrCreateByPath("other-post")
		otherComment, _ := commentSvc.Create(otherPost.ID, walletID, "other", nil)

		_, err := commentSvc.Create(post.ID, walletID, "cross-post reply", &otherComment.ID)
		if err == nil {
			t.Error("expected error for cross-post reply")
		}
	})
}
