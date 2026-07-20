package models

import "time"

type Post struct {
	ID             int       `json:"id"`
	PathIdentifier string    `json:"path_identifier"`
	LikeCount      int       `json:"like_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Comment struct {
	ID        int       `json:"id"`
	PostID    int       `json:"post_id"`
	UserID    int       `json:"user_id"`
	ParentID  *int      `json:"parent_id,omitempty"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// CommentResponse is the API response for a comment, including author info.
type CommentResponse struct {
	ID              int       `json:"id"`
	ParentID        *int      `json:"parent_id,omitempty"`
	Body            string    `json:"body"`
	CreatedAt       time.Time `json:"created_at"`
	AuthorUsername  string    `json:"author_username"`
	AuthorAvatarURL *string   `json:"author_avatar_url,omitempty"`
}

// CommentsResponse wraps the list of comments for a post.
type CommentsResponse struct {
	Comments []CommentResponse `json:"comments"`
	Total    int               `json:"total"`
}

type ToggleLikeResponse struct {
	Liked     bool `json:"liked"`
	LikeCount int  `json:"like_count"`
}

type ToggleReadResponse struct {
	Read bool `json:"read"`
}

type PostInfoResponse struct {
	Path      string `json:"path"`
	LikeCount int    `json:"like_count"`
	Liked     bool   `json:"liked"`
	Read      bool   `json:"read"`
}
