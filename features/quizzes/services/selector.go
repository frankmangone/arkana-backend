package services

import (
	"math/rand"

	"arkana/features/quizzes/models"
)

// QuestionSelector picks the next question to serve given the full pool
// and the answer history so far. WeightedRandomSelector ignores
// history's Correct field entirely; a future AdaptiveSelector would use
// it to change what it serves next - no interface change needed either
// way, and Start() only ever calls this in a tight loop within a single
// request, so nothing here needs to be deterministic across separate
// calls (persistence of the pick-order into the attempt blob's Questions
// field is what provides consistency across the life of an attempt, not
// the selector itself).
type QuestionSelector interface {
	Next(pool []models.Question, history []models.AnsweredQuestion) (question *models.Question, done bool)
}

type WeightedRandomSelector struct{}

// NewWeightedRandomSelector constructs a WeightedRandomSelector.
func NewWeightedRandomSelector() *WeightedRandomSelector {
	return &WeightedRandomSelector{}
}

// Next picks a uniformly random question from pool, excluding any question
// already present in history, until questionsPerAttempt questions have been
// chosen (or the pool is exhausted first, whichever limit is lower) - at
// that point done is true and question is nil. Despite the type name,
// selection here is plain random, not weighted by difficulty or any other
// property; "weighted" names where this selector is headed, not what it
// does today (see the QuestionSelector interface doc).
func (s *WeightedRandomSelector) Next(pool []models.Question, history []models.AnsweredQuestion) (*models.Question, bool) {
	limit := questionsPerAttempt
	if limit > len(pool) {
		limit = len(pool)
	}
	if len(history) >= limit {
		return nil, true
	}

	answered := make(map[int]bool, len(history))
	for _, h := range history {
		answered[h.QuestionID] = true
	}
	var remaining []models.Question
	for _, q := range pool {
		if !answered[q.ID] {
			remaining = append(remaining, q)
		}
	}
	if len(remaining) == 0 {
		return nil, true
	}

	q := remaining[rand.Intn(len(remaining))] //nolint:gosec // non-cryptographic use: picking a quiz question, not a secret
	return &q, false
}
