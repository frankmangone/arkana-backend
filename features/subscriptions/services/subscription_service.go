package services

import (
	"arkana/features/subscriptions/models"
	"arkana/shared/email"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrPostNotFound = errors.New("post not found")
)

type SubscriptionService struct {
	db          *sql.DB
	sender      email.Sender
	tokenSecret string
	frontendURL string
}

func NewSubscriptionService(db *sql.DB, sender email.Sender, tokenSecret, frontendURL string) *SubscriptionService {
	return &SubscriptionService{db: db, sender: sender, tokenSecret: tokenSecret, frontendURL: frontendURL}
}

// Subscribe handles a guest signup. It always returns nil (success) unless
// an internal error occurs — including when the email belongs to an
// existing user account, which is silently rejected (logged, no row
// created, no email sent) so the response never discloses whether an
// email is registered.
func (s *SubscriptionService) Subscribe(ctx context.Context, emailAddr string) error {
	var exists int
	err := s.db.QueryRow("SELECT 1 FROM users WHERE email = ?", emailAddr).Scan(&exists)
	if err == nil {
		log.Printf("subscribe: rejected a guest signup matching an existing account")
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}

	var id int
	var status string
	err = s.db.QueryRow("SELECT id, status FROM subscribers WHERE email = ?", emailAddr).Scan(&id, &status)
	switch {
	case err == nil:
		if status == models.StatusConfirmed {
			return nil
		}
		if _, err := s.db.Exec("UPDATE subscribers SET status = ? WHERE id = ?", models.StatusPending, id); err != nil {
			return err
		}
	case errors.Is(err, sql.ErrNoRows):
		result, err := s.db.Exec("INSERT INTO subscribers (email, status) VALUES (?, ?)", emailAddr, models.StatusPending)
		if err != nil {
			return err
		}
		newID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		id = int(newID)
	default:
		return err
	}

	return s.sendConfirmEmail(ctx, id, emailAddr)
}

// ConfirmSubscription confirms a pending subscriber. It's idempotent: an
// already-confirmed row is left as-is, and — critically — an already
// unsubscribed row is NOT resurrected by replaying an old confirm link,
// since the UPDATE only matches rows still in 'pending'.
func (s *SubscriptionService) ConfirmSubscription(ctx context.Context, subscriberID int, token string) error {
	if !VerifySubscriptionToken(s.tokenSecret, subscriberID, PurposeConfirm, token) {
		return ErrInvalidToken
	}
	_, err := s.db.Exec(
		"UPDATE subscribers SET status = ?, confirmed_at = CURRENT_TIMESTAMP WHERE id = ? AND status = ?",
		models.StatusConfirmed, subscriberID, models.StatusPending,
	)
	return err
}

// AuthenticatedSubscribe subscribes a logged-in user immediately as
// confirmed (their email is already OAuth-verified). If a guest row
// already exists for their account email, it links and confirms that row
// instead of creating a duplicate.
func (s *SubscriptionService) AuthenticatedSubscribe(ctx context.Context, userID int, userEmail string) error {
	var id int
	err := s.db.QueryRow("SELECT id FROM subscribers WHERE email = ?", userEmail).Scan(&id)
	switch {
	case err == nil:
		_, err = s.db.Exec(
			"UPDATE subscribers SET user_id = ?, status = ?, confirmed_at = CURRENT_TIMESTAMP WHERE id = ?",
			userID, models.StatusConfirmed, id,
		)
		return err
	case errors.Is(err, sql.ErrNoRows):
		_, err = s.db.Exec(
			"INSERT INTO subscribers (user_id, email, status, confirmed_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)",
			userID, userEmail, models.StatusConfirmed,
		)
		return err
	default:
		return err
	}
}

// AuthenticatedUnsubscribe unsubscribes the logged-in user's row. A no-op
// (no error) if they were never subscribed.
func (s *SubscriptionService) AuthenticatedUnsubscribe(ctx context.Context, userID int) error {
	_, err := s.db.Exec(
		"UPDATE subscribers SET status = ?, unsubscribed_at = CURRENT_TIMESTAMP WHERE user_id = ?",
		models.StatusUnsubscribed, userID,
	)
	return err
}

// UnsubscribeByToken is the public, no-login one-click unsubscribe path.
func (s *SubscriptionService) UnsubscribeByToken(ctx context.Context, subscriberID int, token string) error {
	if !VerifySubscriptionToken(s.tokenSecret, subscriberID, PurposeUnsubscribe, token) {
		return ErrInvalidToken
	}
	_, err := s.db.Exec(
		"UPDATE subscribers SET status = ?, unsubscribed_at = CURRENT_TIMESTAMP WHERE id = ?",
		models.StatusUnsubscribed, subscriberID,
	)
	return err
}

// GetStatus reports whether userID has a confirmed subscription.
func (s *SubscriptionService) GetStatus(ctx context.Context, userID int) (bool, error) {
	var exists int
	err := s.db.QueryRow(
		"SELECT 1 FROM subscribers WHERE user_id = ? AND status = ? LIMIT 1",
		userID, models.StatusConfirmed,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Broadcast sends a new-post notification to every confirmed subscriber.
// A failure sending to one recipient is logged and does not abort the
// batch; the returned counts reflect the real outcome.
func (s *SubscriptionService) Broadcast(ctx context.Context, postID int) (sent, failed int, err error) {
	var path string
	var title sql.NullString
	err = s.db.QueryRow(
		`SELECT p.path_identifier, pc.title
		 FROM posts p
		 JOIN post_contents pc ON pc.post_id = p.id
		 WHERE p.id = ? AND pc.visible = 1
		 ORDER BY pc.id ASC LIMIT 1`,
		postID,
	).Scan(&path, &title)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, ErrPostNotFound
	}
	if err != nil {
		return 0, 0, err
	}

	postTitle := title.String
	if postTitle == "" {
		postTitle = path
	}

	rows, err := s.db.Query("SELECT id, email FROM subscribers WHERE status = ?", models.StatusConfirmed)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	type recipient struct {
		id    int
		email string
	}
	var recipients []recipient
	for rows.Next() {
		var r recipient
		if err := rows.Scan(&r.id, &r.email); err != nil {
			return 0, 0, err
		}
		recipients = append(recipients, r)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	postURL := fmt.Sprintf("%s/%s", s.frontendURL, path)
	subject := fmt.Sprintf("New post: %s", postTitle)

	for _, r := range recipients {
		unsubToken := GenerateSubscriptionToken(s.tokenSecret, r.id, PurposeUnsubscribe)
		unsubLink := fmt.Sprintf("%s/unsubscribe?sid=%d&token=%s", s.frontendURL, r.id, unsubToken)
		body := fmt.Sprintf(
			`<p>A new post is up: <a href="%s">%s</a></p><p><a href="%s">Unsubscribe</a></p>`,
			postURL, postTitle, unsubLink,
		)
		if err := s.sender.Send(ctx, email.Message{To: r.email, Subject: subject, HTMLBody: body}); err != nil {
			log.Printf("broadcast: failed to send to subscriber %d: %v", r.id, err)
			failed++
			continue
		}
		sent++
	}

	return sent, failed, nil
}

func (s *SubscriptionService) sendConfirmEmail(ctx context.Context, subscriberID int, to string) error {
	token := GenerateSubscriptionToken(s.tokenSecret, subscriberID, PurposeConfirm)
	link := fmt.Sprintf("%s/subscribe/confirm?sid=%d&token=%s", s.frontendURL, subscriberID, token)
	return s.sender.Send(ctx, email.Message{
		To:       to,
		Subject:  "Confirm your subscription",
		HTMLBody: fmt.Sprintf(`<p>Click <a href="%s">here</a> to confirm your subscription to Arkana.</p>`, link),
	})
}
