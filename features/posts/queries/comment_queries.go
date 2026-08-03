package queries

import (
	"arkana/features/posts/models"
	dbpkg "arkana/shared/db"
	"database/sql"
)

type CommentQueries interface {
	GetParentComment(id int) (postID, userID int, err error)
	InsertComment(postID, userID int, parentID *int, body string) (int64, error)
	GetCommentByID(id int64) (*models.Comment, error)
	GetPostWriterUserID(postID int) (sql.NullInt64, error)
	GetByPostID(postID int) (*models.CommentsResponse, error)
	WithTx(tx *sql.Tx) CommentQueries
}

type SQLCommentQueries struct {
	db dbpkg.DBTX
}

func NewSQLCommentQueries(db dbpkg.DBTX) *SQLCommentQueries {
	return &SQLCommentQueries{db: db}
}

func (q *SQLCommentQueries) WithTx(tx *sql.Tx) CommentQueries {
	return NewSQLCommentQueries(tx)
}

// GetParentComment returns a parent comment's post_id and user_id, for
// reply validation. Returns sql.ErrNoRows (unmodified) if it doesn't exist.
func (q *SQLCommentQueries) GetParentComment(id int) (postID, userID int, err error) {
	err = q.db.QueryRow("SELECT post_id, user_id FROM comments WHERE id = ?", id).Scan(&postID, &userID)
	return
}

// InsertComment creates a new comment row and returns its id.
func (q *SQLCommentQueries) InsertComment(postID, userID int, parentID *int, body string) (int64, error) {
	result, err := q.db.Exec(
		"INSERT INTO comments (post_id, user_id, parent_id, body) VALUES (?, ?, ?, ?)",
		postID, userID, parentID, body,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetCommentByID reloads a comment row by id, right after insert.
func (q *SQLCommentQueries) GetCommentByID(id int64) (*models.Comment, error) {
	var c models.Comment
	err := q.db.QueryRow(
		"SELECT id, post_id, user_id, parent_id, body, created_at FROM comments WHERE id = ?",
		id,
	).Scan(&c.ID, &c.PostID, &c.UserID, &c.ParentID, &c.Body, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetPostWriterUserID returns the user_id of postID's writer, if any.
func (q *SQLCommentQueries) GetPostWriterUserID(postID int) (sql.NullInt64, error) {
	var writerUserID sql.NullInt64
	err := q.db.QueryRow(
		"SELECT w.user_id FROM posts p JOIN writers w ON w.id = p.writer_id WHERE p.id = ?",
		postID,
	).Scan(&writerUserID)
	return writerUserID, err
}

// GetByPostID returns all comments for a post, ordered by creation time,
// including each author's username and avatar.
func (q *SQLCommentQueries) GetByPostID(postID int) (*models.CommentsResponse, error) {
	rows, err := q.db.Query(`
		SELECT c.id, c.parent_id, c.body, c.created_at, u.username, u.avatar_url
		FROM comments c
		JOIN users u ON u.id = c.user_id
		WHERE c.post_id = ?
		ORDER BY c.created_at ASC
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []models.CommentResponse
	for rows.Next() {
		var c models.CommentResponse
		var username sql.NullString
		var avatarURL sql.NullString
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Body, &c.CreatedAt, &username, &avatarURL); err != nil {
			return nil, err
		}
		if username.Valid {
			c.AuthorUsername = username.String
		}
		if avatarURL.Valid {
			c.AuthorAvatarURL = &avatarURL.String
		}
		comments = append(comments, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if comments == nil {
		comments = []models.CommentResponse{}
	}

	return &models.CommentsResponse{
		Comments: comments,
		Total:    len(comments),
	}, nil
}
