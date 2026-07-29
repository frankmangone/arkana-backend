package tests

import (
	"database/sql"
	"encoding/json"
	"testing"

	"arkana/features/quizzes/services"
)

// seedSingleQuestionAttempt seeds exactly one question (of the given type
// and answer_key) linked to one post/module, starts an attempt against
// it, and returns everything a test needs to call Answer directly.
func seedSingleQuestionAttempt(t *testing.T, qType, answerKey string) (db *sql.DB, svc *services.QuizSessionService, userID int, attemptUUID, questionUUID string) {
	t.Helper()
	db = setupTestDB(t)
	userID = insertTestUser(t, db, "learner@example.com")
	insertTestModule(t, db, "list-1", "module-1", "item-1", "list-1/item-1")
	postID := insertTestPost(t, db, "list-1/item-1")

	questionUUID = "question-uuid"
	res, err := db.Exec(
		`INSERT INTO questions (uuid, slug, type, difficulty, answer_key) VALUES (?, 'q1', ?, 1, ?)`,
		questionUUID, qType, answerKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	questionID, _ := res.LastInsertId()
	if _, err := db.Exec("INSERT INTO question_posts (question_id, post_id) VALUES (?, ?)", questionID, postID); err != nil {
		t.Fatal(err)
	}
	insertTestQuestionTranslation(t, db, int(questionID), "en", "prompt", `{"explanation":"read again"}`)

	svc = services.NewQuizSessionService(db)
	attemptUUID, total, err := svc.Start(userID, "list-1", "module-1")
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	return db, svc, userID, attemptUUID, questionUUID
}

func TestQuizSessionServiceAnswerGrading(t *testing.T) {
	t.Run("single_choice: correct selection", func(t *testing.T) {
		_, svc, userID, attemptUUID, questionUUID := seedSingleQuestionAttempt(t, "single_choice", `{"correctOptionIds":["b"]}`)
		result, err := svc.Answer(userID, attemptUUID, questionUUID, json.RawMessage(`{"selectedOptionIds":["b"]}`), false, "en")
		if err != nil {
			t.Fatal(err)
		}
		if !result.Correct {
			t.Fatal("Correct = false, want true")
		}
	})

	t.Run("single_choice: incorrect selection reveals the correct option", func(t *testing.T) {
		_, svc, userID, attemptUUID, questionUUID := seedSingleQuestionAttempt(t, "single_choice", `{"correctOptionIds":["b"]}`)
		result, err := svc.Answer(userID, attemptUUID, questionUUID, json.RawMessage(`{"selectedOptionIds":["a"]}`), false, "en")
		if err != nil {
			t.Fatal(err)
		}
		if result.Correct {
			t.Fatal("Correct = true, want false")
		}
		var reveal map[string][]string
		if err := json.Unmarshal(result.CorrectReveal, &reveal); err != nil {
			t.Fatal(err)
		}
		if len(reveal["correctOptionIds"]) != 1 || reveal["correctOptionIds"][0] != "b" {
			t.Fatalf("reveal = %v, want correctOptionIds=[b]", reveal)
		}
	})

	t.Run("matching: correct assignments", func(t *testing.T) {
		_, svc, userID, attemptUUID, questionUUID := seedSingleQuestionAttempt(t, "matching", `{"correctAssignments":{"left1":"right1"}}`)
		result, err := svc.Answer(userID, attemptUUID, questionUUID, json.RawMessage(`{"assignments":{"left1":"right1"}}`), false, "en")
		if err != nil {
			t.Fatal(err)
		}
		if !result.Correct {
			t.Fatal("Correct = false, want true")
		}
	})

	t.Run("bucket_sort: shares matching's assignment shape", func(t *testing.T) {
		_, svc, userID, attemptUUID, questionUUID := seedSingleQuestionAttempt(t, "bucket_sort", `{"correctAssignments":{"item1":"bucketA"}}`)
		result, err := svc.Answer(userID, attemptUUID, questionUUID, json.RawMessage(`{"assignments":{"item1":"bucketB"}}`), false, "en")
		if err != nil {
			t.Fatal(err)
		}
		if result.Correct {
			t.Fatal("Correct = true, want false (wrong bucket)")
		}
	})

	t.Run("range: within tolerance counts as correct", func(t *testing.T) {
		_, svc, userID, attemptUUID, questionUUID := seedSingleQuestionAttempt(t, "range", `{"correctValues":{"r1":{"value":10,"tolerance":2}}}`)
		result, err := svc.Answer(userID, attemptUUID, questionUUID, json.RawMessage(`{"values":{"r1":11}}`), false, "en")
		if err != nil {
			t.Fatal(err)
		}
		if !result.Correct {
			t.Fatal("Correct = false, want true (within tolerance)")
		}
	})

	t.Run("sequencing: order must match exactly", func(t *testing.T) {
		_, svc, userID, attemptUUID, questionUUID := seedSingleQuestionAttempt(t, "sequencing", `{"correctOrder":["a","b","c"]}`)
		result, err := svc.Answer(userID, attemptUUID, questionUUID, json.RawMessage(`{"order":["a","c","b"]}`), false, "en")
		if err != nil {
			t.Fatal(err)
		}
		if result.Correct {
			t.Fatal("Correct = true, want false (wrong order)")
		}
	})

	t.Run("fill_blank: words must match exactly", func(t *testing.T) {
		_, svc, userID, attemptUUID, questionUUID := seedSingleQuestionAttempt(t, "fill_blank", `{"correctWords":{"blank1":"nonce"}}`)
		result, err := svc.Answer(userID, attemptUUID, questionUUID, json.RawMessage(`{"filled":{"blank1":"nonce"}}`), false, "en")
		if err != nil {
			t.Fatal(err)
		}
		if !result.Correct {
			t.Fatal("Correct = false, want true")
		}
	})
}

func TestQuizSessionServiceAnswerFlow(t *testing.T) {
	t.Run("skip is graded incorrect, still reveals the answer, and advances position", func(t *testing.T) {
		_, svc, userID, attemptUUID, questionUUID := seedSingleQuestionAttempt(t, "single_choice", `{"correctOptionIds":["b"]}`)
		result, err := svc.Answer(userID, attemptUUID, questionUUID, nil, true, "en")
		if err != nil {
			t.Fatal(err)
		}
		if result.Correct {
			t.Fatal("Correct = true, want false for a skip")
		}
		if !result.Skipped {
			t.Fatal("Skipped = false, want true")
		}
		if result.CorrectReveal == nil {
			t.Fatal("CorrectReveal is nil, want the correct answer even when skipped")
		}
		if !result.AttemptDone {
			t.Fatal("AttemptDone = false, want true (only question in this attempt)")
		}
	})

	t.Run("rejects a questionId that isn't the attempt's current question", func(t *testing.T) {
		_, svc, userID, attemptUUID, _ := seedSingleQuestionAttempt(t, "single_choice", `{"correctOptionIds":["b"]}`)
		_, err := svc.Answer(userID, attemptUUID, "not-the-current-question", json.RawMessage(`{"selectedOptionIds":["b"]}`), false, "en")
		if err != services.ErrWrongQuestion {
			t.Fatalf("err = %v, want ErrWrongQuestion", err)
		}
	})

	t.Run("rejects answering an attempt owned by another user", func(t *testing.T) {
		db, svc, _, attemptUUID, questionUUID := seedSingleQuestionAttempt(t, "single_choice", `{"correctOptionIds":["b"]}`)
		otherUser := insertTestUser(t, db, "other@example.com")
		_, err := svc.Answer(otherUser, attemptUUID, questionUUID, json.RawMessage(`{"selectedOptionIds":["b"]}`), false, "en")
		if err != services.ErrAttemptForbidden {
			t.Fatalf("err = %v, want ErrAttemptForbidden", err)
		}
	})

	t.Run("a correct answer has no reveal, explanation, or reinforcement", func(t *testing.T) {
		_, svc, userID, attemptUUID, questionUUID := seedSingleQuestionAttempt(t, "single_choice", `{"correctOptionIds":["b"]}`)
		result, err := svc.Answer(userID, attemptUUID, questionUUID, json.RawMessage(`{"selectedOptionIds":["b"]}`), false, "en")
		if err != nil {
			t.Fatal(err)
		}
		if result.CorrectReveal != nil || result.Explanation != nil || result.PostPaths != nil {
			t.Fatalf("expected no reveal data on a correct answer, got reveal=%v explanation=%v postPaths=%v", result.CorrectReveal, result.Explanation, result.PostPaths)
		}
	})

	t.Run("answering twice for the same question is blocked", func(t *testing.T) {
		_, svc, userID, attemptUUID, questionUUID := seedSingleQuestionAttempt(t, "single_choice", `{"correctOptionIds":["b"]}`)
		if _, err := svc.Answer(userID, attemptUUID, questionUUID, json.RawMessage(`{"selectedOptionIds":["b"]}`), false, "en"); err != nil {
			t.Fatal(err)
		}
		// Attempt is already done after the one question; a second call
		// must fail via ErrAttemptCompleted, not silently re-grade.
		_, err := svc.Answer(userID, attemptUUID, questionUUID, json.RawMessage(`{"selectedOptionIds":["b"]}`), false, "en")
		if err != services.ErrAttemptCompleted {
			t.Fatalf("err = %v, want ErrAttemptCompleted", err)
		}
	})
}
