package services

import (
	notifmodels "arkana/features/notifications/models"
	notifservices "arkana/features/notifications/services"
	"arkana/features/posts/models"
	"arkana/features/posts/queries"
	"database/sql"
	"errors"
	"fmt"
)

// MaxCommentLength is the maximum allowed length for a comment body.
const MaxCommentLength = 1000

var ErrCommentTooLong = errors.New("comment exceeds maximum length")

type CommentService struct {
	db            *sql.DB
	queries       queries.CommentQueries
	notifications *notifservices.NotificationService
}

func NewCommentService(db *sql.DB, notifications *notifservices.NotificationService) *CommentService {
	return &CommentService{db: db, queries: queries.NewSQLCommentQueries(db), notifications: notifications}
}

// Create adds a new comment to a post. In the same transaction, it notifies
// the parent comment's author (on a reply) and the post's writer (on any
// comment).
func (s *CommentService) Create(postID, userID int, body string, parentID *int) (*models.Comment, error) {
	if len(body) > MaxCommentLength {
		return nil, ErrCommentTooLong
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	qtx := s.queries.WithTx(tx)

	replyRecipient := 0
	if parentID != nil {
		parentPostID, parentUserID, err := qtx.GetParentComment(*parentID)
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

	id, err := qtx.InsertComment(postID, userID, parentID, body)
	if err != nil {
		return nil, err
	}

	c, err := qtx.GetCommentByID(id)
	if err != nil {
		return nil, err
	}

	if replyRecipient != 0 {
		if err := s.notifications.Create(tx, replyRecipient, userID, notifmodels.TypeCommentReply, &postID, &c.ID); err != nil {
			return nil, err
		}
	}

	writerUserID, err := qtx.GetPostWriterUserID(postID)
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

	return c, nil
}

// GetByPostID returns all comments for a post, ordered by creation time.
func (s *CommentService) GetByPostID(postID int) (*models.CommentsResponse, error) {
	return s.queries.GetByPostID(postID)
}
