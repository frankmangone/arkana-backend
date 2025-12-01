package models

import "time"

// Article represents a blog article
type Article struct {
	ID          int        `db:"id" json:"id"`
	Slug        string     `db:"slug" json:"slug"`
	Title       string     `db:"title" json:"title"`
	ExternalURL string     `db:"external_url" json:"external_url"`
	PublishedAt *time.Time `db:"published_at" json:"published_at,omitempty"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
}
