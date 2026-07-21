package services

import (
	"arkana/features/notifications/models"
	"database/sql"
	"errors"
)

// Executor is satisfied by both *sql.DB and *sql.Tx, letting callers include
// a notification insert in their own transaction.
type Executor interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

var ErrNotificationNotFound = errors.New("notification not found")

type NotificationService struct {
	db *sql.DB
}

func NewNotificationService(db *sql.DB) *NotificationService {
	return &NotificationService{db: db}
}

// Create inserts a notification for recipientUserID caused by actorUserID.
// It no-ops (no error) when the recipient is the actor, so callers never
// need to special-case self-notifications themselves.
func (s *NotificationService) Create(exec Executor, recipientUserID, actorUserID int, notifType string, postID, commentID *int) error {
	if recipientUserID == actorUserID {
		return nil
	}
	_, err := exec.Exec(
		"INSERT INTO notifications (recipient_user_id, actor_user_id, type, post_id, comment_id) VALUES (?, ?, ?, ?, ?)",
		recipientUserID, actorUserID, notifType, postID, commentID,
	)
	return err
}

// List returns a user's notifications, newest first, paginated. Each row
// includes the actor's username/avatar and (when the post still exists)
// its path, so the frontend can render and link the notification without a
// second round trip.
func (s *NotificationService) List(userID, limit, offset int) (*models.NotificationsResponse, error) {
	rows, err := s.db.Query(
		`SELECT n.id, n.recipient_user_id, n.actor_user_id, n.type, n.post_id, n.comment_id, n.read_at, n.created_at,
		        u.username, u.avatar_url, p.path_identifier
		 FROM notifications n
		 JOIN users u ON u.id = n.actor_user_id
		 LEFT JOIN posts p ON p.id = n.post_id
		 WHERE n.recipient_user_id = ?
		 ORDER BY n.created_at DESC, n.id DESC
		 LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notifications := []models.Notification{}
	for rows.Next() {
		var n models.Notification
		var postID, commentID sql.NullInt64
		var readAt sql.NullTime
		var actorUsername, actorAvatarURL, postPath sql.NullString
		if err := rows.Scan(
			&n.ID, &n.RecipientUserID, &n.ActorUserID, &n.Type, &postID, &commentID, &readAt, &n.CreatedAt,
			&actorUsername, &actorAvatarURL, &postPath,
		); err != nil {
			return nil, err
		}
		if postID.Valid {
			v := int(postID.Int64)
			n.PostID = &v
		}
		if commentID.Valid {
			v := int(commentID.Int64)
			n.CommentID = &v
		}
		if readAt.Valid {
			n.ReadAt = &readAt.Time
		}
		if actorUsername.Valid {
			n.ActorUsername = actorUsername.String
		}
		if actorAvatarURL.Valid {
			n.ActorAvatarURL = &actorAvatarURL.String
		}
		if postPath.Valid {
			n.PostPath = &postPath.String
		}
		notifications = append(notifications, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM notifications WHERE recipient_user_id = ?", userID).Scan(&total); err != nil {
		return nil, err
	}

	return &models.NotificationsResponse{Notifications: notifications, Total: total}, nil
}

// MarkRead sets read_at on a notification owned by userID. Returns
// ErrNotificationNotFound if it doesn't exist or belongs to someone else.
// Marking an already-read notification again is a no-op success.
func (s *NotificationService) MarkRead(id, userID int) error {
	result, err := s.db.Exec(
		"UPDATE notifications SET read_at = CURRENT_TIMESTAMP WHERE id = ? AND recipient_user_id = ? AND read_at IS NULL",
		id, userID,
	)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		var exists int
		err := s.db.QueryRow("SELECT 1 FROM notifications WHERE id = ? AND recipient_user_id = ?", id, userID).Scan(&exists)
		if err == sql.ErrNoRows {
			return ErrNotificationNotFound
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// UnreadCount returns how many unread notifications userID has.
func (s *NotificationService) UnreadCount(userID int) (int, error) {
	var count int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM notifications WHERE recipient_user_id = ? AND read_at IS NULL",
		userID,
	).Scan(&count)
	return count, err
}
