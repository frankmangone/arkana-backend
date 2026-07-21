package tests

import (
	"arkana/features/notifications/models"
	"arkana/features/notifications/services"
	"testing"
)

func TestNotificationServiceCreate(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewNotificationService(db)
	recipient := insertTestUser(t, db, "recipient@example.com")
	actor := insertTestUser(t, db, "actor@example.com")

	t.Run("creates a notification", func(t *testing.T) {
		postID := 1
		err := svc.Create(db, recipient, actor, models.TypePostLiked, &postID, nil)
		if err != nil {
			t.Fatal(err)
		}

		resp, err := svc.List(recipient, 20, 0)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Total != 1 {
			t.Fatalf("total = %d, want 1", resp.Total)
		}
		if resp.Notifications[0].Type != models.TypePostLiked {
			t.Errorf("type = %q, want %q", resp.Notifications[0].Type, models.TypePostLiked)
		}
		if resp.Notifications[0].ActorUserID != actor {
			t.Errorf("actor_user_id = %d, want %d", resp.Notifications[0].ActorUserID, actor)
		}
	})

	t.Run("no-ops when recipient is the actor", func(t *testing.T) {
		postID := 2
		err := svc.Create(db, actor, actor, models.TypePostLiked, &postID, nil)
		if err != nil {
			t.Fatal(err)
		}

		resp, err := svc.List(actor, 20, 0)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Total != 0 {
			t.Fatalf("total = %d, want 0 (self-notification should be suppressed)", resp.Total)
		}
	})
}

func TestNotificationServiceList(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewNotificationService(db)
	recipient := insertTestUser(t, db, "listrecipient@example.com")
	actor := insertTestUser(t, db, "listactor@example.com")

	for i := 0; i < 3; i++ {
		postID := i
		if err := svc.Create(db, recipient, actor, models.TypePostLiked, &postID, nil); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("paginates and orders newest first", func(t *testing.T) {
		page, err := svc.List(recipient, 2, 0)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 3 {
			t.Errorf("total = %d, want 3", page.Total)
		}
		if len(page.Notifications) != 2 {
			t.Fatalf("len(notifications) = %d, want 2", len(page.Notifications))
		}
		if *page.Notifications[0].PostID != 2 {
			t.Errorf("first result post_id = %d, want 2 (newest first)", *page.Notifications[0].PostID)
		}
	})
}

func TestNotificationServiceMarkRead(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewNotificationService(db)
	recipient := insertTestUser(t, db, "readrecipient@example.com")
	actor := insertTestUser(t, db, "readactor@example.com")
	otherUser := insertTestUser(t, db, "otheruser@example.com")

	postID := 1
	if err := svc.Create(db, recipient, actor, models.TypePostLiked, &postID, nil); err != nil {
		t.Fatal(err)
	}
	page, _ := svc.List(recipient, 20, 0)
	notifID := page.Notifications[0].ID

	t.Run("marks read and drops unread count", func(t *testing.T) {
		before, _ := svc.UnreadCount(recipient)
		if before != 1 {
			t.Fatalf("before count = %d, want 1", before)
		}

		if err := svc.MarkRead(notifID, recipient); err != nil {
			t.Fatal(err)
		}

		after, _ := svc.UnreadCount(recipient)
		if after != 0 {
			t.Errorf("after count = %d, want 0", after)
		}
	})

	t.Run("rejects marking someone else's notification", func(t *testing.T) {
		postID2 := 2
		if err := svc.Create(db, otherUser, actor, models.TypePostLiked, &postID2, nil); err != nil {
			t.Fatal(err)
		}
		otherPage, _ := svc.List(otherUser, 20, 0)
		otherNotifID := otherPage.Notifications[0].ID

		err := svc.MarkRead(otherNotifID, recipient)
		if err != services.ErrNotificationNotFound {
			t.Errorf("err = %v, want ErrNotificationNotFound", err)
		}
	})

	t.Run("rejects nonexistent notification id", func(t *testing.T) {
		err := svc.MarkRead(99999, recipient)
		if err != services.ErrNotificationNotFound {
			t.Errorf("err = %v, want ErrNotificationNotFound", err)
		}
	})
}
