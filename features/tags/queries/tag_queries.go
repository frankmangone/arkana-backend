package queries

import (
	"database/sql"
	"fmt"
	"strings"

	dbpkg "arkana/shared/db"
	"arkana/features/tags/models"
)

type TagQueries interface {
	List() ([]models.TagResponse, error)
	Sync(payloads []models.TagPayload) (int, error)
	MissingTags(slugs []string) ([]string, error)
	GetIDsBySlugs(slugs []string) (map[string]int, error)
	WithTx(tx *sql.Tx) TagQueries
}

type SQLTagQueries struct {
	db dbpkg.DBTX
}

func NewSQLTagQueries(db dbpkg.DBTX) *SQLTagQueries {
	return &SQLTagQueries{db: db}
}

func (q *SQLTagQueries) WithTx(tx *sql.Tx) TagQueries {
	return NewSQLTagQueries(tx)
}

// UpsertTag inserts a tag by slug, or bumps updated_at if it already exists.
func (q *SQLTagQueries) UpsertTag(slug string) error {
	_, err := q.db.Exec(
		`INSERT INTO tags (slug) VALUES (?)
		 ON CONFLICT(slug) DO UPDATE SET updated_at = CURRENT_TIMESTAMP`,
		slug,
	)
	return err
}

// GetTagIDBySlug returns the id of the tag with the given slug.
func (q *SQLTagQueries) GetTagIDBySlug(slug string) (int64, error) {
	var tagID int64
	err := q.db.QueryRow("SELECT id FROM tags WHERE slug = ?", slug).Scan(&tagID)
	return tagID, err
}

// UpsertTranslation inserts or updates one (tag, lang) translation row.
func (q *SQLTagQueries) UpsertTranslation(tagID int64, lang, name string) error {
	_, err := q.db.Exec(
		`INSERT INTO tag_translations (tag_id, lang, name) VALUES (?, ?, ?)
		 ON CONFLICT(tag_id, lang) DO UPDATE SET name = excluded.name`,
		tagID, lang, name,
	)
	return err
}

// Sync upserts every tag and its translations in payloads within a single
// transaction. Add/update only - a tag or translation absent from
// payloads is left untouched, never deleted, so removing a tag from the
// source file can never orphan a post that still references it.
//
// Called via WithTx from TagService.Sync, which owns the transaction.
func (q *SQLTagQueries) Sync(payloads []models.TagPayload) (int, error) {
	for _, p := range payloads {
		if err := q.UpsertTag(p.Slug); err != nil {
			return 0, err
		}
		tagID, err := q.GetTagIDBySlug(p.Slug)
		if err != nil {
			return 0, err
		}
		for lang, name := range p.Translations {
			if err := q.UpsertTranslation(tagID, lang, name); err != nil {
				return 0, err
			}
		}
	}
	return len(payloads), nil
}

// List returns every tag with its full translation map, ordered by slug.
// LEFT JOIN (not INNER) so a tag with zero translations still appears
// with an empty map, rather than vanishing silently.
func (q *SQLTagQueries) List() ([]models.TagResponse, error) {
	rows, err := q.db.Query(
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
func (q *SQLTagQueries) MissingTags(slugs []string) ([]string, error) {
	if len(slugs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(slugs))
	args := make([]interface{}, len(slugs))
	for i, slug := range slugs {
		placeholders[i] = "?"
		args[i] = slug
	}

	rows, err := q.db.Query(
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

// GetIDsBySlugs returns a map of slug -> tags.id for every slug that has a
// matching row; slugs with no match are simply absent.
func (q *SQLTagQueries) GetIDsBySlugs(slugs []string) (map[string]int, error) {
	if len(slugs) == 0 {
		return map[string]int{}, nil
	}

	placeholders := make([]string, len(slugs))
	args := make([]interface{}, len(slugs))
	for i, slug := range slugs {
		placeholders[i] = "?"
		args[i] = slug
	}

	rows, err := q.db.Query(
		fmt.Sprintf("SELECT id, slug FROM tags WHERE slug IN (%s)", strings.Join(placeholders, ",")),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int, len(slugs))
	for rows.Next() {
		var id int
		var slug string
		if err := rows.Scan(&id, &slug); err != nil {
			return nil, err
		}
		result[slug] = id
	}
	return result, rows.Err()
}
