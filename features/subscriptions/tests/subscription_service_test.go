package tests

import (
	"context"
	"database/sql"
	"testing"

	"arkana/features/subscriptions/services"
)

func getSubscriberByEmail(t *testing.T, db *sql.DB, email string) (id int, userID sql.NullInt64, status string, found bool) {
	t.Helper()
	err := db.QueryRow("SELECT id, user_id, status FROM subscribers WHERE email = ?", email).Scan(&id, &userID, &status)
	if err == sql.ErrNoRows {
		return 0, sql.NullInt64{}, "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return id, userID, status, true
}

func TestSubscriptionServiceSubscribe(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a pending subscriber and sends a confirm email", func(t *testing.T) {
		db := setupTestDB(t)
		sender := newFakeSender()
		svc := setupService(t, db, sender)

		if err := svc.Subscribe(ctx, "guest@example.com"); err != nil {
			t.Fatal(err)
		}

		_, userID, status, found := getSubscriberByEmail(t, db, "guest@example.com")
		if !found {
			t.Fatal("expected a subscribers row to be created")
		}
		if status != "pending" {
			t.Errorf("status = %q, want pending", status)
		}
		if userID.Valid {
			t.Errorf("user_id = %v, want NULL for a guest signup", userID)
		}

		msgs := sender.messages()
		if len(msgs) != 1 {
			t.Fatalf("len(sent) = %d, want 1", len(msgs))
		}
		if msgs[0].To != "guest@example.com" {
			t.Errorf("sent to %q, want guest@example.com", msgs[0].To)
		}
	})

	t.Run("no-ops when the email is already confirmed", func(t *testing.T) {
		db := setupTestDB(t)
		sender := newFakeSender()
		svc := setupService(t, db, sender)

		if err := svc.Subscribe(ctx, "confirmed@example.com"); err != nil {
			t.Fatal(err)
		}
		id, _, _, _ := getSubscriberByEmail(t, db, "confirmed@example.com")
		if err := svc.ConfirmSubscription(ctx, id, services.GenerateSubscriptionToken(testTokenSecret, id, services.PurposeConfirm)); err != nil {
			t.Fatal(err)
		}

		if err := svc.Subscribe(ctx, "confirmed@example.com"); err != nil {
			t.Fatal(err)
		}

		_, _, status, _ := getSubscriberByEmail(t, db, "confirmed@example.com")
		if status != "confirmed" {
			t.Errorf("status = %q, want confirmed (unchanged)", status)
		}
		if len(sender.messages()) != 1 {
			t.Errorf("len(sent) = %d, want 1 (no second confirm email)", len(sender.messages()))
		}
	})

	t.Run("resets an unsubscribed row back to pending and resends the confirm email", func(t *testing.T) {
		db := setupTestDB(t)
		sender := newFakeSender()
		svc := setupService(t, db, sender)

		if err := svc.Subscribe(ctx, "resub@example.com"); err != nil {
			t.Fatal(err)
		}
		id, _, _, _ := getSubscriberByEmail(t, db, "resub@example.com")
		token := services.GenerateSubscriptionToken(testTokenSecret, id, services.PurposeConfirm)
		if err := svc.ConfirmSubscription(ctx, id, token); err != nil {
			t.Fatal(err)
		}
		unsubToken := services.GenerateSubscriptionToken(testTokenSecret, id, services.PurposeUnsubscribe)
		if err := svc.UnsubscribeByToken(ctx, id, unsubToken); err != nil {
			t.Fatal(err)
		}

		if err := svc.Subscribe(ctx, "resub@example.com"); err != nil {
			t.Fatal(err)
		}

		_, _, status, _ := getSubscriberByEmail(t, db, "resub@example.com")
		if status != "pending" {
			t.Errorf("status = %q, want pending", status)
		}
		if len(sender.messages()) != 2 {
			t.Errorf("len(sent) = %d, want 2 (original + resend)", len(sender.messages()))
		}
	})

	t.Run("silently no-ops when the email belongs to an existing user account", func(t *testing.T) {
		db := setupTestDB(t)
		sender := newFakeSender()
		svc := setupService(t, db, sender)
		insertTestUser(t, db, "hasaccount@example.com")

		if err := svc.Subscribe(ctx, "hasaccount@example.com"); err != nil {
			t.Fatal(err)
		}

		_, _, _, found := getSubscriberByEmail(t, db, "hasaccount@example.com")
		if found {
			t.Error("expected no subscribers row to be created for an existing account's email")
		}
		if len(sender.messages()) != 0 {
			t.Error("expected no confirm email to be sent")
		}
	})
}

func TestSubscriptionServiceConfirmSubscription(t *testing.T) {
	ctx := context.Background()

	t.Run("confirms a pending subscriber with a valid token", func(t *testing.T) {
		db := setupTestDB(t)
		svc := setupService(t, db, newFakeSender())
		if err := svc.Subscribe(ctx, "confirmme@example.com"); err != nil {
			t.Fatal(err)
		}
		id, _, _, _ := getSubscriberByEmail(t, db, "confirmme@example.com")
		token := services.GenerateSubscriptionToken(testTokenSecret, id, services.PurposeConfirm)

		if err := svc.ConfirmSubscription(ctx, id, token); err != nil {
			t.Fatal(err)
		}

		_, _, status, _ := getSubscriberByEmail(t, db, "confirmme@example.com")
		if status != "confirmed" {
			t.Errorf("status = %q, want confirmed", status)
		}
	})

	t.Run("rejects an invalid token", func(t *testing.T) {
		db := setupTestDB(t)
		svc := setupService(t, db, newFakeSender())
		if err := svc.Subscribe(ctx, "badtoken@example.com"); err != nil {
			t.Fatal(err)
		}
		id, _, _, _ := getSubscriberByEmail(t, db, "badtoken@example.com")

		err := svc.ConfirmSubscription(ctx, id, "not-the-real-token")
		if err != services.ErrInvalidToken {
			t.Errorf("err = %v, want ErrInvalidToken", err)
		}
	})

	t.Run("rejects an unsubscribe-purpose token", func(t *testing.T) {
		db := setupTestDB(t)
		svc := setupService(t, db, newFakeSender())
		if err := svc.Subscribe(ctx, "wrongpurpose@example.com"); err != nil {
			t.Fatal(err)
		}
		id, _, _, _ := getSubscriberByEmail(t, db, "wrongpurpose@example.com")
		unsubToken := services.GenerateSubscriptionToken(testTokenSecret, id, services.PurposeUnsubscribe)

		err := svc.ConfirmSubscription(ctx, id, unsubToken)
		if err != services.ErrInvalidToken {
			t.Errorf("err = %v, want ErrInvalidToken", err)
		}
	})

	t.Run("does not resurrect a subscriber who already unsubscribed", func(t *testing.T) {
		db := setupTestDB(t)
		svc := setupService(t, db, newFakeSender())
		if err := svc.Subscribe(ctx, "stale@example.com"); err != nil {
			t.Fatal(err)
		}
		id, _, _, _ := getSubscriberByEmail(t, db, "stale@example.com")
		confirmToken := services.GenerateSubscriptionToken(testTokenSecret, id, services.PurposeConfirm)
		if err := svc.ConfirmSubscription(ctx, id, confirmToken); err != nil {
			t.Fatal(err)
		}
		unsubToken := services.GenerateSubscriptionToken(testTokenSecret, id, services.PurposeUnsubscribe)
		if err := svc.UnsubscribeByToken(ctx, id, unsubToken); err != nil {
			t.Fatal(err)
		}

		// Replaying the old confirm link (e.g. from a stale email) must not
		// silently re-subscribe them.
		if err := svc.ConfirmSubscription(ctx, id, confirmToken); err != nil {
			t.Fatal(err)
		}

		_, _, status, _ := getSubscriberByEmail(t, db, "stale@example.com")
		if status != "unsubscribed" {
			t.Errorf("status = %q, want unsubscribed (unchanged)", status)
		}
	})
}

func TestSubscriptionServiceAuthenticatedSubscribe(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a confirmed, linked subscriber for a new email", func(t *testing.T) {
		db := setupTestDB(t)
		svc := setupService(t, db, newFakeSender())
		userID := insertTestUser(t, db, "member@example.com")

		if err := svc.AuthenticatedSubscribe(ctx, userID, "member@example.com"); err != nil {
			t.Fatal(err)
		}

		_, gotUserID, status, found := getSubscriberByEmail(t, db, "member@example.com")
		if !found {
			t.Fatal("expected a subscribers row")
		}
		if status != "confirmed" {
			t.Errorf("status = %q, want confirmed", status)
		}
		if !gotUserID.Valid || int(gotUserID.Int64) != userID {
			t.Errorf("user_id = %v, want %d", gotUserID, userID)
		}
	})

	t.Run("links and confirms a pre-existing guest row instead of duplicating it", func(t *testing.T) {
		db := setupTestDB(t)
		svc := setupService(t, db, newFakeSender())

		if err := svc.Subscribe(ctx, "converts@example.com"); err != nil {
			t.Fatal(err)
		}
		guestID, _, _, _ := getSubscriberByEmail(t, db, "converts@example.com")

		userID := insertTestUser(t, db, "converts@example.com")
		if err := svc.AuthenticatedSubscribe(ctx, userID, "converts@example.com"); err != nil {
			t.Fatal(err)
		}

		linkedID, gotUserID, status, found := getSubscriberByEmail(t, db, "converts@example.com")
		if !found {
			t.Fatal("expected the subscribers row to still exist")
		}
		if linkedID != guestID {
			t.Errorf("expected the same row (id %d) to be reused, got id %d", guestID, linkedID)
		}
		if status != "confirmed" {
			t.Errorf("status = %q, want confirmed", status)
		}
		if !gotUserID.Valid || int(gotUserID.Int64) != userID {
			t.Errorf("user_id = %v, want %d", gotUserID, userID)
		}

		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM subscribers WHERE email = ?", "converts@example.com").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("subscriber row count = %d, want 1 (no duplicate)", count)
		}
	})

	t.Run("is idempotent when already subscribed", func(t *testing.T) {
		db := setupTestDB(t)
		svc := setupService(t, db, newFakeSender())
		userID := insertTestUser(t, db, "already@example.com")

		if err := svc.AuthenticatedSubscribe(ctx, userID, "already@example.com"); err != nil {
			t.Fatal(err)
		}
		if err := svc.AuthenticatedSubscribe(ctx, userID, "already@example.com"); err != nil {
			t.Fatal(err)
		}

		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM subscribers WHERE email = ?", "already@example.com").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("subscriber row count = %d, want 1", count)
		}
	})
}

func TestSubscriptionServiceAuthenticatedUnsubscribe(t *testing.T) {
	ctx := context.Background()

	t.Run("unsubscribes the current user's row", func(t *testing.T) {
		db := setupTestDB(t)
		svc := setupService(t, db, newFakeSender())
		userID := insertTestUser(t, db, "unsub@example.com")
		if err := svc.AuthenticatedSubscribe(ctx, userID, "unsub@example.com"); err != nil {
			t.Fatal(err)
		}

		if err := svc.AuthenticatedUnsubscribe(ctx, userID); err != nil {
			t.Fatal(err)
		}

		_, _, status, _ := getSubscriberByEmail(t, db, "unsub@example.com")
		if status != "unsubscribed" {
			t.Errorf("status = %q, want unsubscribed", status)
		}
	})

	t.Run("is a no-op when the user was never subscribed", func(t *testing.T) {
		db := setupTestDB(t)
		svc := setupService(t, db, newFakeSender())
		userID := insertTestUser(t, db, "neversubbed@example.com")

		if err := svc.AuthenticatedUnsubscribe(ctx, userID); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSubscriptionServiceUnsubscribeByToken(t *testing.T) {
	ctx := context.Background()

	t.Run("unsubscribes with a valid token", func(t *testing.T) {
		db := setupTestDB(t)
		svc := setupService(t, db, newFakeSender())
		if err := svc.Subscribe(ctx, "tokenunsub@example.com"); err != nil {
			t.Fatal(err)
		}
		id, _, _, _ := getSubscriberByEmail(t, db, "tokenunsub@example.com")
		token := services.GenerateSubscriptionToken(testTokenSecret, id, services.PurposeUnsubscribe)

		if err := svc.UnsubscribeByToken(ctx, id, token); err != nil {
			t.Fatal(err)
		}

		_, _, status, _ := getSubscriberByEmail(t, db, "tokenunsub@example.com")
		if status != "unsubscribed" {
			t.Errorf("status = %q, want unsubscribed", status)
		}
	})

	t.Run("rejects an invalid token", func(t *testing.T) {
		db := setupTestDB(t)
		svc := setupService(t, db, newFakeSender())
		if err := svc.Subscribe(ctx, "badunsub@example.com"); err != nil {
			t.Fatal(err)
		}
		id, _, _, _ := getSubscriberByEmail(t, db, "badunsub@example.com")

		err := svc.UnsubscribeByToken(ctx, id, "not-the-real-token")
		if err != services.ErrInvalidToken {
			t.Errorf("err = %v, want ErrInvalidToken", err)
		}
	})
}

func TestSubscriptionServiceBroadcast(t *testing.T) {
	ctx := context.Background()

	t.Run("sends only to confirmed subscribers and reports counts", func(t *testing.T) {
		db := setupTestDB(t)
		sender := newFakeSender()
		svc := setupService(t, db, sender)
		postID := insertTestPost(t, db, "cryptography-101/hashing", "Hashing 101", "post body")

		for _, e := range []string{"a@example.com", "b@example.com"} {
			if err := svc.Subscribe(ctx, e); err != nil {
				t.Fatal(err)
			}
			id, _, _, _ := getSubscriberByEmail(t, db, e)
			token := services.GenerateSubscriptionToken(testTokenSecret, id, services.PurposeConfirm)
			if err := svc.ConfirmSubscription(ctx, id, token); err != nil {
				t.Fatal(err)
			}
		}
		if err := svc.Subscribe(ctx, "notconfirmed@example.com"); err != nil {
			t.Fatal(err)
		}

		messagesBeforeBroadcast := len(sender.messages())

		sent, failed, err := svc.Broadcast(ctx, postID)
		if err != nil {
			t.Fatal(err)
		}
		if sent != 2 {
			t.Errorf("sent = %d, want 2", sent)
		}
		if failed != 0 {
			t.Errorf("failed = %d, want 0", failed)
		}

		gotNewMessages := len(sender.messages()) - messagesBeforeBroadcast
		if gotNewMessages != 2 {
			t.Errorf("new messages sent by broadcast = %d, want 2 (not_confirmed@ must be excluded)", gotNewMessages)
		}
	})

	t.Run("continues past individual failures and reports them", func(t *testing.T) {
		db := setupTestDB(t)
		sender := newFakeSender()
		svc := setupService(t, db, sender)
		postID := insertTestPost(t, db, "cryptography-101/encryption", "Encryption 101", "post body")

		for _, e := range []string{"ok@example.com", "fails@example.com"} {
			if err := svc.Subscribe(ctx, e); err != nil {
				t.Fatal(err)
			}
			id, _, _, _ := getSubscriberByEmail(t, db, e)
			token := services.GenerateSubscriptionToken(testTokenSecret, id, services.PurposeConfirm)
			if err := svc.ConfirmSubscription(ctx, id, token); err != nil {
				t.Fatal(err)
			}
		}
		sender.failFor["fails@example.com"] = true

		sent, failed, err := svc.Broadcast(ctx, postID)
		if err != nil {
			t.Fatal(err)
		}
		if sent != 1 {
			t.Errorf("sent = %d, want 1", sent)
		}
		if failed != 1 {
			t.Errorf("failed = %d, want 1", failed)
		}
	})

	t.Run("returns an error for a post with no content", func(t *testing.T) {
		db := setupTestDB(t)
		svc := setupService(t, db, newFakeSender())

		_, _, err := svc.Broadcast(ctx, 99999)
		if err != services.ErrPostNotFound {
			t.Errorf("err = %v, want ErrPostNotFound", err)
		}
	})
}
