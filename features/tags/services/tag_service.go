// Package services implements the business logic for tags. Its main type,
// TagService, wraps a queries.TagQueries to sync tags and their translations
// from a source payload, list all tags, and look up tags by slug - keeping
// transaction handling and query details out of the HTTP handler layer that
// wires it into the rest of the app.
package services

import (
	"arkana/features/tags/models"
	"arkana/features/tags/queries"
	dbpkg "arkana/shared/db"
	"database/sql"
)

type TagService struct {
	db      *sql.DB
	queries queries.TagQueries
}

// NewTagService constructs a TagService backed by db, wiring it to the
// default SQL-backed TagQueries implementation.
func NewTagService(db *sql.DB) *TagService {
	return &TagService{db: db, queries: queries.NewSQLTagQueries(db)}
}

// Sync upserts every tag and its translations in payloads within a single
// transaction. Add/update only - a tag or translation absent from
// payloads is left untouched, never deleted, so removing a tag from the
// source file can never orphan a post that still references it.
func (s *TagService) Sync(payloads []models.TagPayload) (int, error) {
	var n int
	err := dbpkg.Transact(s.db, func(tx *sql.Tx) error {
		qtx := s.queries.WithTx(tx)
		synced, err := qtx.Sync(payloads)
		if err != nil {
			return err
		}
		n = synced
		return nil
	})
	if err != nil {
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
