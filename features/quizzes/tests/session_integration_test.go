package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestQuizSessionHappyPath drives the entire session flow over real HTTP
// requests through the router - start, then next/answer in a loop
// (mixing a correct answer and a skip) until done, then complete -
// exercising every handler from Tasks 4-7 together in the shape a real
// client would actually call them.
func TestQuizSessionHappyPath(t *testing.T) {
	db := setupTestDB(t)
	router := setupQuizRouter(t, db)

	userID := insertTestUser(t, db, "learner@example.com")
	insertTestModule(t, db, "blockchain-101", "bitcoin-and-fundamentals", "how-it-all-began", "blockchain-101/how-it-all-began")
	postID := insertTestPost(t, db, "blockchain-101/how-it-all-began")

	q1, err := db.Exec(`INSERT INTO questions (uuid, slug, type, difficulty, answer_key) VALUES ('q1-uuid', 'q1', 'single_choice', 1, '{"correctOptionIds":["b"]}')`)
	if err != nil {
		t.Fatal(err)
	}
	q1ID, _ := q1.LastInsertId()
	if _, err := db.Exec("INSERT INTO question_posts (question_id, post_id) VALUES (?, ?)", q1ID, postID); err != nil {
		t.Fatal(err)
	}
	insertTestQuestionTranslation(t, db, int(q1ID), "en", "q1 prompt", `{}`)

	// Same correct answer ("b") as q1 - the selector shuffles which
	// question lands at position 0, and the loop below answers whichever
	// question it's handed first with {"selectedOptionIds":["b"]}
	// unconditionally, so both questions must agree on what's correct
	// regardless of which one that turns out to be.
	q2, err := db.Exec(`INSERT INTO questions (uuid, slug, type, difficulty, answer_key) VALUES ('q2-uuid', 'q2', 'single_choice', 1, '{"correctOptionIds":["b"]}')`)
	if err != nil {
		t.Fatal(err)
	}
	q2ID, _ := q2.LastInsertId()
	if _, err := db.Exec("INSERT INTO question_posts (question_id, post_id) VALUES (?, ?)", q2ID, postID); err != nil {
		t.Fatal(err)
	}
	insertTestQuestionTranslation(t, db, int(q2ID), "en", "q2 prompt", `{}`)

	token := generateTestJWT(t, userID, "learner@example.com")
	authed := func(method, path string, body []byte) *httptest.ResponseRecorder {
		var req *http.Request
		if body != nil {
			req = httptest.NewRequest(method, path, bytes.NewReader(body))
		} else {
			req = httptest.NewRequest(method, path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	startRec := authed("POST", "/api/reading-lists/blockchain-101/modules/bitcoin-and-fundamentals/quiz/attempts", nil)
	if startRec.Code != http.StatusOK {
		t.Fatalf("start status = %d, body = %s", startRec.Code, startRec.Body.String())
	}
	var start struct {
		AttemptID      string `json:"attemptId"`
		TotalQuestions int    `json:"totalQuestions"`
	}
	if err := json.Unmarshal(startRec.Body.Bytes(), &start); err != nil {
		t.Fatal(err)
	}
	if start.TotalQuestions != 2 {
		t.Fatalf("TotalQuestions = %d, want 2", start.TotalQuestions)
	}

	answeredCorrectly := 0
	for i := 0; i < start.TotalQuestions; i++ {
		nextRec := authed("GET", "/api/quiz-attempts/"+start.AttemptID+"/next?lang=en", nil)
		if nextRec.Code != http.StatusOK {
			t.Fatalf("next status = %d, body = %s", nextRec.Code, nextRec.Body.String())
		}
		var next struct {
			Question *struct {
				UUID string `json:"uuid"`
			} `json:"question"`
			Done bool `json:"done"`
		}
		if err := json.Unmarshal(nextRec.Body.Bytes(), &next); err != nil {
			t.Fatal(err)
		}
		if next.Done || next.Question == nil {
			t.Fatalf("next returned done=%v at i=%d, expected a question", next.Done, i)
		}

		// First question answered correctly, the second skipped -
		// exercises both resolution paths in one pass.
		var answerBody []byte
		if i == 0 {
			answerBody, _ = json.Marshal(map[string]any{
				"questionId": next.Question.UUID,
				"response":   map[string]any{"selectedOptionIds": []string{"b"}},
			})
		} else {
			answerBody, _ = json.Marshal(map[string]any{
				"questionId": next.Question.UUID,
				"skipped":    true,
			})
		}
		answerRec := authed("POST", "/api/quiz-attempts/"+start.AttemptID+"/answers?lang=en", answerBody)
		if answerRec.Code != http.StatusOK {
			t.Fatalf("answer status = %d, body = %s", answerRec.Code, answerRec.Body.String())
		}
		var answer struct {
			Correct     bool `json:"correct"`
			AttemptDone bool `json:"attemptDone"`
		}
		if err := json.Unmarshal(answerRec.Body.Bytes(), &answer); err != nil {
			t.Fatal(err)
		}
		if answer.Correct {
			answeredCorrectly++
		}
		if i == start.TotalQuestions-1 && !answer.AttemptDone {
			t.Fatal("expected AttemptDone=true on the final answer")
		}
	}
	if answeredCorrectly != 1 {
		t.Fatalf("answeredCorrectly = %d, want 1", answeredCorrectly)
	}

	completeRec := authed("POST", "/api/quiz-attempts/"+start.AttemptID+"/complete", nil)
	if completeRec.Code != http.StatusOK {
		t.Fatalf("complete status = %d, body = %s", completeRec.Code, completeRec.Body.String())
	}
	var complete struct {
		Score  int  `json:"score"`
		Passed bool `json:"passed"`
	}
	if err := json.Unmarshal(completeRec.Body.Bytes(), &complete); err != nil {
		t.Fatal(err)
	}
	if complete.Score != 50 {
		t.Fatalf("Score = %d, want 50 (1 of 2 correct)", complete.Score)
	}
	if complete.Passed {
		t.Fatal("Passed = true, want false (50%% is below the 70%% passThreshold)")
	}
}
