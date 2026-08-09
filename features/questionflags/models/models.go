package models

import "time"

// QuestionFlag represents one user's feedback on a quiz question. List
// entries carry the joined question/user context an admin needs to act on
// the report; a freshly created flag only fills the fields the insert
// itself produced.
type QuestionFlag struct {
	ID           int       `json:"id"`
	QuestionID   int       `json:"question_id"`
	QuestionUUID string    `json:"question_uuid,omitempty"`
	QuestionSlug string    `json:"question_slug,omitempty"`
	UserID       int       `json:"user_id"`
	UserEmail    string    `json:"user_email,omitempty"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
}

// ListFlagsResponse is the response for the admin list endpoint.
type ListFlagsResponse struct {
	Flags []QuestionFlag `json:"flags"`
}

// DeleteFlagsResponse is the response for the admin flush endpoint.
type DeleteFlagsResponse struct {
	Deleted int64 `json:"deleted"`
}
