package tests

import (
	"arkana/features/posts/services"
	"testing"
)

func TestGetOrCreateByPath(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewPostService(db)

	t.Run("creates new post", func(t *testing.T) {
		post, err := svc.GetOrCreateByPath("blog/hello-world")
		if err != nil {
			t.Fatal(err)
		}
		if post.PathIdentifier != "blog/hello-world" {
			t.Errorf("path = %q, want %q", post.PathIdentifier, "blog/hello-world")
		}
		if post.LikeCount != 0 {
			t.Errorf("like_count = %d, want 0", post.LikeCount)
		}
	})

	t.Run("returns existing post on second call", func(t *testing.T) {
		post1, _ := svc.GetOrCreateByPath("blog/existing")
		post2, err := svc.GetOrCreateByPath("blog/existing")
		if err != nil {
			t.Fatal(err)
		}
		if post1.ID != post2.ID {
			t.Errorf("IDs differ: %d vs %d", post1.ID, post2.ID)
		}
	})
}

func TestToggleLike(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewPostService(db)
	userID := insertTestUser(t, db, "toggler@example.com")
	post, _ := svc.GetOrCreateByPath("test-post")

	t.Run("first toggle likes", func(t *testing.T) {
		liked, count, err := svc.ToggleLike(post.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if !liked {
			t.Error("liked = false, want true")
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
	})

	t.Run("second toggle unlikes", func(t *testing.T) {
		liked, count, err := svc.ToggleLike(post.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if liked {
			t.Error("liked = true, want false")
		}
		if count != 0 {
			t.Errorf("count = %d, want 0", count)
		}
	})

	t.Run("multiple users", func(t *testing.T) {
		user2 := insertTestUser(t, db, "toggler2@example.com")

		svc.ToggleLike(post.ID, userID) // like
		svc.ToggleLike(post.ID, user2)  // like

		liked, count, err := svc.ToggleLike(post.ID, userID) // unlike
		if err != nil {
			t.Fatal(err)
		}
		if liked {
			t.Error("liked = true, want false")
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
	})
}

func TestToggleRead(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewPostService(db)
	userID := insertTestUser(t, db, "reader@example.com")
	post, _ := svc.GetOrCreateByPath("read-test-post")

	t.Run("first toggle marks read", func(t *testing.T) {
		read, err := svc.ToggleRead(post.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if !read {
			t.Error("read = false, want true")
		}
	})

	t.Run("second toggle marks unread", func(t *testing.T) {
		read, err := svc.ToggleRead(post.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if read {
			t.Error("read = true, want false")
		}
	})

	t.Run("multiple users", func(t *testing.T) {
		user2 := insertTestUser(t, db, "reader2@example.com")

		svc.ToggleRead(post.ID, userID) // mark read
		svc.ToggleRead(post.ID, user2)  // mark read

		read, err := svc.ToggleRead(post.ID, userID) // mark unread
		if err != nil {
			t.Fatal(err)
		}
		if read {
			t.Error("read = true, want false")
		}
	})
}

func TestGetReadStatuses(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewPostService(db)
	userID := insertTestUser(t, db, "batchreader@example.com")

	post1, _ := svc.GetOrCreateByPath("batch-post-1")
	_, _ = svc.GetOrCreateByPath("batch-post-2")
	svc.ToggleRead(post1.ID, userID) // mark post1 as read; post2 stays unread

	t.Run("returns read status per path", func(t *testing.T) {
		statuses, err := svc.GetReadStatuses(
			[]string{"batch-post-1", "batch-post-2", "never-created-post"},
			userID,
		)
		if err != nil {
			t.Fatal(err)
		}
		if !statuses["batch-post-1"] {
			t.Error("batch-post-1 = false, want true")
		}
		if statuses["batch-post-2"] {
			t.Error("batch-post-2 = true, want false")
		}
		if statuses["never-created-post"] {
			t.Error("never-created-post = true, want false")
		}
	})

	t.Run("returns all false for anonymous user", func(t *testing.T) {
		statuses, err := svc.GetReadStatuses([]string{"batch-post-1"}, 0)
		if err != nil {
			t.Fatal(err)
		}
		if statuses["batch-post-1"] {
			t.Error("batch-post-1 = true, want false for userID 0")
		}
	})

	t.Run("returns empty map for no paths", func(t *testing.T) {
		statuses, err := svc.GetReadStatuses([]string{}, userID)
		if err != nil {
			t.Fatal(err)
		}
		if len(statuses) != 0 {
			t.Errorf("len(statuses) = %d, want 0", len(statuses))
		}
	})
}
