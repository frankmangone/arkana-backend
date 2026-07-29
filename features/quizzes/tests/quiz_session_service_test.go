package tests

import (
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
}
