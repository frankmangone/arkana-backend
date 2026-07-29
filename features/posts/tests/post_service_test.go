package tests

import (
	notifmodels "arkana/features/notifications/models"
	notifservices "arkana/features/notifications/services"
	"arkana/features/posts/services"
	"fmt"
	"testing"
)

func TestGetOrCreateByPath(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewPostService(db, notifservices.NewNotificationService(db))

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
	svc := services.NewPostService(db, notifservices.NewNotificationService(db))
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
	svc := services.NewPostService(db, notifservices.NewNotificationService(db))
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
	svc := services.NewPostService(db, notifservices.NewNotificationService(db))
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

func TestGetPostInfoReadField(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewPostService(db, notifservices.NewNotificationService(db))
	userID := insertTestUser(t, db, "infosreader@example.com")
	post, _ := svc.GetOrCreateByPath("info-read-post")

	t.Run("read is false before marking read", func(t *testing.T) {
		info, err := svc.GetPostInfo("info-read-post", userID)
		if err != nil {
			t.Fatal(err)
		}
		if info.Read {
			t.Error("read = true, want false")
		}
	})

	t.Run("read is true after marking read", func(t *testing.T) {
		svc.ToggleRead(post.ID, userID)

		info, err := svc.GetPostInfo("info-read-post", userID)
		if err != nil {
			t.Fatal(err)
		}
		if !info.Read {
			t.Error("read = false, want true")
		}
	})

	t.Run("read is false for anonymous request", func(t *testing.T) {
		info, err := svc.GetPostInfo("info-read-post", 0)
		if err != nil {
			t.Fatal(err)
		}
		if info.Read {
			t.Error("read = true, want false for userID 0")
		}
	})
}

func TestToggleLikeNotifications(t *testing.T) {
	db := setupTestDB(t)
	notifSvc := notifservices.NewNotificationService(db)
	svc := services.NewPostService(db, notifSvc)

	t.Run("liking a post notifies the writer", func(t *testing.T) {
		post, _ := svc.GetOrCreateByPath("like-notify-post")
		writerUser := insertTestUser(t, db, "likewriter@example.com")
		writerID := insertTestWriter(t, db, "Like Writer", &writerUser)
		setPostWriter(t, db, post.ID, writerID)
		liker := insertTestUser(t, db, "likeliker@example.com")

		if _, _, err := svc.ToggleLike(post.ID, liker); err != nil {
			t.Fatal(err)
		}

		if got := countNotifications(t, db, writerUser, notifmodels.TypePostLiked); got != 1 {
			t.Errorf("post_liked notifications = %d, want 1", got)
		}
	})

	t.Run("unliking does not notify", func(t *testing.T) {
		post, _ := svc.GetOrCreateByPath("unlike-notify-post")
		writerUser := insertTestUser(t, db, "unlikewriter@example.com")
		writerID := insertTestWriter(t, db, "Unlike Writer", &writerUser)
		setPostWriter(t, db, post.ID, writerID)
		liker := insertTestUser(t, db, "unlikeliker@example.com")

		if _, _, err := svc.ToggleLike(post.ID, liker); err != nil {
			t.Fatal(err)
		}
		if _, _, err := svc.ToggleLike(post.ID, liker); err != nil {
			t.Fatal(err)
		}

		if got := countNotifications(t, db, writerUser, notifmodels.TypePostLiked); got != 1 {
			t.Errorf("post_liked notifications after unlike = %d, want 1 (only the original like notifies)", got)
		}
	})

	t.Run("liking your own post does not notify yourself", func(t *testing.T) {
		post, _ := svc.GetOrCreateByPath("self-like-post")
		writerUser := insertTestUser(t, db, "selfliker@example.com")
		writerID := insertTestWriter(t, db, "Self Liker", &writerUser)
		setPostWriter(t, db, post.ID, writerID)

		if _, _, err := svc.ToggleLike(post.ID, writerUser); err != nil {
			t.Fatal(err)
		}

		if got := countNotifications(t, db, writerUser, notifmodels.TypePostLiked); got != 0 {
			t.Errorf("post_liked notifications for self-like = %d, want 0", got)
		}
	})

	t.Run("post with no writer produces no notification", func(t *testing.T) {
		post, _ := svc.GetOrCreateByPath("like-no-writer-post")
		liker := insertTestUser(t, db, "nowriterliker@example.com")

		if _, _, err := svc.ToggleLike(post.ID, liker); err != nil {
			t.Fatal(err)
		}

		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM notifications WHERE post_id = ?", post.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("notifications = %d, want 0", count)
		}
	})
}

func TestPostServiceListVisibleContentPage(t *testing.T) {
	t.Run("returns only visible content, ordered by id", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewPostService(db, notifservices.NewNotificationService(db))
		post, _ := svc.GetOrCreateByPath("cryptography-101/first")

		insertPostContent(t, db, post.ID, "en", "cryptography-101/first.md", "---\ntitle: First\n---\nbody one\n", true)
		insertPostContent(t, db, post.ID, "en", "cryptography-101/second.md", "---\ntitle: Second\n---\nbody two\n", true)
		insertPostContent(t, db, post.ID, "en", "cryptography-101/hidden.md", "---\ntitle: Hidden\n---\nbody three\n", false)

		items, total, err := svc.ListVisibleContentPage(10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if total != 2 {
			t.Fatalf("total = %d, want 2 (hidden row excluded)", total)
		}
		if len(items) != 2 {
			t.Fatalf("len(items) = %d, want 2", len(items))
		}
		if items[0].Path != "cryptography-101/first.md" || items[1].Path != "cryptography-101/second.md" {
			t.Errorf("order = [%s, %s], want [cryptography-101/first.md, cryptography-101/second.md]", items[0].Path, items[1].Path)
		}
	})

	t.Run("returns the full raw content verbatim, including frontmatter", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewPostService(db, notifservices.NewNotificationService(db))
		post, _ := svc.GetOrCreateByPath("cryptography-101/verbatim")
		raw := "---\ntitle: Verbatim\n---\n# Heading\n\nSome body.\n"
		insertPostContent(t, db, post.ID, "en", "cryptography-101/verbatim.md", raw, true)

		items, _, err := svc.ListVisibleContentPage(10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].Content != raw {
			t.Errorf("content = %q, want the full raw content %q", items[0].Content, raw)
		}
		if items[0].Lang != "en" {
			t.Errorf("lang = %q, want en", items[0].Lang)
		}
	})

	t.Run("respects limit and offset for paging", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewPostService(db, notifservices.NewNotificationService(db))
		post, _ := svc.GetOrCreateByPath("cryptography-101/paged")
		for i := 0; i < 5; i++ {
			path := fmt.Sprintf("cryptography-101/paged-%d.md", i)
			insertPostContent(t, db, post.ID, "en", path, "content", true)
		}

		page1, total, err := svc.ListVisibleContentPage(2, 0)
		if err != nil {
			t.Fatal(err)
		}
		if total != 5 {
			t.Fatalf("total = %d, want 5", total)
		}
		if len(page1) != 2 {
			t.Fatalf("len(page1) = %d, want 2", len(page1))
		}

		page2, _, err := svc.ListVisibleContentPage(2, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page2) != 2 {
			t.Fatalf("len(page2) = %d, want 2", len(page2))
		}
		if page1[0].Path == page2[0].Path {
			t.Error("page1 and page2 overlap, want distinct rows")
		}

		page3, _, err := svc.ListVisibleContentPage(2, 4)
		if err != nil {
			t.Fatal(err)
		}
		if len(page3) != 1 {
			t.Errorf("len(page3) = %d, want 1 (last partial page)", len(page3))
		}
	})

	t.Run("returns an empty slice, not nil, and total 0, when there is no content", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewPostService(db, notifservices.NewNotificationService(db))

		items, total, err := svc.ListVisibleContentPage(10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if items == nil {
			t.Error("items = nil, want an empty (non-nil) slice")
		}
		if len(items) != 0 || total != 0 {
			t.Errorf("len(items)/total = %d/%d, want 0/0", len(items), total)
		}
	})

	t.Run("returns rows with the same path but different languages as distinct items", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewPostService(db, notifservices.NewNotificationService(db))
		postID := insertTestPost(t, db, "cryptography-101/multilang")

		insertPostContent(t, db, postID, "en", "cryptography-101/multilang.md", "english content", true)
		insertPostContent(t, db, postID, "es", "cryptography-101/multilang.md", "contenido en espanol", true)

		items, total, err := svc.ListVisibleContentPage(10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if total != 2 {
			t.Fatalf("total = %d, want 2", total)
		}
		if len(items) != 2 {
			t.Fatalf("len(items) = %d, want 2", len(items))
		}

		langs := make(map[string]bool, len(items))
		for _, item := range items {
			if item.Path != "cryptography-101/multilang.md" {
				t.Errorf("path = %q, want cryptography-101/multilang.md", item.Path)
			}
			langs[item.Lang] = true
		}
		if !langs["en"] || !langs["es"] {
			t.Errorf("langs = %v, want both en and es present", langs)
		}
	})
}

func TestMissingPaths(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewPostService(db, notifservices.NewNotificationService(db))

	t.Run("returns nil for an empty input, without querying", func(t *testing.T) {
		missing, err := svc.MissingPaths(nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(missing) != 0 {
			t.Errorf("missing = %v, want empty", missing)
		}
	})

	t.Run("returns only the paths not present in posts", func(t *testing.T) {
		insertTestPost(t, db, "blockchain-101/how-it-all-began")

		missing, err := svc.MissingPaths([]string{"blockchain-101/how-it-all-began", "blockchain-101/nonexistent"})
		if err != nil {
			t.Fatal(err)
		}
		if len(missing) != 1 || missing[0] != "blockchain-101/nonexistent" {
			t.Errorf("missing = %v, want [blockchain-101/nonexistent]", missing)
		}
	})

	t.Run("returns empty when every path is registered", func(t *testing.T) {
		insertTestPost(t, db, "cryptography-101/hashing")
		insertTestPost(t, db, "cryptography-101/encryption")

		missing, err := svc.MissingPaths([]string{"cryptography-101/hashing", "cryptography-101/encryption"})
		if err != nil {
			t.Fatal(err)
		}
		if len(missing) != 0 {
			t.Errorf("missing = %v, want empty", missing)
		}
	})
}

func TestGetIDsByPaths(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewPostService(db, notifservices.NewNotificationService(db))

	t.Run("returns an empty map for an empty input, without querying", func(t *testing.T) {
		ids, err := svc.GetIDsByPaths(nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) != 0 {
			t.Errorf("ids = %v, want empty", ids)
		}
	})

	t.Run("resolves known paths and omits unknown ones", func(t *testing.T) {
		id := insertTestPost(t, db, "blockchain-101/how-it-all-began")

		ids, err := svc.GetIDsByPaths([]string{"blockchain-101/how-it-all-began", "blockchain-101/nonexistent"})
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) != 1 {
			t.Fatalf("ids = %v, want exactly 1 entry", ids)
		}
		if ids["blockchain-101/how-it-all-began"] != id {
			t.Errorf("ids[...] = %d, want %d", ids["blockchain-101/how-it-all-began"], id)
		}
	})
}
