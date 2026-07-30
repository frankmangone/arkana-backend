package tests

import (
	"testing"

	"arkana/features/tags/models"
)

func TestTagServiceSync(t *testing.T) {
	t.Run("creates tags and their translations", func(t *testing.T) {
		db := setupTestDB(t)
		svc := newTagService(db)

		n, err := svc.Sync([]models.TagPayload{
			{Slug: "cryptography", Translations: map[string]string{"en": "Cryptography", "es": "Criptografía", "pt": "Criptografia"}},
			{Slug: "hashing", Translations: map[string]string{"en": "Hashing", "es": "Hashing", "pt": "Hashing"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if n != 2 {
			t.Errorf("synced = %d, want 2", n)
		}

		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM tags").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Errorf("tags row count = %d, want 2", count)
		}

		var name string
		if err := db.QueryRow(
			`SELECT tt.name FROM tag_translations tt JOIN tags t ON t.id = tt.tag_id WHERE t.slug = ? AND tt.lang = ?`,
			"cryptography", "es",
		).Scan(&name); err != nil {
			t.Fatal(err)
		}
		if name != "Criptografía" {
			t.Errorf("es translation = %q, want Criptografía", name)
		}
	})

	t.Run("re-syncing with a changed translation updates in place, no duplicates", func(t *testing.T) {
		db := setupTestDB(t)
		svc := newTagService(db)

		svc.Sync([]models.TagPayload{
			{Slug: "rollup", Translations: map[string]string{"en": "Rollup"}},
		})
		svc.Sync([]models.TagPayload{
			{Slug: "rollup", Translations: map[string]string{"en": "Rollup (updated)"}},
		})

		var tagCount int
		db.QueryRow("SELECT COUNT(*) FROM tags").Scan(&tagCount)
		if tagCount != 1 {
			t.Errorf("tags row count = %d, want 1 (no duplicate)", tagCount)
		}

		var translationCount int
		db.QueryRow("SELECT COUNT(*) FROM tag_translations").Scan(&translationCount)
		if translationCount != 1 {
			t.Errorf("tag_translations row count = %d, want 1 (updated, not duplicated)", translationCount)
		}

		var name string
		db.QueryRow(
			`SELECT tt.name FROM tag_translations tt JOIN tags t ON t.id = tt.tag_id WHERE t.slug = ?`,
			"rollup",
		).Scan(&name)
		if name != "Rollup (updated)" {
			t.Errorf("name = %q, want Rollup (updated)", name)
		}
	})

	t.Run("a tag omitted from a later sync is left untouched, not deleted", func(t *testing.T) {
		db := setupTestDB(t)
		svc := newTagService(db)

		svc.Sync([]models.TagPayload{
			{Slug: "bitcoin", Translations: map[string]string{"en": "Bitcoin"}},
			{Slug: "ethereum", Translations: map[string]string{"en": "Ethereum"}},
		})
		// Second sync omits "ethereum" entirely.
		svc.Sync([]models.TagPayload{
			{Slug: "bitcoin", Translations: map[string]string{"en": "Bitcoin"}},
		})

		var count int
		db.QueryRow("SELECT COUNT(*) FROM tags").Scan(&count)
		if count != 2 {
			t.Errorf("tags row count = %d, want 2 (ethereum must survive, not be deleted)", count)
		}
	})
}

func TestTagServiceList(t *testing.T) {
	t.Run("returns tags ordered by slug with full translation maps", func(t *testing.T) {
		db := setupTestDB(t)
		svc := newTagService(db)
		svc.Sync([]models.TagPayload{
			{Slug: "zeroKnowledgeProofs", Translations: map[string]string{"en": "Zero Knowledge Proofs", "es": "Pruebas de Conocimiento Cero"}},
			{Slug: "bitcoin", Translations: map[string]string{"en": "Bitcoin"}},
		})

		tags, err := svc.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(tags) != 2 {
			t.Fatalf("len(tags) = %d, want 2", len(tags))
		}
		if tags[0].Slug != "bitcoin" || tags[1].Slug != "zeroKnowledgeProofs" {
			t.Errorf("order = [%s, %s], want [bitcoin, zeroKnowledgeProofs]", tags[0].Slug, tags[1].Slug)
		}
		if tags[1].Translations["es"] != "Pruebas de Conocimiento Cero" {
			t.Errorf("es translation = %q, want Pruebas de Conocimiento Cero", tags[1].Translations["es"])
		}
	})

	t.Run("returns an empty slice, not nil, when there are no tags", func(t *testing.T) {
		db := setupTestDB(t)
		svc := newTagService(db)

		tags, err := svc.List()
		if err != nil {
			t.Fatal(err)
		}
		if tags == nil {
			t.Error("tags = nil, want an empty (non-nil) slice")
		}
		if len(tags) != 0 {
			t.Errorf("len(tags) = %d, want 0", len(tags))
		}
	})
}

func TestTagServiceMissingTags(t *testing.T) {
	t.Run("returns nil for an empty input", func(t *testing.T) {
		db := setupTestDB(t)
		svc := newTagService(db)

		missing, err := svc.MissingTags(nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(missing) != 0 {
			t.Errorf("missing = %v, want empty", missing)
		}
	})

	t.Run("returns only the slugs not present in tags", func(t *testing.T) {
		db := setupTestDB(t)
		svc := newTagService(db)
		svc.Sync([]models.TagPayload{
			{Slug: "cryptography", Translations: map[string]string{"en": "Cryptography"}},
		})

		missing, err := svc.MissingTags([]string{"cryptography", "nonexistentTag"})
		if err != nil {
			t.Fatal(err)
		}
		if len(missing) != 1 || missing[0] != "nonexistentTag" {
			t.Errorf("missing = %v, want [nonexistentTag]", missing)
		}
	})

	t.Run("returns empty when every slug is registered", func(t *testing.T) {
		db := setupTestDB(t)
		svc := newTagService(db)
		svc.Sync([]models.TagPayload{
			{Slug: "bitcoin", Translations: map[string]string{"en": "Bitcoin"}},
			{Slug: "ethereum", Translations: map[string]string{"en": "Ethereum"}},
		})

		missing, err := svc.MissingTags([]string{"bitcoin", "ethereum"})
		if err != nil {
			t.Fatal(err)
		}
		if len(missing) != 0 {
			t.Errorf("missing = %v, want empty", missing)
		}
	})
}

func TestGetIDsBySlugs(t *testing.T) {
	db := setupTestDB(t)
	svc := newTagService(db)

	t.Run("returns an empty map for an empty input", func(t *testing.T) {
		ids, err := svc.GetIDsBySlugs(nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) != 0 {
			t.Errorf("ids = %v, want empty", ids)
		}
	})

	t.Run("resolves known slugs and omits unknown ones", func(t *testing.T) {
		if _, err := svc.Sync([]models.TagPayload{
			{Slug: "cryptography", Translations: map[string]string{"en": "Cryptography"}},
		}); err != nil {
			t.Fatal(err)
		}

		ids, err := svc.GetIDsBySlugs([]string{"cryptography", "nonexistent"})
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) != 1 {
			t.Fatalf("ids = %v, want exactly 1 entry", ids)
		}
		if _, ok := ids["cryptography"]; !ok {
			t.Errorf("ids[cryptography] missing")
		}
	})
}
