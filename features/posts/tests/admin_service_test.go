package tests

import (
	"database/sql"
	"errors"
	"testing"

	notifservices "arkana/features/notifications/services"
	"arkana/features/posts/services"
)

// fakeIndexer records IndexPost calls so tests can assert on them without
// making real Meilisearch calls.
type fakeIndexer struct {
	calls   []indexCall
	failErr error
}

type indexCall struct {
	lang, path, title, description, content string
	tags                                    []string
}

func (f *fakeIndexer) IndexPost(lang, path, title, description, content string, tags []string) error {
	if f.failErr != nil {
		return f.failErr
	}
	f.calls = append(f.calls, indexCall{lang, path, title, description, content, tags})
	return nil
}

// fakeTagChecker reports a slug as missing only if it's explicitly listed
// in missing, so existing Publish tests that don't exercise tag
// validation don't need to change - every tag they use is treated as
// already registered by default (missing is nil).
type fakeTagChecker struct {
	missing []string
}

func (f *fakeTagChecker) MissingTags(slugs []string) ([]string, error) {
	return f.missing, nil
}

func getPostContent(t *testing.T, db *sql.DB, lang, path string) (title, thumbnail, content sql.NullString, found bool) {
	t.Helper()
	err := db.QueryRow(
		"SELECT title, thumbnail, content FROM post_contents WHERE lang = ? AND path = ?",
		lang, path,
	).Scan(&title, &thumbnail, &content)
	if err == sql.ErrNoRows {
		return sql.NullString{}, sql.NullString{}, sql.NullString{}, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return title, thumbnail, content, true
}

func TestAdminPostServicePublish(t *testing.T) {
	t.Run("parses frontmatter, stores the full raw content, and indexes the stripped body", func(t *testing.T) {
		db := setupTestDB(t)
		postSvc := services.NewPostService(db, notifservices.NewNotificationService(db))
		indexer := &fakeIndexer{}
		adminSvc := services.NewAdminPostService(db, postSvc, indexer, &fakeTagChecker{})

		raw := "---\n" +
			"title: Hashing 101\n" +
			"thumbnail: https://example.com/thumb.png\n" +
			"description: a post about hashing\n" +
			"tags:\n  - crypto\n  - hashing\n" +
			"---\n" +
			"# Hashing\n\nSome **bold** body content.\n"

		err := adminSvc.Publish(services.PublishInput{
			Path:       "cryptography-101/hashing",
			Lang:       "en",
			RawContent: raw,
		})
		if err != nil {
			t.Fatal(err)
		}

		title, thumbnail, content, found := getPostContent(t, db, "en", "cryptography-101/hashing.md")
		if !found {
			t.Fatal("expected a post_contents row")
		}
		if title.String != "Hashing 101" {
			t.Errorf("title = %q, want Hashing 101", title.String)
		}
		if thumbnail.String != "https://example.com/thumb.png" {
			t.Errorf("thumbnail = %q, want the given thumbnail", thumbnail.String)
		}
		if content.String != raw {
			t.Errorf("content = %q, want the full raw content (frontmatter preserved)", content.String)
		}

		if len(indexer.calls) != 1 {
			t.Fatalf("len(indexer.calls) = %d, want 1", len(indexer.calls))
		}
		call := indexer.calls[0]
		if call.lang != "en" || call.path != "cryptography-101/hashing" {
			t.Errorf("indexed lang/path = %q/%q, want en/cryptography-101/hashing", call.lang, call.path)
		}
		if call.title != "Hashing 101" || call.description != "a post about hashing" {
			t.Errorf("indexed title/description = %q/%q, want the frontmatter values", call.title, call.description)
		}
		if len(call.tags) != 2 || call.tags[0] != "crypto" || call.tags[1] != "hashing" {
			t.Errorf("indexed tags = %v, want [crypto hashing]", call.tags)
		}
		if call.content != "Hashing\nSome bold body content." {
			t.Errorf("indexed content = %q, want markdown-stripped body", call.content)
		}
	})

	t.Run("handles content with no frontmatter", func(t *testing.T) {
		db := setupTestDB(t)
		postSvc := services.NewPostService(db, notifservices.NewNotificationService(db))
		adminSvc := services.NewAdminPostService(db, postSvc, &fakeIndexer{}, &fakeTagChecker{})

		err := adminSvc.Publish(services.PublishInput{
			Path:       "cryptography-101/no-frontmatter",
			Lang:       "en",
			RawContent: "Just a body, no frontmatter.\n",
		})
		if err != nil {
			t.Fatal(err)
		}

		_, _, content, found := getPostContent(t, db, "en", "cryptography-101/no-frontmatter.md")
		if !found {
			t.Fatal("expected a post_contents row")
		}
		if content.String != "Just a body, no frontmatter.\n" {
			t.Errorf("content = %q, want the original content unchanged", content.String)
		}
	})

	t.Run("re-publishing the same path/lang updates the existing row instead of duplicating", func(t *testing.T) {
		db := setupTestDB(t)
		postSvc := services.NewPostService(db, notifservices.NewNotificationService(db))
		adminSvc := services.NewAdminPostService(db, postSvc, &fakeIndexer{}, &fakeTagChecker{})

		v1 := "---\ntitle: V1\n---\nv1 content\n"
		v2 := "---\ntitle: V2\n---\nv2 content\n"

		input := services.PublishInput{Path: "cryptography-101/republish", Lang: "en", RawContent: v1}
		if err := adminSvc.Publish(input); err != nil {
			t.Fatal(err)
		}
		input.RawContent = v2
		if err := adminSvc.Publish(input); err != nil {
			t.Fatal(err)
		}

		var count int
		if err := db.QueryRow(
			"SELECT COUNT(*) FROM post_contents WHERE lang = 'en' AND path = 'cryptography-101/republish.md'",
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("post_contents row count = %d, want 1 (no duplicate)", count)
		}

		title, _, content, _ := getPostContent(t, db, "en", "cryptography-101/republish.md")
		if title.String != "V2" || content.String != v2 {
			t.Errorf("title/content = %q/%q, want V2/%q (updated, frontmatter preserved)", title.String, content.String, v2)
		}
	})

	t.Run("reuses the same posts row across languages", func(t *testing.T) {
		db := setupTestDB(t)
		postSvc := services.NewPostService(db, notifservices.NewNotificationService(db))
		adminSvc := services.NewAdminPostService(db, postSvc, &fakeIndexer{}, &fakeTagChecker{})

		en := services.PublishInput{Path: "cryptography-101/multilang", Lang: "en", RawContent: "---\ntitle: EN\n---\nen content\n"}
		es := services.PublishInput{Path: "cryptography-101/multilang", Lang: "es", RawContent: "---\ntitle: ES\n---\nes content\n"}
		if err := adminSvc.Publish(en); err != nil {
			t.Fatal(err)
		}
		if err := adminSvc.Publish(es); err != nil {
			t.Fatal(err)
		}

		var postCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM posts WHERE path_identifier = 'cryptography-101/multilang'").Scan(&postCount); err != nil {
			t.Fatal(err)
		}
		if postCount != 1 {
			t.Errorf("posts row count = %d, want 1 (shared across languages)", postCount)
		}
	})

	t.Run("propagates an indexing failure", func(t *testing.T) {
		db := setupTestDB(t)
		postSvc := services.NewPostService(db, notifservices.NewNotificationService(db))
		indexer := &fakeIndexer{failErr: errors.New("meilisearch down")}
		adminSvc := services.NewAdminPostService(db, postSvc, indexer, &fakeTagChecker{})

		err := adminSvc.Publish(services.PublishInput{Path: "cryptography-101/fails", Lang: "en", RawContent: "---\ntitle: T\n---\nC\n"})
		if err == nil {
			t.Fatal("expected an error when indexing fails")
		}
	})

	t.Run("propagates a frontmatter parse failure", func(t *testing.T) {
		db := setupTestDB(t)
		postSvc := services.NewPostService(db, notifservices.NewNotificationService(db))
		adminSvc := services.NewAdminPostService(db, postSvc, &fakeIndexer{}, &fakeTagChecker{})

		err := adminSvc.Publish(services.PublishInput{
			Path:       "cryptography-101/bad-frontmatter",
			Lang:       "en",
			RawContent: "---\ntitle: [unterminated\n---\nbody\n",
		})
		if err == nil {
			t.Fatal("expected an error for malformed frontmatter")
		}
	})
}

func TestAdminPostServicePublishTagValidation(t *testing.T) {
	t.Run("rejects publishing when a frontmatter tag is unregistered", func(t *testing.T) {
		db := setupTestDB(t)
		postSvc := services.NewPostService(db, notifservices.NewNotificationService(db))
		tagChecker := &fakeTagChecker{missing: []string{"nonexistentTag"}}
		adminSvc := services.NewAdminPostService(db, postSvc, &fakeIndexer{}, tagChecker)

		err := adminSvc.Publish(services.PublishInput{
			Path:       "cryptography-101/bad-tag",
			Lang:       "en",
			RawContent: "---\ntitle: T\ntags:\n  - nonexistentTag\n---\nbody\n",
		})
		if err == nil {
			t.Fatal("expected an error for an unregistered tag")
		}
		if !errors.Is(err, services.ErrUnknownTags) {
			t.Errorf("err = %v, want ErrUnknownTags", err)
		}

		var count int
		db.QueryRow("SELECT COUNT(*) FROM post_contents WHERE path = 'cryptography-101/bad-tag.md'").Scan(&count)
		if count != 0 {
			t.Errorf("post_contents row count = %d, want 0 (nothing written on validation failure)", count)
		}
	})

	t.Run("publishing with no tags in frontmatter succeeds without querying tag existence", func(t *testing.T) {
		db := setupTestDB(t)
		postSvc := services.NewPostService(db, notifservices.NewNotificationService(db))
		adminSvc := services.NewAdminPostService(db, postSvc, &fakeIndexer{}, &fakeTagChecker{missing: []string{"anything"}})

		err := adminSvc.Publish(services.PublishInput{
			Path:       "cryptography-101/no-tags",
			Lang:       "en",
			RawContent: "---\ntitle: T\n---\nbody\n",
		})
		if err != nil {
			t.Fatalf("expected success (no tags to validate), got: %v", err)
		}
	})

	t.Run("publishing with all tags registered succeeds", func(t *testing.T) {
		db := setupTestDB(t)
		postSvc := services.NewPostService(db, notifservices.NewNotificationService(db))
		adminSvc := services.NewAdminPostService(db, postSvc, &fakeIndexer{}, &fakeTagChecker{})

		err := adminSvc.Publish(services.PublishInput{
			Path:       "cryptography-101/good-tags",
			Lang:       "en",
			RawContent: "---\ntitle: T\ntags:\n  - cryptography\n  - hashing\n---\nbody\n",
		})
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
	})
}
