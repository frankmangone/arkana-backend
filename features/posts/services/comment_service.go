package services

import (
	notifmodels "arkana/features/notifications/models"
	notifservices "arkana/features/notifications/services"
	"arkana/features/posts/models"
	"database/sql"
	"errors"
	"fmt"
)

// MaxCommentLength is the maximum allowed length for a comment body.
const MaxCommentLength = 1000

var ErrCommentTooLong = errors.New("comment exceeds maximum length")

type CommentService struct {
	db            *sql.DB
	notifications *notifservices.NotificationService
}

func NewCommentService(db *sql.DB, notifications *notifservices.NotificationService) *CommentService {
	return &CommentService{db: db, notifications: notifications}
}

// Create adds a new comment to a post. In the same transaction, it notifies
// the parent comment's author (on a reply) and the post's writer (on any
// comment) — see docs/superpowers/specs/2026-07-20-notifications-design.md
// for the suppression rules.
func (s *CommentService) Create(postID, userID int, body string, parentID *int) (*models.Comment, error) {
	if len(body) > MaxCommentLength {
		return nil, ErrCommentTooLong
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	replyRecipient := 0
	if parentID != nil {
		var parentPostID, parentUserID int
		err := tx.QueryRow(
			"SELECT post_id, user_id FROM comments WHERE id = ?", *parentID,
		).Scan(&parentPostID, &parentUserID)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("parent comment not found")
		}
		if err != nil {
			return nil, err
		}
		if parentPostID != postID {
			return nil, fmt.Errorf("parent comment belongs to a different post")
		}
		replyRecipient = parentUserID
	}

	result, err := tx.Exec(
		"INSERT INTO comments (post_id, user_id, parent_id, body) VALUES (?, ?, ?, ?)",
		postID, userID, parentID, body,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	var c models.Comment
	err = tx.QueryRow(
		"SELECT id, post_id, user_id, parent_id, body, created_at FROM comments WHERE id = ?",
		id,
	).Scan(&c.ID, &c.PostID, &c.UserID, &c.ParentID, &c.Body, &c.CreatedAt)
	if err != nil {
		return nil, err
	}

	if replyRecipient != 0 {
		if err := s.notifications.Create(tx, replyRecipient, userID, notifmodels.TypeCommentReply, &postID, &c.ID); err != nil {
			return nil, err
		}
	}

	var writerUserID sql.NullInt64
	err = tx.QueryRow(
		"SELECT w.user_id FROM posts p JOIN writers w ON w.id = p.writer_id WHERE p.id = ?",
		postID,
	).Scan(&writerUserID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == nil && writerUserID.Valid {
		writer := int(writerUserID.Int64)
		if writer != replyRecipient {
			if err := s.notifications.Create(tx, writer, userID, notifmodels.TypePostCommented, &postID, &c.ID); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &c, nil
}

// GetByPostID returns all comments for a post, ordered by creation time.
// Includes the author's username and avatar from the users table.
func (s *CommentService) GetByPostID(postID int) (*models.CommentsResponse, error) {
	rows, err := s.db.Query(`
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
