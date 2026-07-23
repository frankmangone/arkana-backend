package tests

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"arkana/features/subscriptions/models"
	"arkana/features/subscriptions/services"
)

func postJSON(t *testing.T, router http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func signAdminRequest(secret string, body []byte) map[string]string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))
	return map[string]string{"X-Timestamp": timestamp, "X-Signature": signature}
}

func TestSubscribeHandler(t *testing.T) {
	t.Run("subscribes a new guest email", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db, newFakeSender())

		rec := postJSON(t, router, "POST", "/api/subscribe", models.SubscribeRequest{Email: "guest@example.com"}, nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.SubscribeResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.Status != "pending" {
			t.Errorf("status = %q, want pending", resp.Status)
		}
	})

	t.Run("returns identical success for an email matching an existing account", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db, newFakeSender())
		insertTestUser(t, db, "hasaccount@example.com")

		rec := postJSON(t, router, "POST", "/api/subscribe", models.SubscribeRequest{Email: "hasaccount@example.com"}, nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.SubscribeResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.Status != "pending" {
			t.Errorf("status = %q, want pending (indistinguishable from a real signup)", resp.Status)
		}
	})

	t.Run("rejects a missing email", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db, newFakeSender())

		rec := postJSON(t, router, "POST", "/api/subscribe", models.SubscribeRequest{}, nil)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})
}

func TestConfirmHandler(t *testing.T) {
	t.Run("confirms with a valid token", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db, newFakeSender())
		postJSON(t, router, "POST", "/api/subscribe", models.SubscribeRequest{Email: "confirmhandler@example.com"}, nil)
		id, _, _, _ := getSubscriberByEmail(t, db, "confirmhandler@example.com")
		token := services.GenerateSubscriptionToken(testTokenSecret, id, services.PurposeConfirm)

		rec := postJSON(t, router, "POST", "/api/subscribe/confirm", models.ConfirmRequest{SubscriberID: id, Token: token}, nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.ConfirmResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if !resp.Confirmed {
			t.Error("confirmed = false, want true")
		}
	})

	t.Run("rejects an invalid token", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db, newFakeSender())
		postJSON(t, router, "POST", "/api/subscribe", models.SubscribeRequest{Email: "badconfirmhandler@example.com"}, nil)
		id, _, _, _ := getSubscriberByEmail(t, db, "badconfirmhandler@example.com")

		rec := postJSON(t, router, "POST", "/api/subscribe/confirm", models.ConfirmRequest{SubscriberID: id, Token: "invalid"}, nil)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})
}

func TestAuthenticatedSubscribeHandler(t *testing.T) {
	t.Run("subscribes the authenticated user", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db, newFakeSender())
		userID := insertTestUser(t, db, "authsub@example.com")
		token := generateTestJWT(t, userID, "authsub@example.com")

		rec := postJSON(t, router, "POST", "/api/subscriptions", nil, map[string]string{"Authorization": "Bearer " + token})

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.SubscriptionStatusResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if !resp.Subscribed {
			t.Error("subscribed = false, want true")
		}
	})

	t.Run("rejects an unauthenticated request", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db, newFakeSender())

		rec := postJSON(t, router, "POST", "/api/subscriptions", nil, nil)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}

func TestAuthenticatedUnsubscribeHandler(t *testing.T) {
	t.Run("unsubscribes the authenticated user", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db, newFakeSender())
		userID := insertTestUser(t, db, "authunsub@example.com")
		token := generateTestJWT(t, userID, "authunsub@example.com")
		postJSON(t, router, "POST", "/api/subscriptions", nil, map[string]string{"Authorization": "Bearer " + token})

		rec := postJSON(t, router, "DELETE", "/api/subscriptions", nil, map[string]string{"Authorization": "Bearer " + token})

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.SubscriptionStatusResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.Subscribed {
			t.Error("subscribed = true, want false")
		}
	})

	t.Run("rejects an unauthenticated request", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db, newFakeSender())

		rec := postJSON(t, router, "DELETE", "/api/subscriptions", nil, nil)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}

func TestUnsubscribeHandler(t *testing.T) {
	t.Run("unsubscribes with a valid token", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db, newFakeSender())
		postJSON(t, router, "POST", "/api/subscribe", models.SubscribeRequest{Email: "unsubhandler@example.com"}, nil)
		id, _, _, _ := getSubscriberByEmail(t, db, "unsubhandler@example.com")
		token := services.GenerateSubscriptionToken(testTokenSecret, id, services.PurposeUnsubscribe)

		rec := postJSON(t, router, "POST", "/api/subscriptions/unsubscribe", models.ConfirmRequest{SubscriberID: id, Token: token}, nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.UnsubscribeResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if !resp.Unsubscribed {
			t.Error("unsubscribed = false, want true")
		}
	})

	t.Run("rejects an invalid token", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db, newFakeSender())
		postJSON(t, router, "POST", "/api/subscribe", models.SubscribeRequest{Email: "badunsubhandler@example.com"}, nil)
		id, _, _, _ := getSubscriberByEmail(t, db, "badunsubhandler@example.com")

		rec := postJSON(t, router, "POST", "/api/subscriptions/unsubscribe", models.ConfirmRequest{SubscriberID: id, Token: "invalid"}, nil)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})
}

func TestBroadcastHandler(t *testing.T) {
	t.Run("sends a broadcast with a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db, newFakeSender())
		postID := insertTestPost(t, db, "cryptography-101/hashing", "Hashing 101", "content")
		postJSON(t, router, "POST", "/api/subscribe", models.SubscribeRequest{Email: "broadcastrecipient@example.com"}, nil)
		id, _, _, _ := getSubscriberByEmail(t, db, "broadcastrecipient@example.com")
		token := services.GenerateSubscriptionToken(testTokenSecret, id, services.PurposeConfirm)
		postJSON(t, router, "POST", "/api/subscribe/confirm", models.ConfirmRequest{SubscriberID: id, Token: token}, nil)

		body, _ := json.Marshal(models.BroadcastRequest{PostID: postID})
		headers := signAdminRequest(testAdminSecret, body)
		req := httptest.NewRequest("POST", "/api/admin/subscriptions/broadcast", bytes.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.BroadcastResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.Sent != 1 {
			t.Errorf("sent = %d, want 1", resp.Sent)
		}
	})

	t.Run("rejects a request without a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db, newFakeSender())
		postID := insertTestPost(t, db, "cryptography-101/no-sig", "No Sig", "content")

		body, _ := json.Marshal(models.BroadcastRequest{PostID: postID})
		req := httptest.NewRequest("POST", "/api/admin/subscriptions/broadcast", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("returns 404 for a nonexistent post", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db, newFakeSender())

		body, _ := json.Marshal(models.BroadcastRequest{PostID: 99999})
		headers := signAdminRequest(testAdminSecret, body)
		req := httptest.NewRequest("POST", "/api/admin/subscriptions/broadcast", bytes.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
		}
	})
}
