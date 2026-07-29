package services

import (
	"encoding/json"
	"math/rand"
)

// Question is the internal domain representation shared by the selector
// and QuizSessionService - distinct from models.QuestionDTO (the public
// wire shape), same "response mirrors payload but stays its own type"
// rationale used throughout this codebase's Payload/Response splits.
type Question struct {
	ID         int
	UUID       string
	Type       string
	Difficulty int
	AnswerKey  json.RawMessage
}

type AnsweredQuestion struct {
	QuestionID int
	Correct    bool
}

// QuestionSelector picks the next question to serve given the full pool
// and the answer history so far. WeightedRandomSelector ignores
// history's Correct field entirely; a future AdaptiveSelector would use
// it to change what it serves next - no interface change needed either
// way, and Start() only ever calls this in a tight loop within a single
// request, so nothing here needs to be deterministic across separate
// calls (persistence into quiz_attempt_questions is what provides
// consistency across the life of an attempt, not the selector itself).
type QuestionSelector interface {
	Next(pool []Question, history []AnsweredQuestion) (question *Question, done bool)
}

type WeightedRandomSelector struct{}

func NewWeightedRandomSelector() *WeightedRandomSelector {
	return &WeightedRandomSelector{}
}

func (s *WeightedRandomSelector) Next(pool []Question, history []AnsweredQuestion) (*Question, bool) {
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
	var remaining []Question
	for _, q := range pool {
		if !answered[q.ID] {
			remaining = append(remaining, q)
		}
	}
	if len(remaining) == 0 {
		return nil, true
	}

	q := remaining[rand.Intn(len(remaining))]
	return &q, false
}
