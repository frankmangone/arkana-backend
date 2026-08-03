// Package services implements the subscription business logic: guest and
// authenticated signup, email confirmation, unsubscribe (both the logged-in
// and token-based one-click paths), status lookups, and broadcasting new-post
// notifications. The main type is SubscriptionService, constructed via
// NewSubscriptionService and wired with a *sql.DB, an email.Sender, and the
// token secret / frontend URL used to sign and build the links embedded in
// outgoing emails. It persists through queries.SubscriptionQueries and
// renders outgoing email bodies using the templates in this package.
package services

import (
	"arkana/features/subscriptions/models"
	"arkana/features/subscriptions/queries"
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
	queries     queries.SubscriptionQueries
	sender      email.Sender
	tokenSecret string
	frontendURL string
}

// NewSubscriptionService constructs a SubscriptionService backed by db, using
// sender to deliver confirmation and broadcast emails, tokenSecret to sign
// and verify subscription tokens, and frontendURL to build the links
// embedded in those emails.
func NewSubscriptionService(db *sql.DB, sender email.Sender, tokenSecret, frontendURL string) *SubscriptionService {
	return &SubscriptionService{
		db:          db,
		queries:     queries.NewSQLSubscriptionQueries(db),
		sender:      sender,
		tokenSecret: tokenSecret,
		frontendURL: frontendURL,
	}
}

// Subscribe handles a guest signup. It always returns nil (success) unless
// an internal error occurs — including when the email belongs to an
// existing user account, which is silently rejected (logged, no row
// created, no email sent) so the response never discloses whether an
// email is registered.
func (s *SubscriptionService) Subscribe(ctx context.Context, emailAddr string) error {
	exists, err := s.queries.UserEmailExists(emailAddr)
	if err != nil {
		return err
	}
	if exists {
		log.Printf("subscribe: rejected a guest signup matching an existing account")
		return nil
	}

	id, status, err := s.queries.GetSubscriberByEmail(emailAddr)
	switch {
	case err == nil:
		if status == models.StatusConfirmed {
			return nil
		}
		if err := s.queries.UpdateSubscriberStatus(id, models.StatusPending); err != nil {
			return err
		}
	case errors.Is(err, sql.ErrNoRows):
		newID, err := s.queries.InsertSubscriber(emailAddr, models.StatusPending)
		if err != nil {
			return err
		}
		id = int(newID)
	default:
		return err
	}

	return s.sendConfirmEmail(ctx, id, emailAddr)
}

// ConfirmSubscription confirms a pending subscriber. It's idempotent.
func (s *SubscriptionService) ConfirmSubscription(ctx context.Context, subscriberID int, token string) error {
	if !VerifySubscriptionToken(s.tokenSecret, subscriberID, PurposeConfirm, token) {
		return ErrInvalidToken
	}
	return s.queries.ConfirmSubscriber(subscriberID)
}

// AuthenticatedSubscribe subscribes a logged-in user immediately as
// confirmed. If a guest row already exists for their account email, it
// links and confirms that row instead of creating a duplicate.
func (s *SubscriptionService) AuthenticatedSubscribe(ctx context.Context, userID int, userEmail string) error {
	id, err := s.queries.GetSubscriberIDByEmail(userEmail)
	switch {
	case err == nil:
		return s.queries.LinkAndConfirmSubscriber(userID, id)
	case errors.Is(err, sql.ErrNoRows):
		return s.queries.InsertConfirmedSubscriber(userID, userEmail)
	default:
		return err
	}
}

// AuthenticatedUnsubscribe unsubscribes the logged-in user's row. A no-op
// (no error) if they were never subscribed.
func (s *SubscriptionService) AuthenticatedUnsubscribe(ctx context.Context, userID int) error {
	return s.queries.UnsubscribeByUserID(userID)
}

// UnsubscribeByToken is the public, no-login one-click unsubscribe path.
func (s *SubscriptionService) UnsubscribeByToken(ctx context.Context, subscriberID int, token string) error {
	if !VerifySubscriptionToken(s.tokenSecret, subscriberID, PurposeUnsubscribe, token) {
		return ErrInvalidToken
	}
	return s.queries.UnsubscribeByID(subscriberID)
}

// GetStatus reports whether userID has a confirmed subscription.
func (s *SubscriptionService) GetStatus(ctx context.Context, userID int) (bool, error) {
	return s.queries.IsConfirmedSubscriber(userID)
}

// Broadcast sends a new-post notification to every confirmed subscriber.
// A failure sending to one recipient is logged and does not abort the
// batch; the returned counts reflect the real outcome.
func (s *SubscriptionService) Broadcast(ctx context.Context, postID int) (sent, failed int, err error) {
	path, title, err := s.queries.GetVisiblePostForBroadcast(postID)
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

	recipients, err := s.queries.ListConfirmedSubscribers()
	if err != nil {
		return 0, 0, err
	}

	postURL := fmt.Sprintf("%s/%s", s.frontendURL, path)
	subject := fmt.Sprintf("New post: %s", postTitle)

	for _, r := range recipients {
		unsubToken := GenerateSubscriptionToken(s.tokenSecret, r.ID, PurposeUnsubscribe)
		unsubLink := fmt.Sprintf("%s/unsubscribe?sid=%d&token=%s", s.frontendURL, r.ID, unsubToken)
		body, renderErr := RenderBroadcast(BroadcastEmailData{
			PostTitle:       postTitle,
			PostURL:         postURL,
			UnsubscribeLink: unsubLink,
		})
		if renderErr != nil {
			return sent, failed, renderErr
		}
		if err := s.sender.Send(ctx, email.Message{To: r.Email, Subject: subject, HTMLBody: body}); err != nil {
			log.Printf("broadcast: failed to send to subscriber %d: %v", r.ID, err)
			failed++
			continue
		}
		sent++
	}

	return sent, failed, nil
}

// sendConfirmEmail generates a confirmation token and link for subscriberID
// and emails it to to.
func (s *SubscriptionService) sendConfirmEmail(ctx context.Context, subscriberID int, to string) error {
	token := GenerateSubscriptionToken(s.tokenSecret, subscriberID, PurposeConfirm)
	link := fmt.Sprintf("%s/subscribe/confirm?sid=%d&token=%s", s.frontendURL, subscriberID, token)
	body, err := RenderConfirm(ConfirmEmailData{Link: link})
	if err != nil {
		return err
	}
	return s.sender.Send(ctx, email.Message{
		To:       to,
		Subject:  "Confirm your subscription",
		HTMLBody: body,
	})
}
