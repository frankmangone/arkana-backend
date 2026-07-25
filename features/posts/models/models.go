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

// PublishPostRequest is the body of POST /api/admin/posts. RawContent is
// the whole markdown file as-is (YAML frontmatter + body) - the backend
// parses frontmatter, stores the body, and strips markdown for search
// indexing, so the calling CI workflow doesn't need any content-parsing
// logic of its own.
type PublishPostRequest struct {
	Path       string `json:"path"`
	Lang       string `json:"lang"`
	RawContent string `json:"raw_content"`
}

type PublishPostResponse struct {
	Published bool `json:"published"`
}

// AdminPostContentItem is one page item for the admin/CI content pull -
// just the fields the pull script writes to disk (lang, path, full raw
// content with frontmatter). Title/thumbnail/description stay embedded in
// Content's own frontmatter, not duplicated here.
type AdminPostContentItem struct {
	Lang    string `json:"lang"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

// AdminPostContentListResponse wraps one page of AdminPostContentItem plus
// the total visible count, so the calling pull script knows when to stop
// paging.
type AdminPostContentListResponse struct {
	Data  []AdminPostContentItem `json:"data"`
	Total int                    `json:"total"`
}
