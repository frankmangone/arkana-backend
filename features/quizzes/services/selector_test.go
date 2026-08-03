package services

import (
	"testing"

	"arkana/features/quizzes/models"
)

func questionPool(ids ...int) []models.Question {
	pool := make([]models.Question, len(ids))
	for i, id := range ids {
		pool[i] = models.Question{ID: id}
	}
	return pool
}

func answered(ids ...int) []models.AnsweredQuestion {
	history := make([]models.AnsweredQuestion, len(ids))
	for i, id := range ids {
		// Alternate Correct so tests also cover that Next ignores it.
		history[i] = models.AnsweredQuestion{QuestionID: id, Correct: i%2 == 0}
	}
	return history
}

func TestWeightedRandomSelectorNext(t *testing.T) {
	t.Run("with no history, returns a question from the pool", func(t *testing.T) {
		s := NewWeightedRandomSelector()
		pool := questionPool(1, 2, 3)

		for i := 0; i < 25; i++ {
			q, done := s.Next(pool, nil)
			if done {
				t.Fatal("done = true, want a question since history is empty")
			}
			if q == nil {
				t.Fatal("expected a non-nil question")
			}
			if q.ID != 1 && q.ID != 2 && q.ID != 3 {
				t.Fatalf("returned question id %d not in pool", q.ID)
			}
		}
	})

	t.Run("never returns a question already present in history, regardless of Correct", func(t *testing.T) {
		s := NewWeightedRandomSelector()
		pool := questionPool(1, 2, 3, 4, 5, 6)
		history := answered(1, 2, 3, 4, 5) // one question (6) left in a 6-question pool

		for i := 0; i < 25; i++ {
			q, done := s.Next(pool, history)
			if done {
				t.Fatal("done = true, want the one remaining question (6)")
			}
			if q.ID != 6 {
				t.Fatalf("q.ID = %d, want 6 (the only unanswered question)", q.ID)
			}
		}
	})

	t.Run("done once history reaches questionsPerAttempt, even with a larger pool", func(t *testing.T) {
		s := NewWeightedRandomSelector()
		pool := questionPool(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
		history := answered(1, 2, 3, 4, 5, 6, 7, 8) // 8 = questionsPerAttempt

		q, done := s.Next(pool, history)
		if !done {
			t.Fatal("done = false, want true once questionsPerAttempt answers exist")
		}
		if q != nil {
			t.Errorf("q = %+v, want nil when done", q)
		}
	})

	t.Run("done once the pool itself is exhausted, even below questionsPerAttempt", func(t *testing.T) {
		s := NewWeightedRandomSelector()
		pool := questionPool(1, 2, 3)
		history := answered(1, 2, 3) // pool has only 3 questions, all answered

		q, done := s.Next(pool, history)
		if !done {
			t.Fatal("done = false, want true once every pool question has been answered")
		}
		if q != nil {
			t.Errorf("q = %+v, want nil when done", q)
		}
	})

	t.Run("done on an empty pool", func(t *testing.T) {
		s := NewWeightedRandomSelector()
		q, done := s.Next(nil, nil)
		if !done || q != nil {
			t.Fatalf("Next(nil, nil) = (%+v, %v), want (nil, true)", q, done)
		}
	})
}
