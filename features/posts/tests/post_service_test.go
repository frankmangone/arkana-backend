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
