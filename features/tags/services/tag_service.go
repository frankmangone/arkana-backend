package services

import (
	"database/sql"
	"fmt"
	"strings"

	"arkana/features/tags/models"
)

type TagService struct {
	db *sql.DB
}

func NewTagService(db *sql.DB) *TagService {
	return &TagService{db: db}
}

// Sync upserts every tag and its translations in payloads within a single
// transaction. Add/update only - a tag or translation absent from
// payloads is left untouched, never deleted, so removing a tag from the
// source file can never orphan a post that still references it.
func (s *TagService) Sync(payloads []models.TagPayload) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	for _, p := range payloads {
		if _, err := tx.Exec(
			`INSERT INTO tags (slug) VALUES (?)
			 ON CONFLICT(slug) DO UPDATE SET updated_at = CURRENT_TIMESTAMP`,
			p.Slug,
		); err != nil {
			return 0, err
		}

		var tagID int64
		if err := tx.QueryRow("SELECT id FROM tags WHERE slug = ?", p.Slug).Scan(&tagID); err != nil {
			return 0, err
		}

		for lang, name := range p.Translations {
			if _, err := tx.Exec(
				`INSERT INTO tag_translations (tag_id, lang, name) VALUES (?, ?, ?)
				 ON CONFLICT(tag_id, lang) DO UPDATE SET name = excluded.name`,
				tagID, lang, name,
			); err != nil {
				return 0, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(payloads), nil
}

// List returns every tag with its full translation map, ordered by slug.
// LEFT JOIN (not INNER) so a tag with zero translations still appears
// with an empty map, rather than vanishing silently.
func (s *TagService) List() ([]models.TagResponse, error) {
	rows, err := s.db.Query(
		`SELECT t.slug, tt.lang, tt.name
		 FROM tags t
		 LEFT JOIN tag_translations tt ON tt.tag_id = t.id
		 ORDER BY t.slug`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bySlug := map[string]*models.TagResponse{}
	var order []string
	for rows.Next() {
		var slug string
		var lang, name sql.NullString
		if err := rows.Scan(&slug, &lang, &name); err != nil {
			return nil, err
		}
		entry, ok := bySlug[slug]
		if !ok {
			entry = &models.TagResponse{Slug: slug, Translations: map[string]string{}}
			bySlug[slug] = entry
			order = append(order, slug)
		}
		if lang.Valid && name.Valid {
			entry.Translations[lang.String] = name.String
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]models.TagResponse, 0, len(order))
	for _, slug := range order {
		result = append(result, *bySlug[slug])
	}
	return result, nil
}

// MissingTags returns the subset of slugs that have no row in tags, for
// publish-time validation. Returns nil for an empty input without
// querying.
func (s *TagService) MissingTags(slugs []string) ([]string, error) {
	if len(slugs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(slugs))
	args := make([]interface{}, len(slugs))
	for i, slug := range slugs {
		placeholders[i] = "?"
		args[i] = slug
	}

	rows, err := s.db.Query(
		fmt.Sprintf("SELECT slug FROM tags WHERE slug IN (%s)", strings.Join(placeholders, ",")),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	found := make(map[string]bool, len(slugs))
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		found[slug] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var missing []string
	for _, slug := range slugs {
		if !found[slug] {
			missing = append(missing, slug)
		}
	}
	return missing, nil
}
