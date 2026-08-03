package queries

import (
	"arkana/features/readinglists/models"
	dbpkg "arkana/shared/db"
	"database/sql"
	"fmt"
)

type ReadingListQueries interface {
	UpsertReadingList(slug, coverImage string, ongoing bool) error
	GetReadingListIDBySlug(slug string) (int64, error)
	DeleteAllTranslations(listID int64) error
	DeleteTranslationsNotIn(listID int64, langs []string) error
	UpsertTranslation(listID int64, lang, title, description string) error
	DeleteModuleTree(listID int64) error
	InsertModule(listID int64, m models.ModulePayload, position int) (int64, error)
	InsertItem(moduleID int64, item models.ItemPayload) error
	ListAll() ([]models.ReadingListResponse, error)
	WithTx(tx *sql.Tx) ReadingListQueries
}

type SQLReadingListQueries struct {
	db dbpkg.DBTX
}

func NewSQLReadingListQueries(db dbpkg.DBTX) *SQLReadingListQueries {
	return &SQLReadingListQueries{db: db}
}

func (q *SQLReadingListQueries) WithTx(tx *sql.Tx) ReadingListQueries {
	return NewSQLReadingListQueries(tx)
}

// UpsertReadingList inserts a reading_lists row by slug, or updates its
// cover_image/ongoing fields if the slug already exists.
func (q *SQLReadingListQueries) UpsertReadingList(slug, coverImage string, ongoing bool) error {
	_, err := q.db.Exec(
		`INSERT INTO reading_lists (slug, cover_image, ongoing) VALUES (?, ?, ?)
		 ON CONFLICT(slug) DO UPDATE SET
		   cover_image = excluded.cover_image, ongoing = excluded.ongoing,
		   updated_at = CURRENT_TIMESTAMP`,
		slug, coverImage, ongoing,
	)
	return err
}

// GetReadingListIDBySlug returns a reading list's id by slug.
func (q *SQLReadingListQueries) GetReadingListIDBySlug(slug string) (int64, error) {
	var listID int64
	err := q.db.QueryRow("SELECT id FROM reading_lists WHERE slug = ?", slug).Scan(&listID)
	return listID, err
}

// DeleteAllTranslations removes every translation row for a reading list.
func (q *SQLReadingListQueries) DeleteAllTranslations(listID int64) error {
	_, err := q.db.Exec(`DELETE FROM reading_list_translations WHERE reading_list_id = ?`, listID)
	return err
}

// DeleteTranslationsNotIn removes translation rows whose lang isn't in langs.
func (q *SQLReadingListQueries) DeleteTranslationsNotIn(listID int64, langs []string) error {
	_, err := q.db.Exec(
		fmt.Sprintf(`DELETE FROM reading_list_translations WHERE reading_list_id = ? AND lang NOT IN (%s)`, dbpkg.Placeholders(len(langs))),
		append([]any{listID}, dbpkg.ToAny(langs)...)...,
	)
	return err
}

// UpsertTranslation inserts or updates one (reading list, lang) translation.
func (q *SQLReadingListQueries) UpsertTranslation(listID int64, lang, title, description string) error {
	_, err := q.db.Exec(
		`INSERT INTO reading_list_translations (reading_list_id, lang, title, description)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(reading_list_id, lang) DO UPDATE SET
		   title = excluded.title, description = excluded.description`,
		listID, lang, title, description,
	)
	return err
}

// DeleteModuleTree removes every module (and its translations/items) under
// listID, children before parents since this SQLite setup has no FK
// cascade.
func (q *SQLReadingListQueries) DeleteModuleTree(listID int64) error {
	if _, err := q.db.Exec(
		`DELETE FROM reading_list_items WHERE module_id IN (SELECT id FROM reading_list_modules WHERE reading_list_id = ?)`,
		listID,
	); err != nil {
		return err
	}
	if _, err := q.db.Exec(
		`DELETE FROM reading_list_module_translations WHERE module_id IN (SELECT id FROM reading_list_modules WHERE reading_list_id = ?)`,
		listID,
	); err != nil {
		return err
	}
	if _, err := q.db.Exec(`DELETE FROM reading_list_modules WHERE reading_list_id = ?`, listID); err != nil {
		return err
	}
	return nil
}

// InsertModule creates a module row (and its translations) and returns its id.
func (q *SQLReadingListQueries) InsertModule(listID int64, m models.ModulePayload, position int) (int64, error) {
	result, err := q.db.Exec(
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
		if _, err := q.db.Exec(
			`INSERT INTO reading_list_module_translations (module_id, lang, title, description) VALUES (?, ?, ?, ?)`,
			moduleID, lang, t.Title, t.Description,
		); err != nil {
			return 0, err
		}
	}
	return moduleID, nil
}

// InsertItem creates an item row under a module.
func (q *SQLReadingListQueries) InsertItem(moduleID int64, item models.ItemPayload) error {
	_, err := q.db.Exec(
		`INSERT INTO reading_list_items (module_id, slug, post_path, position) VALUES (?, ?, ?, ?)`,
		moduleID, item.Slug, item.PostPath, item.Order,
	)
	return err
}

// ListAll returns every reading list, fully nested, for the admin CI pull.
// Implemented as three separate single-join queries assembled in Go keyed
// by ID, rather than one query joining all three levels at once - a query
// joining translations at two different levels simultaneously would
// cross-multiply rows.
func (q *SQLReadingListQueries) ListAll() ([]models.ReadingListResponse, error) {
	listsByID, listOrder, err := q.loadLists()
	if err != nil {
		return nil, err
	}
	modulesByID, moduleOrderByList, err := q.loadModules()
	if err != nil {
		return nil, err
	}
	itemsByModule, err := q.loadItems()
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

func (q *SQLReadingListQueries) loadLists() (map[int64]models.ReadingListResponse, []int64, error) {
	rows, err := q.db.Query(
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

func (q *SQLReadingListQueries) loadModules() (map[int64]models.ModuleResponse, map[int64][]int64, error) {
	rows, err := q.db.Query(
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
			orderByList[listID] = append(orderByList[listID], id)
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

func (q *SQLReadingListQueries) loadItems() (map[int64][]models.ItemResponse, error) {
	rows, err := q.db.Query(
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
