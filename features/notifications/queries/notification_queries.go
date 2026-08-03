package queries

import (
	"database/sql"

	"arkana/features/notifications/models"
	dbpkg "arkana/shared/db"
)

type NotificationQueries interface {
	Create(exec dbpkg.DBTX, recipientUserID, actorUserID int, notifType string, postID, commentID *int) error
	List(userID, limit, offset int) (*models.NotificationsResponse, error)
	MarkRead(id, userID int) (int64, error)
	Exists(id, userID int) (bool, error)
	UnreadCount(userID int) (int, error)
}

type SQLNotificationQueries struct {
	db dbpkg.DBTX
}

func NewSQLNotificationQueries(db dbpkg.DBTX) *SQLNotificationQueries {
	return &SQLNotificationQueries{db: db}
}

// Create inserts a notification for recipientUserID caused by actorUserID.
// It no-ops (no error) when the recipient is the actor, so callers never
// need to special-case self-notifications themselves. exec is supplied by
// the caller (often another service's own open transaction), not q.db.
func (q *SQLNotificationQueries) Create(exec dbpkg.DBTX, recipientUserID, actorUserID int, notifType string, postID, commentID *int) error {
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
func (q *SQLNotificationQueries) List(userID, limit, offset int) (*models.NotificationsResponse, error) {
	rows, err := q.db.Query(
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
	if err := q.db.QueryRow("SELECT COUNT(*) FROM notifications WHERE recipient_user_id = ?", userID).Scan(&total); err != nil {
		return nil, err
	}

	return &models.NotificationsResponse{Notifications: notifications, Total: total}, nil
}

// MarkRead sets read_at on a notification owned by userID, returning the
// number of rows the UPDATE affected.
func (q *SQLNotificationQueries) MarkRead(id, userID int) (int64, error) {
	result, err := q.db.Exec(
		"UPDATE notifications SET read_at = CURRENT_TIMESTAMP WHERE id = ? AND recipient_user_id = ? AND read_at IS NULL",
		id, userID,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Exists reports whether a notification with this id belongs to userID.
func (q *SQLNotificationQueries) Exists(id, userID int) (bool, error) {
	var exists int
	err := q.db.QueryRow("SELECT 1 FROM notifications WHERE id = ? AND recipient_user_id = ?", id, userID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// UnreadCount returns how many unread notifications userID has.
func (q *SQLNotificationQueries) UnreadCount(userID int) (int, error) {
	var count int
	err := q.db.QueryRow(
		"SELECT COUNT(*) FROM notifications WHERE recipient_user_id = ? AND read_at IS NULL",
		userID,
	).Scan(&count)
	return count, err
}
