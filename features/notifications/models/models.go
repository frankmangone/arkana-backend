package models

import "time"

const (
	TypeCommentReply  = "comment_reply"
	TypePostLiked     = "post_liked"
	TypePostCommented = "post_commented"
)

// Notification represents a single in-app notification for a recipient.
type Notification struct {
	ID              int        `json:"id"`
	RecipientUserID int        `json:"recipient_user_id"`
	ActorUserID     int        `json:"actor_user_id"`
	Type            string     `json:"type"`
	PostID          *int       `json:"post_id,omitempty"`
	CommentID       *int       `json:"comment_id,omitempty"`
	ReadAt          *time.Time `json:"read_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// NotificationsResponse wraps a page of notifications for a user.
type NotificationsResponse struct {
	Notifications []Notification `json:"notifications"`
	Total         int            `json:"total"`
}

// UnreadCountResponse is the response for the unread-count endpoint.
type UnreadCountResponse struct {
	Count int `json:"count"`
}

// MarkReadResponse is the response for marking one notification read.
type MarkReadResponse struct {
	Read bool `json:"read"`
}
