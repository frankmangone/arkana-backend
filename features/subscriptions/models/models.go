package models

import "time"

const (
	StatusPending      = "pending"
	StatusConfirmed    = "confirmed"
	StatusUnsubscribed = "unsubscribed"
)

// Subscriber is a row in the subscribers table. UserID is nil for a pure
// guest subscription and set once it's linked to an account.
type Subscriber struct {
	ID             int        `json:"id"`
	UserID         *int       `json:"user_id,omitempty"`
	Email          string     `json:"email"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ConfirmedAt    *time.Time `json:"confirmed_at,omitempty"`
	UnsubscribedAt *time.Time `json:"unsubscribed_at,omitempty"`
}

// SubscribeRequest is the body of POST /api/subscribe.
type SubscribeRequest struct {
	Email string `json:"email"`
}

// SubscribeResponse is always the same shape regardless of what happened
// internally, so a guest signup never discloses whether the email belongs
// to an existing account.
type SubscribeResponse struct {
	Status string `json:"status"`
}

// ConfirmRequest is the body of POST /api/subscribe/confirm and
// POST /api/subscriptions/unsubscribe.
type ConfirmRequest struct {
	SubscriberID int    `json:"subscriber_id"`
	Token        string `json:"token"`
}

type ConfirmResponse struct {
	Confirmed bool `json:"confirmed"`
}

type UnsubscribeResponse struct {
	Unsubscribed bool `json:"unsubscribed"`
}

// SubscriptionStatusResponse is returned by the authenticated
// subscribe/unsubscribe endpoints.
type SubscriptionStatusResponse struct {
	Subscribed bool `json:"subscribed"`
}

// BroadcastRequest is the body of POST /api/admin/subscriptions/broadcast.
type BroadcastRequest struct {
	PostID int `json:"post_id"`
}

type BroadcastResponse struct {
	Sent   int `json:"sent"`
	Failed int `json:"failed"`
}
