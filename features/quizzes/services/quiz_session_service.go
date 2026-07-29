package services

import (
	"database/sql"
	"encoding/json"
	"errors"

	"arkana/shared/idgen"
)

// questionsPerAttempt and passThreshold are deliberate implementation
// constants, not config - see docs/superpowers/specs/2026-07-29-quiz-data-model-design.md.
const questionsPerAttempt = 8
const passThreshold = 0.7

var ErrModuleNotFound = errors.New("module not found")

type QuizSessionService struct {
	db *sql.DB
}

func NewQuizSessionService(db *sql.DB) *QuizSessionService {
	return &QuizSessionService{db: db}
}

// CanAttempt is a monetization gating stub (quiz_spec.md requirement
// 4.1) - always true today. The counting logic and its call site already
// exist so wiring in the real free/paid gate later is a one-function
// change, not a redesign.
func (s *QuizSessionService) CanAttempt(userID, moduleID int) (bool, error) {
	return true, nil
}

// Start begins a new attempt for the given module, running the selector
// once and persisting its full pick-order into quiz_attempt_questions -
// this is what makes every later Next() call a trivial, idempotent
// lookup instead of a re-run of selection logic.
func (s *QuizSessionService) Start(userID int, listSlug, moduleSlug string) (attemptUUID string, totalQuestions int, err error) {
	var moduleID int
	err = s.db.QueryRow(`
		SELECT rlm.id FROM reading_list_modules rlm
		JOIN reading_lists rl ON rl.id = rlm.reading_list_id
		WHERE rl.slug = ? AND rlm.slug = ?
	`, listSlug, moduleSlug).Scan(&moduleID)
	if err == sql.ErrNoRows {
		return "", 0, ErrModuleNotFound
	}
	if err != nil {
		return "", 0, err
	}

	if ok, err := s.CanAttempt(userID, moduleID); err != nil {
		return "", 0, err
	} else if !ok {
		return "", 0, errors.New("attempt not allowed")
	}

	pool, err := s.questionPool(moduleID)
	if err != nil {
		return "", 0, err
	}

	selector := NewWeightedRandomSelector()
	var history []AnsweredQuestion
	var chosen []Question
	for {
		q, done := selector.Next(pool, history)
		if done {
			break
		}
		chosen = append(chosen, *q)
		history = append(history, AnsweredQuestion{QuestionID: q.ID})
	}

	uuid, err := idgen.NewV4()
	if err != nil {
		return "", 0, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`INSERT INTO quiz_attempts (uuid, module_id, user_id) VALUES (?, ?, ?)`, uuid, moduleID, userID)
	if err != nil {
		return "", 0, err
	}
	attemptID, err := result.LastInsertId()
	if err != nil {
		return "", 0, err
	}
	for position, q := range chosen {
		if _, err := tx.Exec(
			`INSERT INTO quiz_attempt_questions (attempt_id, question_id, position) VALUES (?, ?, ?)`,
			attemptID, q.ID, position,
		); err != nil {
			return "", 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", 0, err
	}
	return uuid, len(chosen), nil
}

// questionPool assembles every question linked (via question_posts) to a
// post referenced by any item in this module - the "quiz" for a module is
// this query, not a stored entity.
func (s *QuizSessionService) questionPool(moduleID int) ([]Question, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT q.id, q.uuid, q.type, q.difficulty, q.answer_key
		FROM questions q
		JOIN question_posts qp ON qp.question_id = q.id
		JOIN posts p ON p.id = qp.post_id
		JOIN reading_list_items rli ON rli.post_path = p.path_identifier
		WHERE rli.module_id = ?
	`, moduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pool []Question
	for rows.Next() {
		var q Question
		var answerKey string
		if err := rows.Scan(&q.ID, &q.UUID, &q.Type, &q.Difficulty, &answerKey); err != nil {
			return nil, err
		}
		q.AnswerKey = json.RawMessage(answerKey)
		pool = append(pool, q)
	}
	return pool, rows.Err()
}
