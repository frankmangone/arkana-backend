package tests

import (
	"errors"
	"testing"

	"arkana/features/readinglists/models"
	"arkana/features/readinglists/services"
)

func samplePayload() models.ReadingListPayload {
	return models.ReadingListPayload{
		Slug:       "blockchain-101",
		CoverImage: "/images/blockchain-101/cover.webp",
		Ongoing:    false,
		Translations: map[string]models.Translation{
			"en": {Title: "Blockchain 101", Description: "Intro to blockchain"},
			"es": {Title: "Blockchain 101", Description: "Introducción a blockchain"},
		},
		Modules: []models.ModulePayload{
			{
				Slug:  "bitcoin-and-fundamentals",
				Order: 1,
				Translations: map[string]models.Translation{
					"en": {Title: "Bitcoin and Fundamentals", Description: "The basics"},
					"es": {Title: "Bitcoin y Fundamentos", Description: "Los fundamentos"},
				},
				Items: []models.ItemPayload{
					{Slug: "how-it-all-began", PostPath: "blockchain-101/how-it-all-began", Order: 1},
					{Slug: "transactions", PostPath: "blockchain-101/transactions", Order: 2},
				},
			},
		},
	}
}

func TestReadingListServicePublish(t *testing.T) {
	t.Run("creates the list, its translations, modules, module translations, and items", func(t *testing.T) {
		db := setupTestDB(t)
		svc := newReadingListService(db)

		if err := svc.Publish(samplePayload()); err != nil {
			t.Fatal(err)
		}

		var listCount, translationCount, moduleCount, moduleTranslationCount, itemCount int
		db.QueryRow("SELECT COUNT(*) FROM reading_lists").Scan(&listCount)
		db.QueryRow("SELECT COUNT(*) FROM reading_list_translations").Scan(&translationCount)
		db.QueryRow("SELECT COUNT(*) FROM reading_list_modules").Scan(&moduleCount)
		db.QueryRow("SELECT COUNT(*) FROM reading_list_module_translations").Scan(&moduleTranslationCount)
		db.QueryRow("SELECT COUNT(*) FROM reading_list_items").Scan(&itemCount)

		if listCount != 1 {
			t.Errorf("reading_lists row count = %d, want 1", listCount)
		}
		if translationCount != 2 {
			t.Errorf("reading_list_translations row count = %d, want 2", translationCount)
		}
		if moduleCount != 1 {
			t.Errorf("reading_list_modules row count = %d, want 1", moduleCount)
		}
		if moduleTranslationCount != 2 {
			t.Errorf("reading_list_module_translations row count = %d, want 2", moduleTranslationCount)
		}
		if itemCount != 2 {
			t.Errorf("reading_list_items row count = %d, want 2", itemCount)
		}
	})

	t.Run("rejects an item with an unregistered post path and writes nothing", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewReadingListService(db, &fakePostChecker{missing: []string{"blockchain-101/nonexistent"}})

		payload := samplePayload()
		payload.Modules[0].Items = append(payload.Modules[0].Items, models.ItemPayload{
			Slug: "nonexistent", PostPath: "blockchain-101/nonexistent", Order: 3,
		})

		err := svc.Publish(payload)
		if err == nil {
			t.Fatal("expected an error for an unregistered post path")
		}
		if !errors.Is(err, services.ErrUnknownPosts) {
			t.Errorf("err = %v, want ErrUnknownPosts", err)
		}

		var listCount int
		db.QueryRow("SELECT COUNT(*) FROM reading_lists").Scan(&listCount)
		if listCount != 0 {
			t.Errorf("reading_lists row count = %d, want 0 (nothing written on validation failure)", listCount)
		}
	})

	t.Run("re-publishing the same slug with a changed translation updates in place, no duplicates", func(t *testing.T) {
		db := setupTestDB(t)
		svc := newReadingListService(db)
		svc.Publish(samplePayload())

		updated := samplePayload()
		en := updated.Translations["en"]
		en.Description = "Updated description"
		updated.Translations["en"] = en
		if err := svc.Publish(updated); err != nil {
			t.Fatal(err)
		}

		var listCount int
		db.QueryRow("SELECT COUNT(*) FROM reading_lists").Scan(&listCount)
		if listCount != 1 {
			t.Errorf("reading_lists row count = %d, want 1 (no duplicate)", listCount)
		}

		var description string
		db.QueryRow(
			`SELECT rlt.description FROM reading_list_translations rlt JOIN reading_lists rl ON rl.id = rlt.reading_list_id WHERE rl.slug = ? AND rlt.lang = ?`,
			"blockchain-101", "en",
		).Scan(&description)
		if description != "Updated description" {
			t.Errorf("description = %q, want %q", description, "Updated description")
		}
	})

	t.Run("re-publishing with a module removed deletes that module and its items", func(t *testing.T) {
		db := setupTestDB(t)
		svc := newReadingListService(db)
		svc.Publish(samplePayload())

		emptied := samplePayload()
		emptied.Modules = nil
		if err := svc.Publish(emptied); err != nil {
			t.Fatal(err)
		}

		var moduleCount, itemCount int
		db.QueryRow("SELECT COUNT(*) FROM reading_list_modules").Scan(&moduleCount)
		db.QueryRow("SELECT COUNT(*) FROM reading_list_items").Scan(&itemCount)
		if moduleCount != 0 {
			t.Errorf("reading_list_modules row count = %d, want 0", moduleCount)
		}
		if itemCount != 0 {
			t.Errorf("reading_list_items row count = %d, want 0", itemCount)
		}
	})

	t.Run("re-publishing with an item reordered updates position without duplicating", func(t *testing.T) {
		db := setupTestDB(t)
		svc := newReadingListService(db)
		svc.Publish(samplePayload())

		reordered := samplePayload()
		reordered.Modules[0].Items[0].Order = 5
		if err := svc.Publish(reordered); err != nil {
			t.Fatal(err)
		}

		var itemCount, position int
		db.QueryRow("SELECT COUNT(*) FROM reading_list_items").Scan(&itemCount)
		if itemCount != 2 {
			t.Errorf("reading_list_items row count = %d, want 2 (no duplicate)", itemCount)
		}
		db.QueryRow("SELECT position FROM reading_list_items WHERE slug = ?", "how-it-all-began").Scan(&position)
		if position != 5 {
			t.Errorf("position = %d, want 5", position)
		}
	})
}

func TestReadingListServiceListAll(t *testing.T) {
	t.Run("returns lists ordered by slug, modules and items ordered by position, full translation maps", func(t *testing.T) {
		db := setupTestDB(t)
		svc := newReadingListService(db)
		svc.Publish(samplePayload())

		second := models.ReadingListPayload{
			Slug: "cryptography-101",
			Translations: map[string]models.Translation{
				"en": {Title: "Cryptography 101", Description: "Intro to crypto"},
			},
			Modules: []models.ModulePayload{
				{
					Slug:  "hashing-basics",
					Order: 1,
					Translations: map[string]models.Translation{
						"en": {Title: "Hashing Basics", Description: "Hash functions"},
					},
					Items: []models.ItemPayload{
						{Slug: "hashing", PostPath: "cryptography-101/hashing", Order: 1},
					},
				},
			},
		}
		svc.Publish(second)

		lists, err := svc.ListAll()
		if err != nil {
			t.Fatal(err)
		}
		if len(lists) != 2 {
			t.Fatalf("len(lists) = %d, want 2", len(lists))
		}
		if lists[0].Slug != "blockchain-101" || lists[1].Slug != "cryptography-101" {
			t.Errorf("order = [%s, %s], want [blockchain-101, cryptography-101]", lists[0].Slug, lists[1].Slug)
		}
		if lists[0].Translations["es"].Title != "Blockchain 101" {
			t.Errorf("es title = %q, want Blockchain 101", lists[0].Translations["es"].Title)
		}
		if len(lists[0].Modules) != 1 || len(lists[0].Modules[0].Items) != 2 {
			t.Fatalf("lists[0] modules/items = %+v, want 1 module with 2 items", lists[0].Modules)
		}
		if lists[0].Modules[0].Items[0].Slug != "how-it-all-began" || lists[0].Modules[0].Items[1].Slug != "transactions" {
			t.Errorf("item order = [%s, %s], want [how-it-all-began, transactions]",
				lists[0].Modules[0].Items[0].Slug, lists[0].Modules[0].Items[1].Slug)
		}
	})

	t.Run("returns an empty slice, not nil, when there are no reading lists", func(t *testing.T) {
		db := setupTestDB(t)
		svc := newReadingListService(db)

		lists, err := svc.ListAll()
		if err != nil {
			t.Fatal(err)
		}
		if lists == nil {
			t.Error("lists = nil, want an empty (non-nil) slice")
		}
		if len(lists) != 0 {
			t.Errorf("len(lists) = %d, want 0", len(lists))
		}
	})
}
