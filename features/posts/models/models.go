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
	WalletID  int       `json:"wallet_id"`
	ParentID  *int      `json:"parent_id,omitempty"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type ToggleLikeResponse struct {
	Liked     bool `json:"liked"`
	LikeCount int  `json:"like_count"`
}

type CreateCommentRequest struct {
	Body     string `json:"body" validate:"required"`
	ParentID *int   `json:"parent_id,omitempty"`
}
