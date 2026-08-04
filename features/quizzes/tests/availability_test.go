package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"arkana/features/quizzes/services"
)

// TestAvailabilityService covers the service-level Availability logic
// directly (no HTTP), exercising the strict per-language coverage rule:
// a language only counts as available if every question currently in the
// module's pool has a translation row for it.
func TestAvailabilityService(t *testing.T) {
	t.Run("unknown module returns ErrModuleNotFound", func(t *testing.T) {
		db := setupTestDB(t)
		sessions := services.NewQuizSessionService(db, setupTestRedis(t))

		_, _, err := sessions.Availability("no-such-list", "no-such-module")
		if err != services.ErrModuleNotFound {
			t.Fatalf("err = %v, want ErrModuleNotFound", err)
		}
	})

	t.Run("module with no questions is unavailable", func(t *testing.T) {
		db := setupTestDB(t)
		sessions := services.NewQuizSessionService(db, setupTestRedis(t))
		insertTestModule(t, db, "blockchain-101", "bitcoin-and-fundamentals", "how-it-all-began", "blockchain-101/how-it-all-began")

		available, languages, err := sessions.Availability("blockchain-101", "bitcoin-and-fundamentals")
		if err != nil {
			t.Fatal(err)
		}
		if available {
			t.Fatal("available = true, want false")
		}
		if len(languages) != 0 {
			t.Fatalf("languages = %v, want empty", languages)
		}
	})

	t.Run("only languages with full coverage over the pool are reported", func(t *testing.T) {
		db := setupTestDB(t)
		sessions := services.NewQuizSessionService(db, setupTestRedis(t))
		insertTestModule(t, db, "blockchain-101", "bitcoin-and-fundamentals", "how-it-all-began", "blockchain-101/how-it-all-began")
		postID := insertTestPost(t, db, "blockchain-101/how-it-all-began")

		q1 := insertTestQuestion(t, db, "q1", postID)
		q2 := insertTestQuestion(t, db, "q2", postID)

		// Both questions are translated into "en", so "en" should be
		// fully covered. Only q1 is translated into "es", so "es" must
		// NOT be reported - a session in "es" would 500 on q2.
		insertTestQuestionTranslation(t, db, q1, "en", "q1 prompt", "{}")
		insertTestQuestionTranslation(t, db, q2, "en", "q2 prompt", "{}")
		insertTestQuestionTranslation(t, db, q1, "es", "q1 prompt es", "{}")

		available, languages, err := sessions.Availability("blockchain-101", "bitcoin-and-fundamentals")
		if err != nil {
			t.Fatal(err)
		}
		if !available {
			t.Fatal("available = false, want true")
		}
		sort.Strings(languages)
		if len(languages) != 1 || languages[0] != "en" {
			t.Fatalf("languages = %v, want [en]", languages)
		}
	})

	t.Run("fully translated pool reports every language", func(t *testing.T) {
		db := setupTestDB(t)
		sessions := services.NewQuizSessionService(db, setupTestRedis(t))
		insertTestModule(t, db, "blockchain-101", "bitcoin-and-fundamentals", "how-it-all-began", "blockchain-101/how-it-all-began")
		postID := insertTestPost(t, db, "blockchain-101/how-it-all-began")

		q1 := insertTestQuestion(t, db, "q1", postID)
		for _, lang := range []string{"en", "es", "pt"} {
			insertTestQuestionTranslation(t, db, q1, lang, "prompt", "{}")
		}

		available, languages, err := sessions.Availability("blockchain-101", "bitcoin-and-fundamentals")
		if err != nil {
			t.Fatal(err)
		}
		if !available {
			t.Fatal("available = false, want true")
		}
		sort.Strings(languages)
		want := []string{"en", "es", "pt"}
		if len(languages) != len(want) {
			t.Fatalf("languages = %v, want %v", languages, want)
		}
		for i, lang := range want {
			if languages[i] != lang {
				t.Fatalf("languages = %v, want %v", languages, want)
			}
		}
	})
}

// TestAvailabilityHandler drives the same checks over real HTTP, without
// any Authorization header - confirming the route is genuinely public.
func TestAvailabilityHandler(t *testing.T) {
	t.Run("unknown module returns 404", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupQuizRouter(t, db)

		req := httptest.NewRequest("GET", "/api/reading-lists/no-such-list/modules/no-such-module/quiz/availability", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("module with a fully translated pool returns available with its languages", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupQuizRouter(t, db)
		insertTestModule(t, db, "blockchain-101", "bitcoin-and-fundamentals", "how-it-all-began", "blockchain-101/how-it-all-began")
		postID := insertTestPost(t, db, "blockchain-101/how-it-all-began")
		q1 := insertTestQuestion(t, db, "q1", postID)
		insertTestQuestionTranslation(t, db, q1, "en", "prompt", "{}")

		req := httptest.NewRequest("GET", "/api/reading-lists/blockchain-101/modules/bitcoin-and-fundamentals/quiz/availability", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Available bool     `json:"available"`
			Languages []string `json:"languages"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if !resp.Available || len(resp.Languages) != 1 || resp.Languages[0] != "en" {
			t.Fatalf("resp = %+v, want available=true languages=[en]", resp)
		}
	})
}
