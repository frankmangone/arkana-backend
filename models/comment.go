package models

import "time"

// Comment represents a comment on an article
type Comment struct {
	ID        int       `db:"id" json:"id"`
	ArticleID int       `db:"article_id" json:"article_id"`
	Author    string    `db:"author" json:"author"`
	Content   string    `db:"content" json:"content"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
