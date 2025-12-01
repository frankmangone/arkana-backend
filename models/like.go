package models

import "time"

// Like represents a like on an article
type Like struct {
	ID        int       `db:"id" json:"id"`
	ArticleID int       `db:"article_id" json:"article_id"`
	IPOrUserID string   `db:"ip_or_user_id" json:"ip_or_user_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
