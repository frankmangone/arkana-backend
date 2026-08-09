package queries

import (
	"arkana/features/posts/models"
	dbpkg "arkana/shared/db"
	"database/sql"
	"fmt"
)

type PostQueries interface {
	GetByPath(path string) (*models.Post, error)
	InsertPost(path string) (int64, error)
	GetByID(id int) (*models.Post, error)
	SetWriterID(postID int, writerID int64) error
	MissingPaths(paths []string) ([]string, error)
	GetIDsByPaths(paths []string) (map[string]int, error)
	LikeExists(postID, userID int) (bool, error)
	InsertLike(postID, userID int) error
	IncrementLikeCount(postID int) error
	DeleteLike(postID, userID int) error
	DecrementLikeCount(postID int) error
	GetPostWriterUserID(postID int) (sql.NullInt64, error)
	GetLikeCount(postID int) (int, error)
	ReadExists(postID, userID int) (bool, error)
	InsertRead(postID, userID int) error
	DeleteRead(postID, userID int) error
	GetReadStatuses(paths []string, userID int) (map[string]bool, error)
	GetPostInfo(path string) (postID, likeCount int, err error)
	UserHasLiked(postID, userID int) (bool, error)
	UserHasRead(postID, userID int) (bool, error)
	ListVisibleContentPage(limit, offset int) ([]models.AdminPostContentItem, int, error)
	WithTx(tx *sql.Tx) PostQueries
}

type SQLPostQueries struct {
	db dbpkg.DBTX
}

func NewSQLPostQueries(db dbpkg.DBTX) *SQLPostQueries {
	return &SQLPostQueries{db: db}
}

func (q *SQLPostQueries) WithTx(tx *sql.Tx) PostQueries {
	return NewSQLPostQueries(tx)
}

// GetByPath finds a post by path_identifier. Returns sql.ErrNoRows
// (unmodified) if none exists — the service maps this to ErrPostNotFound.
func (q *SQLPostQueries) GetByPath(path string) (*models.Post, error) {
	var p models.Post
	err := q.db.QueryRow(
		"SELECT id, path_identifier, like_count, created_at, updated_at FROM posts WHERE path_identifier = ?",
		path,
	).Scan(&p.ID, &p.PathIdentifier, &p.LikeCount, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// InsertPost creates a new posts row and returns its id.
func (q *SQLPostQueries) InsertPost(path string) (int64, error) {
	result, err := q.db.Exec("INSERT INTO posts (path_identifier) VALUES (?)", path)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// SetWriterID sets posts.writer_id for postID, linking the post to a
// writer so post_liked/post_commented notifications can be routed.
func (q *SQLPostQueries) SetWriterID(postID int, writerID int64) error {
	_, err := q.db.Exec("UPDATE posts SET writer_id = ? WHERE id = ?", writerID, postID)
	return err
}

// GetByID finds a post by its id.
func (q *SQLPostQueries) GetByID(id int) (*models.Post, error) {
	var p models.Post
	err := q.db.QueryRow(
		"SELECT id, path_identifier, like_count, created_at, updated_at FROM posts WHERE id = ?",
		id,
	).Scan(&p.ID, &p.PathIdentifier, &p.LikeCount, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// MissingPaths returns the subset of paths that have no row in posts.
// Returns nil for an empty input without querying.
func (q *SQLPostQueries) MissingPaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	rows, err := q.db.Query(
		fmt.Sprintf("SELECT path_identifier FROM posts WHERE path_identifier IN (%s)", dbpkg.Placeholders(len(paths))),
		dbpkg.ToAny(paths)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	found := make(map[string]bool, len(paths))
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		found[path] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var missing []string
	for _, p := range paths {
		if !found[p] {
			missing = append(missing, p)
		}
	}
	return missing, nil
}

// GetIDsByPaths returns a map of path_identifier -> posts.id for every
// path that has a matching row.
func (q *SQLPostQueries) GetIDsByPaths(paths []string) (map[string]int, error) {
	if len(paths) == 0 {
		return map[string]int{}, nil
	}

	rows, err := q.db.Query(
		fmt.Sprintf("SELECT id, path_identifier FROM posts WHERE path_identifier IN (%s)", dbpkg.Placeholders(len(paths))),
		dbpkg.ToAny(paths)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int, len(paths))
	for rows.Next() {
		var id int
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		result[path] = id
	}
	return result, rows.Err()
}

// LikeExists reports whether userID has already liked postID.
func (q *SQLPostQueries) LikeExists(postID, userID int) (bool, error) {
	var exists int
	err := q.db.QueryRow(
		"SELECT 1 FROM post_likes WHERE post_id = ? AND user_id = ?",
		postID, userID,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// InsertLike records a like.
func (q *SQLPostQueries) InsertLike(postID, userID int) error {
	_, err := q.db.Exec("INSERT INTO post_likes (post_id, user_id) VALUES (?, ?)", postID, userID)
	return err
}

// IncrementLikeCount bumps a post's like_count by one.
func (q *SQLPostQueries) IncrementLikeCount(postID int) error {
	_, err := q.db.Exec("UPDATE posts SET like_count = like_count + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?", postID)
	return err
}

// DeleteLike removes a like.
func (q *SQLPostQueries) DeleteLike(postID, userID int) error {
	_, err := q.db.Exec("DELETE FROM post_likes WHERE post_id = ? AND user_id = ?", postID, userID)
	return err
}

// DecrementLikeCount drops a post's like_count by one.
func (q *SQLPostQueries) DecrementLikeCount(postID int) error {
	_, err := q.db.Exec("UPDATE posts SET like_count = like_count - 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?", postID)
	return err
}

// GetPostWriterUserID returns the user_id of postID's writer, if any.
func (q *SQLPostQueries) GetPostWriterUserID(postID int) (sql.NullInt64, error) {
	var writerUserID sql.NullInt64
	err := q.db.QueryRow(
		"SELECT w.user_id FROM posts p JOIN writers w ON w.id = p.writer_id WHERE p.id = ?",
		postID,
	).Scan(&writerUserID)
	return writerUserID, err
}

// GetLikeCount returns a post's current like_count.
func (q *SQLPostQueries) GetLikeCount(postID int) (int, error) {
	var likeCount int
	err := q.db.QueryRow("SELECT like_count FROM posts WHERE id = ?", postID).Scan(&likeCount)
	return likeCount, err
}

// ReadExists reports whether userID has already marked postID read.
func (q *SQLPostQueries) ReadExists(postID, userID int) (bool, error) {
	var exists int
	err := q.db.QueryRow(
		"SELECT 1 FROM post_reads WHERE post_id = ? AND user_id = ?",
		postID, userID,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// InsertRead records a post as read.
func (q *SQLPostQueries) InsertRead(postID, userID int) error {
	_, err := q.db.Exec("INSERT INTO post_reads (post_id, user_id) VALUES (?, ?)", postID, userID)
	return err
}

// DeleteRead removes a post's read marker.
func (q *SQLPostQueries) DeleteRead(postID, userID int) error {
	_, err := q.db.Exec("DELETE FROM post_reads WHERE post_id = ? AND user_id = ?", postID, userID)
	return err
}

// GetReadStatuses returns path -> read status for the given user across
// many posts in one query.
func (q *SQLPostQueries) GetReadStatuses(paths []string, userID int) (map[string]bool, error) {
	result := make(map[string]bool, len(paths))
	for _, p := range paths {
		result[p] = false
	}

	if userID <= 0 || len(paths) == 0 {
		return result, nil
	}

	query := fmt.Sprintf(
		`SELECT p.path_identifier
		 FROM posts p
		 JOIN post_reads r ON r.post_id = p.id
		 WHERE p.path_identifier IN (%s) AND r.user_id = ?`,
		dbpkg.Placeholders(len(paths)),
	)

	rows, err := q.db.Query(query, append(dbpkg.ToAny(paths), userID)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		result[path] = true
	}

	return result, rows.Err()
}

// GetPostInfo returns a post's id and like_count by path. Returns
// sql.ErrNoRows (unmodified) if none exists.
func (q *SQLPostQueries) GetPostInfo(path string) (postID, likeCount int, err error) {
	err = q.db.QueryRow(
		"SELECT id, like_count FROM posts WHERE path_identifier = ?",
		path,
	).Scan(&postID, &likeCount)
	return
}

// UserHasLiked reports whether userID has liked postID.
func (q *SQLPostQueries) UserHasLiked(postID, userID int) (bool, error) {
	var exists int
	err := q.db.QueryRow(
		"SELECT 1 FROM post_likes WHERE post_id = ? AND user_id = ?",
		postID, userID,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// UserHasRead reports whether userID has read postID.
func (q *SQLPostQueries) UserHasRead(postID, userID int) (bool, error) {
	var exists int
	err := q.db.QueryRow(
		"SELECT 1 FROM post_reads WHERE post_id = ? AND user_id = ?",
		postID, userID,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListVisibleContentPage returns one page of visible post_contents rows
// (ordered by id for stable pagination) plus the total visible count.
func (q *SQLPostQueries) ListVisibleContentPage(limit, offset int) ([]models.AdminPostContentItem, int, error) {
	var total int
	if err := q.db.QueryRow("SELECT COUNT(*) FROM post_contents WHERE visible = 1").Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := q.db.Query(
		"SELECT lang, path, content FROM post_contents WHERE visible = 1 ORDER BY id ASC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []models.AdminPostContentItem{}
	for rows.Next() {
		var item models.AdminPostContentItem
		if err := rows.Scan(&item.Lang, &item.Path, &item.Content); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}

	return items, total, rows.Err()
}
