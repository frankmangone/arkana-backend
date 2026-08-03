package services

import (
	"arkana/features/readinglists/models"
	"arkana/features/readinglists/queries"
	dbpkg "arkana/shared/db"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrUnknownPosts = errors.New("unknown post path(s)")

// PostChecker validates that a set of post paths correspond to real posts.
// Satisfied structurally by *posts/services.PostService, without this
// package depending on it directly.
type PostChecker interface {
	MissingPaths(paths []string) ([]string, error)
}

type ReadingListService struct {
	db      *sql.DB
	queries queries.ReadingListQueries
	posts   PostChecker
}

func NewReadingListService(db *sql.DB, posts PostChecker) *ReadingListService {
	return &ReadingListService{db: db, queries: queries.NewSQLReadingListQueries(db), posts: posts}
}

// Publish validates every item's post path, then full-replaces the
// reading list's row, translations, and entire module/item tree in one
// transaction - unlike tags' add/update-only Sync, a reading list's
// current payload is authoritative for its whole structure, so modules/
// items missing from this payload must be deleted, not left stale.
func (s *ReadingListService) Publish(p models.ReadingListPayload) error {
	var postPaths []string
	for _, m := range p.Modules {
		for _, item := range m.Items {
			postPaths = append(postPaths, item.PostPath)
		}
	}
	if missing, err := s.posts.MissingPaths(postPaths); err != nil {
		return err
	} else if len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrUnknownPosts, strings.Join(missing, ", "))
	}

	return dbpkg.Transact(s.db, func(tx *sql.Tx) error {
		qtx := s.queries.WithTx(tx)

		if err := qtx.UpsertReadingList(p.Slug, p.CoverImage, p.Ongoing); err != nil {
			return err
		}
		listID, err := qtx.GetReadingListIDBySlug(p.Slug)
		if err != nil {
			return err
		}

		if len(p.Translations) == 0 {
			if err := qtx.DeleteAllTranslations(listID); err != nil {
				return err
			}
		} else {
			var langs []string
			for lang := range p.Translations {
				langs = append(langs, lang)
			}
			if err := qtx.DeleteTranslationsNotIn(listID, langs); err != nil {
				return err
			}
		}

		for lang, t := range p.Translations {
			if err := qtx.UpsertTranslation(listID, lang, t.Title, t.Description); err != nil {
				return err
			}
		}

		if err := qtx.DeleteModuleTree(listID); err != nil {
			return err
		}
		for _, m := range p.Modules {
			moduleID, err := qtx.InsertModule(listID, m, m.Order)
			if err != nil {
				return err
			}
			for _, item := range m.Items {
				if err := qtx.InsertItem(moduleID, item); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// ListAll returns every reading list, fully nested, for the admin CI pull.
func (s *ReadingListService) ListAll() ([]models.ReadingListResponse, error) {
	return s.queries.ListAll()
}
