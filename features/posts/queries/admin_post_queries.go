package queries

import (
	dbpkg "arkana/shared/db"
	"database/sql"
)

type AdminPostQueries interface {
	UpsertPostContent(postID int, lang, path, rawContent string, title, thumbnail sql.NullString) error
}

type SQLAdminPostQueries struct {
	db dbpkg.DBTX
}

func NewSQLAdminPostQueries(db dbpkg.DBTX) *SQLAdminPostQueries {
	return &SQLAdminPostQueries{db: db}
}

// UpsertPostContent inserts or updates the post_contents row for one
// (path, lang) pair.
func (q *SQLAdminPostQueries) UpsertPostContent(postID int, lang, path, rawContent string, title, thumbnail sql.NullString) error {
	_, err := q.db.Exec(
		`INSERT INTO post_contents (post_id, lang, path, content, title, thumbnail, visible)
		 VALUES (?, ?, ?, ?, ?, ?, 1)
		 ON CONFLICT (lang, path) DO UPDATE SET
		   post_id = excluded.post_id, content = excluded.content,
		   title = excluded.title, thumbnail = excluded.thumbnail,
		   visible = excluded.visible, updated_at = CURRENT_TIMESTAMP`,
		postID, lang, path, rawContent, title, thumbnail,
	)
	return err
}
