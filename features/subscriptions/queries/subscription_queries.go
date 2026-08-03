package queries

import (
	"arkana/features/subscriptions/models"
	dbpkg "arkana/shared/db"
	"database/sql"
)

type SubscriberRecipient struct {
	ID    int
	Email string
}

type SubscriptionQueries interface {
	UserEmailExists(email string) (bool, error)
	GetSubscriberByEmail(email string) (id int, status string, err error)
	UpdateSubscriberStatus(id int, status string) error
	InsertSubscriber(email, status string) (int64, error)
	ConfirmSubscriber(id int) error
	GetSubscriberIDByEmail(email string) (int, error)
	LinkAndConfirmSubscriber(userID, subscriberID int) error
	InsertConfirmedSubscriber(userID int, email string) error
	UnsubscribeByUserID(userID int) error
	UnsubscribeByID(id int) error
	IsConfirmedSubscriber(userID int) (bool, error)
	GetVisiblePostForBroadcast(postID int) (path string, title sql.NullString, err error)
	ListConfirmedSubscribers() ([]SubscriberRecipient, error)
}

type SQLSubscriptionQueries struct {
	db dbpkg.DBTX
}

func NewSQLSubscriptionQueries(db dbpkg.DBTX) *SQLSubscriptionQueries {
	return &SQLSubscriptionQueries{db: db}
}

// UserEmailExists reports whether a users row has this email.
func (q *SQLSubscriptionQueries) UserEmailExists(email string) (bool, error) {
	var exists int
	err := q.db.QueryRow("SELECT 1 FROM users WHERE email = ?", email).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetSubscriberByEmail returns a subscriber's id and status. Returns
// sql.ErrNoRows (unmodified) if none exists.
func (q *SQLSubscriptionQueries) GetSubscriberByEmail(email string) (id int, status string, err error) {
	err = q.db.QueryRow("SELECT id, status FROM subscribers WHERE email = ?", email).Scan(&id, &status)
	return
}

// UpdateSubscriberStatus sets a subscriber's status by id.
func (q *SQLSubscriptionQueries) UpdateSubscriberStatus(id int, status string) error {
	_, err := q.db.Exec("UPDATE subscribers SET status = ? WHERE id = ?", status, id)
	return err
}

// InsertSubscriber creates a new subscriber row and returns its id.
func (q *SQLSubscriptionQueries) InsertSubscriber(email, status string) (int64, error) {
	result, err := q.db.Exec("INSERT INTO subscribers (email, status) VALUES (?, ?)", email, status)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// ConfirmSubscriber marks a pending subscriber confirmed. It's a no-op
// (no error) if the row isn't currently 'pending' — the UPDATE simply
// matches zero rows, so an already-unsubscribed row is never resurrected
// by replaying an old confirm link.
func (q *SQLSubscriptionQueries) ConfirmSubscriber(id int) error {
	_, err := q.db.Exec(
		"UPDATE subscribers SET status = ?, confirmed_at = CURRENT_TIMESTAMP WHERE id = ? AND status = ?",
		models.StatusConfirmed, id, models.StatusPending,
	)
	return err
}

// GetSubscriberIDByEmail returns a subscriber's id by email. Returns
// sql.ErrNoRows (unmodified) if none exists.
func (q *SQLSubscriptionQueries) GetSubscriberIDByEmail(email string) (int, error) {
	var id int
	err := q.db.QueryRow("SELECT id FROM subscribers WHERE email = ?", email).Scan(&id)
	return id, err
}

// LinkAndConfirmSubscriber links an existing guest subscriber row to a
// logged-in user and marks it confirmed.
func (q *SQLSubscriptionQueries) LinkAndConfirmSubscriber(userID, subscriberID int) error {
	_, err := q.db.Exec(
		"UPDATE subscribers SET user_id = ?, status = ?, confirmed_at = CURRENT_TIMESTAMP WHERE id = ?",
		userID, models.StatusConfirmed, subscriberID,
	)
	return err
}

// InsertConfirmedSubscriber creates a new, already-confirmed subscriber
// row for a logged-in user (their email is already OAuth-verified).
func (q *SQLSubscriptionQueries) InsertConfirmedSubscriber(userID int, email string) error {
	_, err := q.db.Exec(
		"INSERT INTO subscribers (user_id, email, status, confirmed_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)",
		userID, email, models.StatusConfirmed,
	)
	return err
}

// UnsubscribeByUserID unsubscribes the row owned by userID. A no-op (no
// error) if there is none.
func (q *SQLSubscriptionQueries) UnsubscribeByUserID(userID int) error {
	_, err := q.db.Exec(
		"UPDATE subscribers SET status = ?, unsubscribed_at = CURRENT_TIMESTAMP WHERE user_id = ?",
		models.StatusUnsubscribed, userID,
	)
	return err
}

// UnsubscribeByID unsubscribes a subscriber row by id.
func (q *SQLSubscriptionQueries) UnsubscribeByID(id int) error {
	_, err := q.db.Exec(
		"UPDATE subscribers SET status = ?, unsubscribed_at = CURRENT_TIMESTAMP WHERE id = ?",
		models.StatusUnsubscribed, id,
	)
	return err
}

// IsConfirmedSubscriber reports whether userID has a confirmed subscription.
func (q *SQLSubscriptionQueries) IsConfirmedSubscriber(userID int) (bool, error) {
	var exists int
	err := q.db.QueryRow(
		"SELECT 1 FROM subscribers WHERE user_id = ? AND status = ? LIMIT 1",
		userID, models.StatusConfirmed,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetVisiblePostForBroadcast returns a post's path and title, for the
// broadcast email. Returns sql.ErrNoRows (unmodified) if no visible
// content row exists for it.
func (q *SQLSubscriptionQueries) GetVisiblePostForBroadcast(postID int) (path string, title sql.NullString, err error) {
	err = q.db.QueryRow(
		`SELECT p.path_identifier, pc.title
		 FROM posts p
		 JOIN post_contents pc ON pc.post_id = p.id
		 WHERE p.id = ? AND pc.visible = 1
		 ORDER BY pc.id ASC LIMIT 1`,
		postID,
	).Scan(&path, &title)
	return
}

// ListConfirmedSubscribers returns every confirmed subscriber's id and email.
func (q *SQLSubscriptionQueries) ListConfirmedSubscribers() ([]SubscriberRecipient, error) {
	rows, err := q.db.Query("SELECT id, email FROM subscribers WHERE status = ?", models.StatusConfirmed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recipients []SubscriberRecipient
	for rows.Next() {
		var r SubscriberRecipient
		if err := rows.Scan(&r.ID, &r.Email); err != nil {
			return nil, err
		}
		recipients = append(recipients, r)
	}
	return recipients, rows.Err()
}
