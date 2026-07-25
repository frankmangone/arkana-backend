package tests

import (
	"database/sql"
	"testing"

	"arkana/features/writers/models"
	"arkana/features/writers/services"
)

func getWriterRow(t *testing.T, db *sql.DB, slug string) (name, imageURL, avatarURL string, visible bool, organization, bio, social, walletAddress sql.NullString, found bool) {
	t.Helper()
	err := db.QueryRow(
		`SELECT name, image_url, avatar_url, visible, organization, bio, social, wallet_address
		 FROM writers WHERE slug = ?`,
		slug,
	).Scan(&name, &imageURL, &avatarURL, &visible, &organization, &bio, &social, &walletAddress)
	if err == sql.ErrNoRows {
		return "", "", "", false, sql.NullString{}, sql.NullString{}, sql.NullString{}, sql.NullString{}, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return name, imageURL, avatarURL, visible, organization, bio, social, walletAddress, true
}

func TestAdminWriterServicePublish(t *testing.T) {
	t.Run("creates a new writer with all fields", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewAdminWriterService(db)

		visible := true
		err := svc.Publish(models.WriterPayload{
			Slug:      "frank-mangone",
			Name:      "Frank Mangone",
			ImageURL:  "/images/writers/frank-mangone/full-size.png",
			AvatarURL: "/images/writers/frank-mangone/avatar.png",
			Visible:   &visible,
			Organization: &models.Organization{
				Name: "SpaceDev",
				URL:  "https://spacedev.io",
			},
			Bio: map[string]string{
				"en": "Creator of Arkana.",
				"es": "Creador de Arkana.",
			},
			Social: &models.Social{
				Twitter: "https://x.com/0xfrankmangone",
				GitHub:  "https://github.com/frankmangone",
			},
			WalletAddress: "0x1433e4b5349367a5870caeb4d4a2b89d1bd02754",
		})
		if err != nil {
			t.Fatal(err)
		}

		name, imageURL, avatarURL, visibleGot, organization, bio, social, wallet, found := getWriterRow(t, db, "frank-mangone")
		if !found {
			t.Fatal("expected a writers row")
		}
		if name != "Frank Mangone" || imageURL != "/images/writers/frank-mangone/full-size.png" || avatarURL != "/images/writers/frank-mangone/avatar.png" {
			t.Errorf("name/image_url/avatar_url = %q/%q/%q, want the given values", name, imageURL, avatarURL)
		}
		if !visibleGot {
			t.Error("visible = false, want true")
		}
		if organization.String != `{"name":"SpaceDev","url":"https://spacedev.io"}` {
			t.Errorf("organization = %q, want the marshaled organization JSON", organization.String)
		}
		if bio.String != `{"en":"Creator of Arkana.","es":"Creador de Arkana."}` {
			t.Errorf("bio = %q, want the marshaled bio JSON", bio.String)
		}
		if social.String != `{"twitter":"https://x.com/0xfrankmangone","github":"https://github.com/frankmangone"}` {
			t.Errorf("social = %q, want the marshaled social JSON", social.String)
		}
		if wallet.String != "0x1433e4b5349367a5870caeb4d4a2b89d1bd02754" {
			t.Errorf("wallet_address = %q, want the given wallet address", wallet.String)
		}
	})

	t.Run("omitting optional fields defaults visible to true and leaves JSON columns NULL", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewAdminWriterService(db)

		err := svc.Publish(models.WriterPayload{
			Slug:      "minimal-writer",
			Name:      "Minimal Writer",
			ImageURL:  "/img.png",
			AvatarURL: "/avatar.png",
		})
		if err != nil {
			t.Fatal(err)
		}

		_, _, _, visible, organization, bio, social, wallet, found := getWriterRow(t, db, "minimal-writer")
		if !found {
			t.Fatal("expected a writers row")
		}
		if !visible {
			t.Error("visible = false, want true (default)")
		}
		if organization.Valid || bio.Valid || social.Valid || wallet.Valid {
			t.Errorf("organization/bio/social/wallet = %v/%v/%v/%v, want all NULL", organization, bio, social, wallet)
		}
	})

	t.Run("re-publishing the same slug updates the existing row instead of duplicating", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewAdminWriterService(db)

		v1 := models.WriterPayload{Slug: "republish-writer", Name: "V1", ImageURL: "/img1.png", AvatarURL: "/avatar1.png"}
		v2 := models.WriterPayload{Slug: "republish-writer", Name: "V2", ImageURL: "/img2.png", AvatarURL: "/avatar2.png"}

		if err := svc.Publish(v1); err != nil {
			t.Fatal(err)
		}
		if err := svc.Publish(v2); err != nil {
			t.Fatal(err)
		}

		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM writers WHERE slug = 'republish-writer'").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("writers row count = %d, want 1 (no duplicate)", count)
		}

		name, imageURL, _, _, _, _, _, _, _ := getWriterRow(t, db, "republish-writer")
		if name != "V2" || imageURL != "/img2.png" {
			t.Errorf("name/image_url = %q/%q, want V2//img2.png (updated)", name, imageURL)
		}
	})

	t.Run("preserves user_id across republish since Publish never touches it", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewAdminWriterService(db)

		if _, err := db.Exec("INSERT INTO writers (slug, name, user_id) VALUES (?, ?, ?)", "existing-writer", "Old Name", 42); err != nil {
			t.Fatal(err)
		}

		err := svc.Publish(models.WriterPayload{
			Slug:      "existing-writer",
			Name:      "New Name",
			ImageURL:  "/img.png",
			AvatarURL: "/avatar.png",
		})
		if err != nil {
			t.Fatal(err)
		}

		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM writers WHERE slug = 'existing-writer'").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("writers row count = %d, want 1 (no duplicate)", count)
		}

		var name string
		var userID sql.NullInt64
		if err := db.QueryRow("SELECT name, user_id FROM writers WHERE slug = 'existing-writer'").Scan(&name, &userID); err != nil {
			t.Fatal(err)
		}
		if name != "New Name" {
			t.Errorf("name = %q, want New Name (updated)", name)
		}
		if !userID.Valid || userID.Int64 != 42 {
			t.Errorf("user_id = %v, want 42 (preserved)", userID)
		}
	})
}
