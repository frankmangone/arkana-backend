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
	"strings"
	"testing"
	"time"

	"arkana/features/questionflags/models"
)

func signAdminRequest(secret string, body []byte) map[string]string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))
	return map[string]string{"X-Timestamp": timestamp, "X-Signature": signature}
}

func TestCreateFlagHandler(t *testing.T) {
	db := setupTestDB(t)
	router := setupRouter(t, db)
	userID := insertTestUser(t, db, "flagger@example.com")
	token := generateTestJWT(t, userID, "flagger@example.com")
	insertTestQuestion(t, db, "question-uuid-1", "question-slug-1")

	t.Run("creates a flag", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"reason": "The answer key looks wrong"})
		req := httptest.NewRequest("POST", "/api/questions/question-uuid-1/flags", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
		}

		var flag models.QuestionFlag
		json.NewDecoder(rec.Body).Decode(&flag)
		if flag.Reason != "The answer key looks wrong" {
			t.Errorf("reason = %q, want %q", flag.Reason, "The answer key looks wrong")
		}
		if flag.UserID != userID {
			t.Errorf("user_id = %d, want %d", flag.UserID, userID)
		}
	})

	t.Run("overwrites the reason on a second flag from the same user", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"reason": "Actually, the wording is just confusing"})
		req := httptest.NewRequest("POST", "/api/questions/question-uuid-1/flags", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
		}

		var count int
		db.QueryRow("SELECT COUNT(*) FROM question_flags").Scan(&count)
		if count != 1 {
			t.Errorf("question_flags row count = %d, want 1 (upsert, not insert)", count)
		}
	})

	t.Run("rejects an empty reason", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"reason": ""})
		req := httptest.NewRequest("POST", "/api/questions/question-uuid-1/flags", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("returns 404 for an unknown question uuid", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"reason": "wrong"})
		req := httptest.NewRequest("POST", "/api/questions/does-not-exist/flags", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejects an oversized reason", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"reason": strings.Repeat("x", 1001)})
		req := httptest.NewRequest("POST", "/api/questions/question-uuid-1/flags", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"reason": "wrong"})
		req := httptest.NewRequest("POST", "/api/questions/question-uuid-1/flags", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}

func TestListFlagsHandler(t *testing.T) {
	db := setupTestDB(t)
	router := setupRouter(t, db)
	userID := insertTestUser(t, db, "lister@example.com")
	insertTestQuestion(t, db, "question-uuid-2", "question-slug-2")
	db.Exec(
		"INSERT INTO question_flags (question_id, user_id, reason) VALUES (1, ?, 'too easy')",
		userID,
	)

	t.Run("lists flags with a valid HMAC signature", func(t *testing.T) {
		headers := signAdminRequest(testAdminSecret, nil)
		req := httptest.NewRequest("GET", "/api/admin/question-flags", nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		var resp models.ListFlagsResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if len(resp.Flags) != 1 {
			t.Fatalf("len(flags) = %d, want 1", len(resp.Flags))
		}
		if resp.Flags[0].QuestionUUID != "question-uuid-2" {
			t.Errorf("question_uuid = %q, want %q", resp.Flags[0].QuestionUUID, "question-uuid-2")
		}
		if resp.Flags[0].UserEmail != "lister@example.com" {
			t.Errorf("user_email = %q, want %q", resp.Flags[0].UserEmail, "lister@example.com")
		}
	})

	t.Run("rejects a request without a valid signature", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/question-flags", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}

func TestDeleteFlagsHandler(t *testing.T) {
	t.Run("flushes all flags", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db)
		userID := insertTestUser(t, db, "deleter@example.com")
		insertTestQuestion(t, db, "question-uuid-3", "question-slug-3")
		db.Exec("INSERT INTO question_flags (question_id, user_id, reason) VALUES (1, ?, 'too hard')", userID)

		headers := signAdminRequest(testAdminSecret, nil)
		req := httptest.NewRequest("DELETE", "/api/admin/question-flags", nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		var resp models.DeleteFlagsResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.Deleted != 1 {
			t.Errorf("deleted = %d, want 1", resp.Deleted)
		}

		var count int
		db.QueryRow("SELECT COUNT(*) FROM question_flags").Scan(&count)
		if count != 0 {
			t.Errorf("remaining flags = %d, want 0", count)
		}
	})

	t.Run("deletes a single flag by id", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db)
		user1 := insertTestUser(t, db, "keep@example.com")
		user2 := insertTestUser(t, db, "remove@example.com")
		insertTestQuestion(t, db, "question-uuid-4", "question-slug-4")
		db.Exec("INSERT INTO question_flags (question_id, user_id, reason) VALUES (1, ?, 'keep this one')", user1)
		db.Exec("INSERT INTO question_flags (question_id, user_id, reason) VALUES (1, ?, 'remove this one')", user2)

		var removeID int
		db.QueryRow("SELECT id FROM question_flags WHERE user_id = ?", user2).Scan(&removeID)

		headers := signAdminRequest(testAdminSecret, nil)
		req := httptest.NewRequest("DELETE", "/api/admin/question-flags?id="+strconv.Itoa(removeID), nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		var count int
		db.QueryRow("SELECT COUNT(*) FROM question_flags").Scan(&count)
		if count != 1 {
			t.Errorf("remaining flags = %d, want 1", count)
		}
	})

	t.Run("rejects a request without a valid signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db)

		req := httptest.NewRequest("DELETE", "/api/admin/question-flags", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}
