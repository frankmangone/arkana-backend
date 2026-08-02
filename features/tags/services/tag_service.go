package services

import (
	"arkana/features/tags/models"
	"arkana/features/tags/queries"
	"database/sql"
)

type TagService struct {
	db      *sql.DB
	queries queries.TagQueries
}

func NewTagService(db *sql.DB) *TagService {
	return &TagService{db: db, queries: queries.NewSQLTagQueries(db)}
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

	qtx := s.queries.WithTx(tx)
	n, err := qtx.Sync(payloads)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

// List returns every tag with its full translation map, ordered by slug.
func (s *TagService) List() ([]models.TagResponse, error) {
	return s.queries.List()
}

// MissingTags returns the subset of slugs that have no row in tags, for
// publish-time validation.
func (s *TagService) MissingTags(slugs []string) ([]string, error) {
	return s.queries.MissingTags(slugs)
}

// GetIDsBySlugs returns a map of slug -> tags.id for every slug that has a
// matching row.
func (s *TagService) GetIDsBySlugs(slugs []string) (map[string]int, error) {
	return s.queries.GetIDsBySlugs(slugs)
}
