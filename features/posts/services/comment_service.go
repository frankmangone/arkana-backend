package services

import (
	"arkana/features/posts/models"
	"database/sql"
	"fmt"
)

type CommentService struct {
	db *sql.DB
}

func NewCommentService(db *sql.DB) *CommentService {
	return &CommentService{db: db}
}

// Create adds a new comment to a post. If parentID is non-nil, validates
// that the parent comment belongs to the same post.
func (s *CommentService) Create(postID, walletID int, body string, parentID *int) (*models.Comment, error) {
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
