package services

import (
	notifmodels "arkana/features/notifications/models"
	notifservices "arkana/features/notifications/services"
	"arkana/features/posts/models"
	"arkana/features/posts/queries"
	dbpkg "arkana/shared/db"
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

// NewCommentService constructs a CommentService backed by db, using
// notifications to notify parent comment authors and post writers of new
// comments.
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

	var comment *models.Comment
	err := dbpkg.Transact(s.db, func(tx *sql.Tx) error {
		qtx := s.queries.WithTx(tx)

		replyRecipient := 0
		if parentID != nil {
			parentPostID, parentUserID, err := qtx.GetParentComment(*parentID)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("parent comment not found")
			}
			if err != nil {
				return err
			}
			if parentPostID != postID {
				return fmt.Errorf("parent comment belongs to a different post")
			}
			replyRecipient = parentUserID
		}

		id, err := qtx.InsertComment(postID, userID, parentID, body)
		if err != nil {
			return err
		}

		c, err := qtx.GetCommentByID(id)
		if err != nil {
			return err
		}

		if replyRecipient != 0 {
			if err := s.notifications.Create(tx, replyRecipient, userID, notifmodels.TypeCommentReply, &postID, &c.ID); err != nil {
				return err
			}
		}

		writerUserID, err := qtx.GetPostWriterUserID(postID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && writerUserID.Valid {
			writer := int(writerUserID.Int64)
			if writer != replyRecipient {
				if err := s.notifications.Create(tx, writer, userID, notifmodels.TypePostCommented, &postID, &c.ID); err != nil {
					return err
				}
			}
		}

		comment = c
		return nil
	})
	if err != nil {
		return nil, err
	}

	return comment, nil
}

// GetByPostID returns all comments for a post, ordered by creation time.
func (s *CommentService) GetByPostID(postID int) (*models.CommentsResponse, error) {
	return s.queries.GetByPostID(postID)
}
