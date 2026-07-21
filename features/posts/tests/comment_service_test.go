package tests

import (
	notifmodels "arkana/features/notifications/models"
	notifservices "arkana/features/notifications/services"
	"arkana/features/posts/services"
	"testing"
)

func TestCreateComment(t *testing.T) {
	db := setupTestDB(t)
	postSvc := services.NewPostService(db, notifservices.NewNotificationService(db))
	commentSvc := services.NewCommentService(db, notifservices.NewNotificationService(db))
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

func TestCreateCommentNotifications(t *testing.T) {
	db := setupTestDB(t)
	notifSvc := notifservices.NewNotificationService(db)
	postSvc := services.NewPostService(db, notifservices.NewNotificationService(db))
	commentSvc := services.NewCommentService(db, notifSvc)

	t.Run("reply notifies the parent comment's author", func(t *testing.T) {
		post, _ := postSvc.GetOrCreateByPath("reply-notify-post")
		parentAuthor := insertTestUser(t, db, "parentauthor@example.com")
		replier := insertTestUser(t, db, "replier@example.com")

		parent, err := commentSvc.Create(post.ID, parentAuthor, "parent comment", nil)
		if err != nil {
			t.Fatal(err)
		}

		_, err = commentSvc.Create(post.ID, replier, "a reply", &parent.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got := countNotifications(t, db, parentAuthor, notifmodels.TypeCommentReply); got != 1 {
			t.Errorf("comment_reply notifications for parent author = %d, want 1", got)
		}
	})

	t.Run("replying to your own comment does not notify yourself", func(t *testing.T) {
		post, _ := postSvc.GetOrCreateByPath("self-reply-post")
		user := insertTestUser(t, db, "selfreplier@example.com")

		parent, _ := commentSvc.Create(post.ID, user, "parent comment", nil)
		_, err := commentSvc.Create(post.ID, user, "reply to self", &parent.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got := countNotifications(t, db, user, notifmodels.TypeCommentReply); got != 0 {
			t.Errorf("comment_reply notifications for self-reply = %d, want 0", got)
		}
	})

	t.Run("comment on a post notifies the post's writer", func(t *testing.T) {
		post, _ := postSvc.GetOrCreateByPath("writer-notify-post")
		writerUser := insertTestUser(t, db, "writeruser@example.com")
		writerID := insertTestWriter(t, db, "The Writer", &writerUser)
		setPostWriter(t, db, post.ID, writerID)
		commenter := insertTestUser(t, db, "commenter2@example.com")

		_, err := commentSvc.Create(post.ID, commenter, "nice post", nil)
		if err != nil {
			t.Fatal(err)
		}

		if got := countNotifications(t, db, writerUser, notifmodels.TypePostCommented); got != 1 {
			t.Errorf("post_commented notifications for writer = %d, want 1", got)
		}
	})

	t.Run("writer commenting on their own post does not notify themselves", func(t *testing.T) {
		post, _ := postSvc.GetOrCreateByPath("self-comment-post")
		writerUser := insertTestUser(t, db, "selfwriter@example.com")
		writerID := insertTestWriter(t, db, "Self Writer", &writerUser)
		setPostWriter(t, db, post.ID, writerID)

		_, err := commentSvc.Create(post.ID, writerUser, "commenting on my own post", nil)
		if err != nil {
			t.Fatal(err)
		}

		if got := countNotifications(t, db, writerUser, notifmodels.TypePostCommented); got != 0 {
			t.Errorf("post_commented notifications for self-comment = %d, want 0", got)
		}
	})

	t.Run("reply to the writer's own comment only fires the reply notification, not a duplicate writer notification", func(t *testing.T) {
		post, _ := postSvc.GetOrCreateByPath("dedup-post")
		writerUser := insertTestUser(t, db, "dedupwriter@example.com")
		writerID := insertTestWriter(t, db, "Dedup Writer", &writerUser)
		setPostWriter(t, db, post.ID, writerID)
		replier := insertTestUser(t, db, "dedupreplier@example.com")

		parent, err := commentSvc.Create(post.ID, writerUser, "writer's own comment", nil)
		if err != nil {
			t.Fatal(err)
		}

		_, err = commentSvc.Create(post.ID, replier, "reply to the writer", &parent.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got := countNotifications(t, db, writerUser, notifmodels.TypeCommentReply); got != 1 {
			t.Errorf("comment_reply notifications for writer = %d, want 1", got)
		}
		if got := countNotifications(t, db, writerUser, notifmodels.TypePostCommented); got != 0 {
			t.Errorf("post_commented notifications for writer = %d, want 0 (would double-notify the same reply)", got)
		}
	})

	t.Run("post with no writer produces no post_commented notification", func(t *testing.T) {
		post, _ := postSvc.GetOrCreateByPath("no-writer-post")
		commenter := insertTestUser(t, db, "nowritercommenter@example.com")

		_, err := commentSvc.Create(post.ID, commenter, "comment on writerless post", nil)
		if err != nil {
			t.Fatal(err)
		}

		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM notifications WHERE post_id = ? AND type = ?", post.ID, notifmodels.TypePostCommented).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("post_commented notifications = %d, want 0", count)
		}
	})
}
