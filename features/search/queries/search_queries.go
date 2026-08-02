package queries

import (
	dbpkg "arkana/shared/db"
	"database/sql"
	"errors"
	"log"
)

type SearchQueries interface {
	LookupThumbnail(lang, path string) string
}

type SQLSearchQueries struct {
	db dbpkg.DBTX
}

func NewSQLSearchQueries(db dbpkg.DBTX) *SQLSearchQueries {
	return &SQLSearchQueries{db: db}
}

// LookupThumbnail joins back to post_contents by (lang, path) since
// Meilisearch itself doesn't store the thumbnail. post_contents.path
// includes the ".md" extension; callers pass paths without it, so it's
// appended here. A missing row or unset thumbnail just yields "" rather
// than failing the whole search.
func (q *SQLSearchQueries) LookupThumbnail(lang, path string) string {
	var thumbnail sql.NullString

	err := q.db.QueryRow(
		"SELECT thumbnail FROM post_contents WHERE lang = ? AND path = ?",
		lang, path+".md",
	).Scan(&thumbnail)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("[Search] thumbnail lookup failed for lang=%q path=%q: %v", lang, path, err)
		}
		return ""
	}

	return thumbnail.String
}
