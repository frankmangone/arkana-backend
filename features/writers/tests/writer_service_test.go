package tests

import (
	"errors"
	"testing"

	"arkana/features/writers/services"
)

func TestWriterServiceGetBySlug(t *testing.T) {
	t.Run("returns the full profile for a visible writer", func(t *testing.T) {
		db := setupTestDB(t)
		insertTestWriter(t, db, "frank-mangone", "Frank Mangone", true)
		svc := services.NewWriterService(db)

		writer, err := svc.GetBySlug("frank-mangone")
		if err != nil {
			t.Fatal(err)
		}
		if writer.Name != "Frank Mangone" {
			t.Errorf("name = %q, want Frank Mangone", writer.Name)
		}
		if writer.Organization == nil || writer.Organization.Name != "SpaceDev" {
			t.Errorf("organization = %+v, want SpaceDev", writer.Organization)
		}
		if writer.Bio["en"] != "An English bio." {
			t.Errorf("bio[en] = %q, want An English bio.", writer.Bio["en"])
		}
		if writer.Social == nil || writer.Social.Twitter != "https://x.com/example" {
			t.Errorf("social = %+v, want twitter set", writer.Social)
		}
		if writer.WalletAddress != "0xWALLET" {
			t.Errorf("wallet_address = %q, want 0xWALLET", writer.WalletAddress)
		}
	})

	t.Run("returns ErrWriterNotFound for an unknown slug", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewWriterService(db)

		_, err := svc.GetBySlug("nonexistent")
		if !errors.Is(err, services.ErrWriterNotFound) {
			t.Errorf("err = %v, want ErrWriterNotFound", err)
		}
	})

	t.Run("returns ErrWriterNotFound for a non-visible writer", func(t *testing.T) {
		db := setupTestDB(t)
		insertTestWriter(t, db, "hidden-writer", "Hidden Writer", false)
		svc := services.NewWriterService(db)

		_, err := svc.GetBySlug("hidden-writer")
		if !errors.Is(err, services.ErrWriterNotFound) {
			t.Errorf("err = %v, want ErrWriterNotFound", err)
		}
	})

	t.Run("returns empty strings for missing optional image fields instead of erroring", func(t *testing.T) {
		db := setupTestDB(t)
		_, err := db.Exec(
			`INSERT INTO writers (slug, name, visible) VALUES (?, ?, 1)`,
			"partial-writer", "Partial Writer",
		)
		if err != nil {
			t.Fatal(err)
		}
		svc := services.NewWriterService(db)

		writer, err := svc.GetBySlug("partial-writer")
		if err != nil {
			t.Fatal(err)
		}
		if writer.ImageURL != "" {
			t.Errorf("ImageURL = %q, want empty string", writer.ImageURL)
		}
		if writer.AvatarURL != "" {
			t.Errorf("AvatarURL = %q, want empty string", writer.AvatarURL)
		}
	})
}

func TestWriterServiceList(t *testing.T) {
	t.Run("returns only visible writers ordered by name", func(t *testing.T) {
		db := setupTestDB(t)
		insertTestWriter(t, db, "zed-writer", "Zed Writer", true)
		insertTestWriter(t, db, "anna-writer", "Anna Writer", true)
		insertTestWriter(t, db, "hidden-writer", "Hidden Writer", false)
		svc := services.NewWriterService(db)

		writers, err := svc.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(writers) != 2 {
			t.Fatalf("len(writers) = %d, want 2", len(writers))
		}
		if writers[0].Slug != "anna-writer" || writers[1].Slug != "zed-writer" {
			t.Errorf("order = [%s, %s], want [anna-writer, zed-writer]", writers[0].Slug, writers[1].Slug)
		}
	})

	t.Run("returns an empty slice, not nil, when there are no visible writers", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewWriterService(db)

		writers, err := svc.List()
		if err != nil {
			t.Fatal(err)
		}
		if writers == nil {
			t.Error("writers = nil, want an empty (non-nil) slice")
		}
		if len(writers) != 0 {
			t.Errorf("len(writers) = %d, want 0", len(writers))
		}
	})

	t.Run("excludes legacy writer rows that have no slug", func(t *testing.T) {
		db := setupTestDB(t)
		_, err := db.Exec(`INSERT INTO writers (name, user_id) VALUES (?, ?)`, "Legacy Writer", 7)
		if err != nil {
			t.Fatal(err)
		}
		insertTestWriter(t, db, "normal-writer", "Normal Writer", true)
		svc := services.NewWriterService(db)

		writers, err := svc.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(writers) != 1 {
			t.Fatalf("len(writers) = %d, want 1", len(writers))
		}
		if writers[0].Name != "Normal Writer" {
			t.Errorf("writers[0].Name = %q, want Normal Writer", writers[0].Name)
		}
	})
}
