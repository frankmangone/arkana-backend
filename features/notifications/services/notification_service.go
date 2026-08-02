package services

import (
	"arkana/features/notifications/models"
	"arkana/features/notifications/queries"
	dbpkg "arkana/shared/db"
	"database/sql"
	"errors"
)

var ErrNotificationNotFound = errors.New("notification not found")

type NotificationService struct {
	db      *sql.DB
	queries queries.NotificationQueries
}

func NewNotificationService(db *sql.DB) *NotificationService {
	return &NotificationService{db: db, queries: queries.NewSQLNotificationQueries(db)}
}

// Create inserts a notification for recipientUserID caused by actorUserID.
// It no-ops (no error) when the recipient is the actor.
func (s *NotificationService) Create(exec dbpkg.DBTX, recipientUserID, actorUserID int, notifType string, postID, commentID *int) error {
	return s.queries.Create(exec, recipientUserID, actorUserID, notifType, postID, commentID)
}

// List returns a user's notifications, newest first, paginated.
func (s *NotificationService) List(userID, limit, offset int) (*models.NotificationsResponse, error) {
	return s.queries.List(userID, limit, offset)
}

// MarkRead sets read_at on a notification owned by userID. Returns
// ErrNotificationNotFound if it doesn't exist or belongs to someone else.
// Marking an already-read notification again is a no-op success.
func (s *NotificationService) MarkRead(id, userID int) error {
	rowsAffected, err := s.queries.MarkRead(id, userID)
	if err != nil {
		return err
	}
	if rowsAffected > 0 {
		return nil
	}
	exists, err := s.queries.Exists(id, userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotificationNotFound
	}
	return nil
}

// UnreadCount returns how many unread notifications userID has.
func (s *NotificationService) UnreadCount(userID int) (int, error) {
	return s.queries.UnreadCount(userID)
}
