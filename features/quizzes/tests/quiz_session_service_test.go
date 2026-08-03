package tests

import (
	"database/sql"
	"encoding/json"
	"testing"

	"arkana/features/quizzes/services"
)

func TestQuizSessionServiceStart(t *testing.T) {
	t.Run("creates an attempt and persists the full pick-order", func(t *testing.T) {
		db := setupTestDB(t)
		userID := insertTestUser(t, db, "learner@example.com")
		moduleID := insertTestModule(t, db, "blockchain-101", "bitcoin-and-fundamentals", "how-it-all-began", "blockchain-101/how-it-all-began")
		postID := insertTestPost(t, db, "blockchain-101/how-it-all-began")
		insertTestQuestion(t, db, "q1", postID)
		insertTestQuestion(t, db, "q2", postID)

		svc := services.NewQuizSessionService(db)
		attemptUUID, total, err := svc.Start(userID, "blockchain-101", "bitcoin-and-fundamentals")
		if err != nil {
			t.Fatal(err)
		}
		if attemptUUID == "" {
			t.Fatal("attemptUUID is empty")
		}
		if total != 2 {
			t.Fatalf("total = %d, want 2 (pool has exactly 2 questions)", total)
		}

		var attemptID, gotModuleID int
		if err := db.QueryRow("SELECT id, module_id FROM quiz_attempts WHERE uuid = ?", attemptUUID).Scan(&attemptID, &gotModuleID); err != nil {
			t.Fatal(err)
		}
		if gotModuleID != moduleID {
			t.Errorf("module_id = %d, want %d", gotModuleID, moduleID)
		}

		var positionCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM quiz_attempt_questions WHERE attempt_id = ?", attemptID).Scan(&positionCount); err != nil {
			t.Fatal(err)
		}
		if positionCount != 2 {
			t.Fatalf("quiz_attempt_questions row count = %d, want 2", positionCount)
		}
	})

	t.Run("caps the pick-order at questionsPerAttempt even with a larger pool", func(t *testing.T) {
		db := setupTestDB(t)
		userID := insertTestUser(t, db, "learner@example.com")
		insertTestModule(t, db, "blockchain-101", "bitcoin-and-fundamentals", "how-it-all-began", "blockchain-101/how-it-all-began")
		postID := insertTestPost(t, db, "blockchain-101/how-it-all-began")
		for i := 0; i < 12; i++ {
			insertTestQuestion(t, db, "q"+string(rune('a'+i)), postID)
		}

		svc := services.NewQuizSessionService(db)
		_, total, err := svc.Start(userID, "blockchain-101", "bitcoin-and-fundamentals")
		if err != nil {
			t.Fatal(err)
		}
		if total != 8 {
			t.Fatalf("total = %d, want 8 (questionsPerAttempt cap)", total)
		}
	})

	t.Run("returns ErrModuleNotFound for an unknown module", func(t *testing.T) {
		db := setupTestDB(t)
		userID := insertTestUser(t, db, "learner@example.com")

		svc := services.NewQuizSessionService(db)
		_, _, err := svc.Start(userID, "nonexistent-list", "nonexistent-module")
		if err != services.ErrModuleNotFound {
			t.Fatalf("err = %v, want ErrModuleNotFound", err)
		}
	})

	t.Run("pool query dedupes a question linked to two posts within the same module", func(t *testing.T) {
		db := setupTestDB(t)
		userID := insertTestUser(t, db, "learner@example.com")
		moduleID := insertTestModule(t, db, "blockchain-101", "bitcoin-and-fundamentals", "how-it-all-began", "blockchain-101/how-it-all-began")
		postA := insertTestPost(t, db, "blockchain-101/how-it-all-began")

		// A second item in the SAME module, pointing at a different post.
		postB := insertTestPost(t, db, "blockchain-101/transactions")
		if _, err := db.Exec(
			"INSERT INTO reading_list_items (module_id, slug, post_path, position) VALUES (?, 'transactions', 'blockchain-101/transactions', 2)",
			moduleID,
		); err != nil {
			t.Fatal(err)
		}

		// One question linked to BOTH posts - without DISTINCT this would
		// appear twice in the pool (and could theoretically be picked twice
		// into the same attempt).
		questionID := insertTestQuestion(t, db, "shared-question", postA)
		if _, err := db.Exec("INSERT INTO question_posts (question_id, post_id) VALUES (?, ?)", questionID, postB); err != nil {
			t.Fatal(err)
		}

		svc := services.NewQuizSessionService(db)
		_, total, err := svc.Start(userID, "blockchain-101", "bitcoin-and-fundamentals")
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 {
			t.Fatalf("total = %d, want 1 (DISTINCT must dedupe a question linked to two posts in the same module)", total)
		}
	})

	t.Run("resumes an in-progress attempt instead of creating a new one", func(t *testing.T) {
		db := setupTestDB(t)
		userID := insertTestUser(t, db, "learner@example.com")
		insertTestModule(t, db, "blockchain-101", "bitcoin-and-fundamentals", "how-it-all-began", "blockchain-101/how-it-all-began")
		postID := insertTestPost(t, db, "blockchain-101/how-it-all-began")
		insertTestQuestion(t, db, "q1", postID)
		insertTestQuestion(t, db, "q2", postID)

		svc := services.NewQuizSessionService(db)
		firstUUID, firstTotal, err := svc.Start(userID, "blockchain-101", "bitcoin-and-fundamentals")
		if err != nil {
			t.Fatal(err)
		}
		secondUUID, secondTotal, err := svc.Start(userID, "blockchain-101", "bitcoin-and-fundamentals")
		if err != nil {
			t.Fatal(err)
		}
		if secondUUID != firstUUID {
			t.Fatalf("second Start returned uuid %q, want the in-progress attempt %q", secondUUID, firstUUID)
		}
		if secondTotal != firstTotal {
			t.Fatalf("second Start total = %d, want %d", secondTotal, firstTotal)
		}

		var attemptCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM quiz_attempts WHERE user_id = ?", userID).Scan(&attemptCount); err != nil {
			t.Fatal(err)
		}
		if attemptCount != 1 {
			t.Fatalf("attempt row count = %d, want 1 (Start must be get-or-create)", attemptCount)
		}
	})

	t.Run("resume is per-user - another user's in-progress attempt is not shared", func(t *testing.T) {
		db := setupTestDB(t)
		userA := insertTestUser(t, db, "learner@example.com")
		userB := insertTestUser(t, db, "other@example.com")
		insertTestModule(t, db, "blockchain-101", "bitcoin-and-fundamentals", "how-it-all-began", "blockchain-101/how-it-all-began")
		postID := insertTestPost(t, db, "blockchain-101/how-it-all-began")
		insertTestQuestion(t, db, "q1", postID)

		svc := services.NewQuizSessionService(db)
		uuidA, _, err := svc.Start(userA, "blockchain-101", "bitcoin-and-fundamentals")
		if err != nil {
			t.Fatal(err)
		}
		uuidB, _, err := svc.Start(userB, "blockchain-101", "bitcoin-and-fundamentals")
		if err != nil {
			t.Fatal(err)
		}
		if uuidA == uuidB {
			t.Fatal("users must never share an attempt")
		}
	})

	t.Run("a completed attempt is not resumed - a fresh one is created", func(t *testing.T) {
		db := setupTestDB(t)
		userID := insertTestUser(t, db, "learner@example.com")
		insertTestModule(t, db, "blockchain-101", "bitcoin-and-fundamentals", "how-it-all-began", "blockchain-101/how-it-all-began")
		postID := insertTestPost(t, db, "blockchain-101/how-it-all-began")
		insertTestQuestion(t, db, "q1", postID)

		svc := services.NewQuizSessionService(db)
		firstUUID, _, err := svc.Start(userID, "blockchain-101", "bitcoin-and-fundamentals")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(
			"UPDATE quiz_attempts SET completed_at = CURRENT_TIMESTAMP WHERE uuid = ?",
			firstUUID,
		); err != nil {
			t.Fatal(err)
		}

		secondUUID, _, err := svc.Start(userID, "blockchain-101", "bitcoin-and-fundamentals")
		if err != nil {
			t.Fatal(err)
		}
		if secondUUID == firstUUID {
			t.Fatal("Start resumed a completed attempt - it must create a fresh one")
		}
	})
}

func TestQuizSessionServiceNext(t *testing.T) {
	setup := func(t *testing.T) (db *sql.DB, svc *services.QuizSessionService, userID int, attemptUUID string) {
		t.Helper()
		db = setupTestDB(t)
		userID = insertTestUser(t, db, "learner@example.com")
		insertTestModule(t, db, "blockchain-101", "bitcoin-and-fundamentals", "how-it-all-began", "blockchain-101/how-it-all-began")
		postID := insertTestPost(t, db, "blockchain-101/how-it-all-began")
		q1 := insertTestQuestion(t, db, "q1", postID)
		insertTestQuestionTranslation(t, db, q1, "en", "What is q1?", `{"options":[]}`)
		q2 := insertTestQuestion(t, db, "q2", postID)
		insertTestQuestionTranslation(t, db, q2, "en", "What is q2?", `{"options":[]}`)

		svc = services.NewQuizSessionService(db)
		attemptUUID, _, err := svc.Start(userID, "blockchain-101", "bitcoin-and-fundamentals")
		if err != nil {
			t.Fatal(err)
		}
		return db, svc, userID, attemptUUID
	}

	t.Run("returns the question at position 0 before any answer", func(t *testing.T) {
		_, svc, userID, attemptUUID := setup(t)

		q, position, total, done, err := svc.Next(userID, attemptUUID, "en")
		if err != nil {
			t.Fatal(err)
		}
		if done || q == nil {
			t.Fatalf("done = %v, q = %v, want a question and done=false", done, q)
		}
		if position != 0 || total != 2 {
			t.Fatalf("position=%d total=%d, want 0 and 2", position, total)
		}
	})

	t.Run("repeated calls without an intervening answer return the identical question", func(t *testing.T) {
		_, svc, userID, attemptUUID := setup(t)

		first, _, _, _, err := svc.Next(userID, attemptUUID, "en")
		if err != nil {
			t.Fatal(err)
		}
		second, _, _, _, err := svc.Next(userID, attemptUUID, "en")
		if err != nil {
			t.Fatal(err)
		}
		if first.UUID != second.UUID {
			t.Fatalf("first.UUID = %q, second.UUID = %q, want identical", first.UUID, second.UUID)
		}
	})

	t.Run("advances to the next position once an answer row exists", func(t *testing.T) {
		db, svc, userID, attemptUUID := setup(t)
		var attemptID int
		if err := db.QueryRow("SELECT id FROM quiz_attempts WHERE uuid = ?", attemptUUID).Scan(&attemptID); err != nil {
			t.Fatal(err)
		}
		var firstQuestionID int
		if err := db.QueryRow("SELECT question_id FROM quiz_attempt_questions WHERE attempt_id = ? AND position = 0", attemptID).Scan(&firstQuestionID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(
			"INSERT INTO quiz_attempt_answers (attempt_id, question_id, response, correct) VALUES (?, ?, '{}', 1)",
			attemptID, firstQuestionID,
		); err != nil {
			t.Fatal(err)
		}

		_, position, _, done, err := svc.Next(userID, attemptUUID, "en")
		if err != nil {
			t.Fatal(err)
		}
		if done || position != 1 {
			t.Fatalf("position=%d done=%v, want 1 and false", position, done)
		}
	})

	t.Run("returns done=true once every position is answered", func(t *testing.T) {
		db, svc, userID, attemptUUID := setup(t)
		var attemptID int
		db.QueryRow("SELECT id FROM quiz_attempts WHERE uuid = ?", attemptUUID).Scan(&attemptID)
		rows, _ := db.Query("SELECT question_id FROM quiz_attempt_questions WHERE attempt_id = ?", attemptID)
		defer rows.Close()
		var qids []int
		for rows.Next() {
			var id int
			rows.Scan(&id)
			qids = append(qids, id)
		}
		for _, qid := range qids {
			if _, err := db.Exec(
				"INSERT INTO quiz_attempt_answers (attempt_id, question_id, response, correct) VALUES (?, ?, '{}', 1)",
				attemptID, qid,
			); err != nil {
				t.Fatal(err)
			}
		}

		q, _, _, done, err := svc.Next(userID, attemptUUID, "en")
		if err != nil {
			t.Fatal(err)
		}
		if !done || q != nil {
			t.Fatalf("done=%v q=%v, want done=true and q=nil", done, q)
		}
	})

	t.Run("rejects an attempt owned by a different user", func(t *testing.T) {
		db, svc, _, attemptUUID := setup(t)
		otherUser := insertTestUser(t, db, "other@example.com")

		_, _, _, _, err := svc.Next(otherUser, attemptUUID, "en")
		if err != services.ErrAttemptForbidden {
			t.Fatalf("err = %v, want ErrAttemptForbidden", err)
		}
	})

	t.Run("returns ErrAttemptNotFound for an unknown uuid", func(t *testing.T) {
		_, svc, userID, _ := setup(t)

		_, _, _, _, err := svc.Next(userID, "nonexistent-uuid", "en")
		if err != services.ErrAttemptNotFound {
			t.Fatalf("err = %v, want ErrAttemptNotFound", err)
		}
	})

	t.Run("returns ErrAttemptCompleted once completed_at is set", func(t *testing.T) {
		db, svc, userID, attemptUUID := setup(t)
		if _, err := db.Exec("UPDATE quiz_attempts SET completed_at = CURRENT_TIMESTAMP WHERE uuid = ?", attemptUUID); err != nil {
			t.Fatal(err)
		}

		_, _, _, _, err := svc.Next(userID, attemptUUID, "en")
		if err != services.ErrAttemptCompleted {
			t.Fatalf("err = %v, want ErrAttemptCompleted", err)
		}
	})

	t.Run("strips the explanation key from content, leaving other fields intact", func(t *testing.T) {
		db := setupTestDB(t)
		userID := insertTestUser(t, db, "learner@example.com")
		insertTestModule(t, db, "blockchain-101", "bitcoin-and-fundamentals", "how-it-all-began", "blockchain-101/how-it-all-began")
		postID := insertTestPost(t, db, "blockchain-101/how-it-all-began")
		q1 := insertTestQuestion(t, db, "q1", postID)
		insertTestQuestionTranslation(t, db, q1, "en", "What is q1?",
			`{"options":["a","b"],"explanation":"some explanation text"}`)

		svc := services.NewQuizSessionService(db)
		attemptUUID, _, err := svc.Start(userID, "blockchain-101", "bitcoin-and-fundamentals")
		if err != nil {
			t.Fatal(err)
		}

		q, _, _, done, err := svc.Next(userID, attemptUUID, "en")
		if err != nil {
			t.Fatal(err)
		}
		if done || q == nil {
			t.Fatalf("done = %v, q = %v, want a question and done=false", done, q)
		}

		var parsed map[string]json.RawMessage
		if err := json.Unmarshal(q.Content, &parsed); err != nil {
			t.Fatalf("Content did not unmarshal as an object: %v", err)
		}
		if _, ok := parsed["explanation"]; ok {
			t.Fatalf("Content = %s, want no \"explanation\" key before the question is answered", q.Content)
		}
		var opts []string
		if err := json.Unmarshal(parsed["options"], &opts); err != nil {
			t.Fatal(err)
		}
		if len(opts) != 2 || opts[0] != "a" || opts[1] != "b" {
			t.Fatalf("options = %v, want [a b] (unrelated content fields must survive stripping)", opts)
		}
	})
}
