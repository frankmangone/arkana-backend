package tests

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"arkana/features/auth/middlewares"
	"arkana/features/quizzes/handlers"
	"arkana/features/quizzes/models"
	"arkana/features/quizzes/services"
	"arkana/shared/adminauth"

	"github.com/gorilla/mux"
)

func signAdminRequest(secret string, body []byte) map[string]string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))
	return map[string]string{"X-Timestamp": timestamp, "X-Signature": signature}
}

func setupQuizRouter(t *testing.T, db *sql.DB) *mux.Router {
	t.Helper()
	router := mux.NewRouter()
	adminAuth := adminauth.NewAdminAuthMiddleware(testAdminSecret)
	auth := middlewares.NewAuthMiddleware(testJWTSecret)
	questions := services.NewQuestionService(db, &fakePostChecker{}, &fakeTagChecker{})
	sessions := services.NewQuizSessionService(db)
	handlers.RegisterRoutes(router, questions, sessions, auth, adminAuth)
	return router
}

func TestPublishQuestionsHandler(t *testing.T) {
	t.Run("publishes questions with a valid HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupQuizRouter(t, db)

		body, _ := json.Marshal(models.QuestionPublishRequest{
			Questions: []models.QuestionPayload{
				{
					Slug: "q1", Type: "single_choice", Difficulty: 1,
					AnswerKey: json.RawMessage(`{"correctOptionIds":["a"]}`),
					Translations: map[string]models.QuestionTranslationPayload{
						"en": {Prompt: "p", Content: json.RawMessage(`{}`)},
					},
				},
			},
		})
		headers := signAdminRequest(testAdminSecret, body)
		req := httptest.NewRequest("POST", "/api/admin/questions", bytes.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var resp models.QuestionPublishResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp.Published != 1 {
			t.Errorf("Published = %d, want 1", resp.Published)
		}
	})

	t.Run("rejects a missing HMAC signature", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupQuizRouter(t, db)

		body, _ := json.Marshal(models.QuestionPublishRequest{})
		req := httptest.NewRequest("POST", "/api/admin/questions", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("rejects a question with an empty slug", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupQuizRouter(t, db)

		body, _ := json.Marshal(models.QuestionPublishRequest{
			Questions: []models.QuestionPayload{{Slug: "", Type: "single_choice", Difficulty: 1, AnswerKey: json.RawMessage(`{}`)}},
		})
		headers := signAdminRequest(testAdminSecret, body)
		req := httptest.NewRequest("POST", "/api/admin/questions", bytes.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})
}
