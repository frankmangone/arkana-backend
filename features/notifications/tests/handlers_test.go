package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"arkana/features/notifications/models"
	"arkana/features/notifications/services"
)

func TestListNotificationsHandler(t *testing.T) {
	db := setupTestDB(t)
	router := setupRouter(t, db)
	svc := services.NewNotificationService(db)
	userID := insertTestUser(t, db, "listhandler@example.com")
	actor := insertTestUser(t, db, "listhandleractor@example.com")
	token := generateTestJWT(t, userID, "listhandler@example.com")

	postID := 1
	if err := svc.Create(db, userID, actor, models.TypePostLiked, &postID, nil); err != nil {
		t.Fatal(err)
	}

	t.Run("lists the caller's notifications", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/notifications", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		var resp models.NotificationsResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.Total != 1 {
			t.Errorf("total = %d, want 1", resp.Total)
		}
	})

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/notifications", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}

func TestMarkReadHandler(t *testing.T) {
	db := setupTestDB(t)
	router := setupRouter(t, db)
	svc := services.NewNotificationService(db)
	userID := insertTestUser(t, db, "markreadhandler@example.com")
	actor := insertTestUser(t, db, "markreadhandleractor@example.com")
	token := generateTestJWT(t, userID, "markreadhandler@example.com")

	postID := 1
	if err := svc.Create(db, userID, actor, models.TypePostLiked, &postID, nil); err != nil {
		t.Fatal(err)
	}
	page, _ := svc.List(userID, 20, 0)
	notifID := page.Notifications[0].ID

	t.Run("marks a notification read", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/notifications/%d/read", notifID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		var resp models.MarkReadResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if !resp.Read {
			t.Error("read = false, want true")
		}
	})

	t.Run("returns 404 for another user's notification", func(t *testing.T) {
		otherUser := insertTestUser(t, db, "othermarkread@example.com")
		otherToken := generateTestJWT(t, otherUser, "othermarkread@example.com")

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/notifications/%d/read", notifID), nil)
		req.Header.Set("Authorization", "Bearer "+otherToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/notifications/%d/read", notifID), nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}

func TestUnreadCountHandler(t *testing.T) {
	db := setupTestDB(t)
	router := setupRouter(t, db)
	svc := services.NewNotificationService(db)
	userID := insertTestUser(t, db, "unreadhandler@example.com")
	actor := insertTestUser(t, db, "unreadhandleractor@example.com")
	token := generateTestJWT(t, userID, "unreadhandler@example.com")

	postID := 1
	if err := svc.Create(db, userID, actor, models.TypePostLiked, &postID, nil); err != nil {
		t.Fatal(err)
	}

	t.Run("returns unread count", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/notifications/unread-count", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		var resp models.UnreadCountResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.Count != 1 {
			t.Errorf("count = %d, want 1", resp.Count)
		}
	})

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/notifications/unread-count", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}
