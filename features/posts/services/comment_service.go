package services

import (
	"arkana/features/posts/models"
	"database/sql"
	"errors"
	"fmt"
)

// MaxCommentLength is the maximum allowed length for a comment body.
const MaxCommentLength = 1000

var ErrCommentTooLong = errors.New("comment exceeds maximum length")

type CommentService struct {
	db *sql.DB
}

func NewCommentService(db *sql.DB) *CommentService {
	return &CommentService{db: db}
}

// Create adds a new comment to a post. If parentID is non-nil, validates
// that the parent comment belongs to the same post.
func (s *CommentService) Create(postID, walletID int, body string, parentID *int) (*models.Comment, error) {
	if len(body) > MaxCommentLength {
		return nil, ErrCommentTooLong
	}

	if parentID != nil {
		var parentPostID int
		err := s.db.QueryRow(
			"SELECT post_id FROM comments WHERE id = ?", *parentID,
		).Scan(&parentPostID)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("parent comment not found")
		}
		if err != nil {
			return nil, err
		}
		if parentPostID != postID {
			return nil, fmt.Errorf("parent comment belongs to a different post")
		}
	}

	result, err := s.db.Exec(
		"INSERT INTO comments (post_id, wallet_id, parent_id, body) VALUES (?, ?, ?, ?)",
		postID, walletID, parentID, body,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	var c models.Comment
	err = s.db.QueryRow(
		"SELECT id, post_id, wallet_id, parent_id, body, created_at FROM comments WHERE id = ?",
		id,
	).Scan(&c.ID, &c.PostID, &c.WalletID, &c.ParentID, &c.Body, &c.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

// GetByPostID returns all comments for a post, ordered by creation time.
// Includes the author's wallet address for display.
func (s *CommentService) GetByPostID(postID int) (*models.CommentsResponse, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.parent_id, c.body, c.created_at, w.address
		FROM comments c
		JOIN wallets w ON w.id = c.wallet_id
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
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Body, &c.CreatedAt, &c.AuthorAddress); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Return empty slice instead of nil for cleaner JSON
	if comments == nil {
		comments = []models.CommentResponse{}
	}

	return &models.CommentsResponse{
		Comments: comments,
		Total:    len(comments),
	}, nil
}
