package services

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"arkana/features/readinglists/models"
)

var ErrUnknownPosts = errors.New("unknown post path(s)")

// PostChecker validates that a set of post paths correspond to real posts.
// Satisfied structurally by *posts/services.PostService, without this
// package depending on it directly.
type PostChecker interface {
	MissingPaths(paths []string) ([]string, error)
}

type ReadingListService struct {
	db    *sql.DB
	posts PostChecker
}

func NewReadingListService(db *sql.DB, posts PostChecker) *ReadingListService {
	return &ReadingListService{db: db, posts: posts}
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

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var listID int64
	if _, err := tx.Exec(
		`INSERT INTO reading_lists (slug, cover_image, ongoing) VALUES (?, ?, ?)
		 ON CONFLICT(slug) DO UPDATE SET
		   cover_image = excluded.cover_image, ongoing = excluded.ongoing,
		   updated_at = CURRENT_TIMESTAMP`,
		p.Slug, p.CoverImage, p.Ongoing,
	); err != nil {
		return err
	}
	if err := tx.QueryRow("SELECT id FROM reading_lists WHERE slug = ?", p.Slug).Scan(&listID); err != nil {
		return err
	}

	for lang, t := range p.Translations {
		if _, err := tx.Exec(
			`INSERT INTO reading_list_translations (reading_list_id, lang, title, description)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(reading_list_id, lang) DO UPDATE SET
			   title = excluded.title, description = excluded.description`,
			listID, lang, t.Title, t.Description,
		); err != nil {
			return err
		}
	}

	if err := deleteModuleTree(tx, listID); err != nil {
		return err
	}
	for mi, m := range p.Modules {
		moduleID, err := insertModule(tx, listID, m, mi)
		if err != nil {
			return err
		}
		for _, item := range m.Items {
			if err := insertItem(tx, moduleID, item); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// deleteModuleTree removes every module (and its translations/items) under
// listID, children before parents since this SQLite setup has no FK
// cascade.
func deleteModuleTree(tx *sql.Tx, listID int64) error {
	if _, err := tx.Exec(
		`DELETE FROM reading_list_items WHERE module_id IN (SELECT id FROM reading_list_modules WHERE reading_list_id = ?)`,
		listID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`DELETE FROM reading_list_module_translations WHERE module_id IN (SELECT id FROM reading_list_modules WHERE reading_list_id = ?)`,
		listID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM reading_list_modules WHERE reading_list_id = ?`, listID); err != nil {
		return err
	}
	return nil
}

func insertModule(tx *sql.Tx, listID int64, m models.ModulePayload, position int) (int64, error) {
	result, err := tx.Exec(
		`INSERT INTO reading_list_modules (reading_list_id, slug, position) VALUES (?, ?, ?)`,
		listID, m.Slug, position,
	)
	if err != nil {
		return 0, err
	}
	moduleID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	for lang, t := range m.Translations {
		if _, err := tx.Exec(
			`INSERT INTO reading_list_module_translations (module_id, lang, title, description) VALUES (?, ?, ?, ?)`,
			moduleID, lang, t.Title, t.Description,
		); err != nil {
			return 0, err
		}
	}
	return moduleID, nil
}

func insertItem(tx *sql.Tx, moduleID int64, item models.ItemPayload) error {
	_, err := tx.Exec(
		`INSERT INTO reading_list_items (module_id, slug, post_path, position) VALUES (?, ?, ?, ?)`,
		moduleID, item.Slug, item.PostPath, item.Order,
	)
	return err
}

// ListAll returns every reading list, fully nested, for the admin CI pull.
// Implemented as three separate single-join queries assembled in Go keyed
// by ID, rather than one query joining all three levels at once - a query
// joining translations at two different levels simultaneously would
// cross-multiply rows (N translations x M items per module). Each query
// joins at most one child table, same row-explosion avoidance as
// tags.TagService.List's single LEFT JOIN, one level deeper.
func (s *ReadingListService) ListAll() ([]models.ReadingListResponse, error) {
	listsByID, listOrder, err := s.loadLists()
	if err != nil {
		return nil, err
	}
	modulesByID, moduleOrderByList, err := s.loadModules()
	if err != nil {
		return nil, err
	}
	itemsByModule, err := s.loadItems()
	if err != nil {
		return nil, err
	}

	result := make([]models.ReadingListResponse, 0, len(listOrder))
	for _, listID := range listOrder {
		list := listsByID[listID]
		for _, moduleID := range moduleOrderByList[listID] {
			module := modulesByID[moduleID]
			module.Items = itemsByModule[moduleID]
			list.Modules = append(list.Modules, module)
		}
		result = append(result, list)
	}
	return result, nil
}

func (s *ReadingListService) loadLists() (map[int64]models.ReadingListResponse, []int64, error) {
	rows, err := s.db.Query(
		`SELECT rl.id, rl.slug, rl.cover_image, rl.ongoing, rlt.lang, rlt.title, rlt.description
		 FROM reading_lists rl
		 LEFT JOIN reading_list_translations rlt ON rlt.reading_list_id = rl.id
		 ORDER BY rl.slug`,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	byID := map[int64]models.ReadingListResponse{}
	var order []int64
	for rows.Next() {
		var id int64
		var slug string
		var coverImage sql.NullString
		var ongoing bool
		var lang, title, description sql.NullString
		if err := rows.Scan(&id, &slug, &coverImage, &ongoing, &lang, &title, &description); err != nil {
			return nil, nil, err
		}
		entry, ok := byID[id]
		if !ok {
			entry = models.ReadingListResponse{
				Slug:         slug,
				CoverImage:   coverImage.String,
				Ongoing:      ongoing,
				Translations: map[string]models.Translation{},
				Modules:      []models.ModuleResponse{},
			}
			order = append(order, id)
		}
		if lang.Valid {
			entry.Translations[lang.String] = models.Translation{Title: title.String, Description: description.String}
		}
		byID[id] = entry
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return byID, order, nil
}

func (s *ReadingListService) loadModules() (map[int64]models.ModuleResponse, map[int64][]int64, error) {
	rows, err := s.db.Query(
		`SELECT rlm.id, rlm.reading_list_id, rlm.slug, rlm.position, rlmt.lang, rlmt.title, rlmt.description
		 FROM reading_list_modules rlm
		 LEFT JOIN reading_list_module_translations rlmt ON rlmt.module_id = rlm.id
		 ORDER BY rlm.reading_list_id, rlm.position`,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	byID := map[int64]models.ModuleResponse{}
	orderByList := map[int64][]int64{}
	seen := map[int64]bool{}
	for rows.Next() {
		var id, listID int64
		var slug string
		var position int
		var lang, title, description sql.NullString
		if err := rows.Scan(&id, &listID, &slug, &position, &lang, &title, &description); err != nil {
			return nil, nil, err
		}
		entry, ok := byID[id]
		if !ok {
			entry = models.ModuleResponse{
				Slug:         slug,
				Order:        position,
				Translations: map[string]models.Translation{},
				Items:        []models.ItemResponse{},
			}
			if !seen[id] {
				orderByList[listID] = append(orderByList[listID], id)
				seen[id] = true
			}
		}
		if lang.Valid {
			entry.Translations[lang.String] = models.Translation{Title: title.String, Description: description.String}
		}
		byID[id] = entry
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return byID, orderByList, nil
}

func (s *ReadingListService) loadItems() (map[int64][]models.ItemResponse, error) {
	rows, err := s.db.Query(
		`SELECT module_id, slug, post_path, position FROM reading_list_items ORDER BY module_id, position`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byModule := map[int64][]models.ItemResponse{}
	for rows.Next() {
		var moduleID int64
		var slug, postPath string
		var position int
		if err := rows.Scan(&moduleID, &slug, &postPath, &position); err != nil {
			return nil, err
		}
		byModule[moduleID] = append(byModule[moduleID], models.ItemResponse{Slug: slug, PostPath: postPath, Order: position})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return byModule, nil
}
