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
var ErrAttemptNotFound = errors.New("attempt not found")
var ErrAttemptForbidden = errors.New("attempt belongs to another user")
var ErrAttemptCompleted = errors.New("attempt already completed")

// QuestionDelivery is the correctness-stripped shape served over
// GET .../next - deliberately has no answer_key/correct-* field anywhere,
// same rule the DB schema already enforces on questions.answer_key, now
// also enforced explicitly on this read path.
type QuestionDelivery struct {
	UUID       string
	Type       string
	Difficulty int
	Prompt     string
	Content    json.RawMessage
}

type attemptRow struct {
	ID          int
	CompletedAt sql.NullTime
}

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

// getOwnedAttempt loads a quiz_attempts row by its public uuid and
// enforces that it belongs to userID - the actual guard against
// cross-user access; the opaque uuid is defense-in-depth, never a
// substitute for this check. Tasks 6-7 (Answer/Complete) call this same
// helper.
func (s *QuizSessionService) getOwnedAttempt(userID int, attemptUUID string) (*attemptRow, error) {
	var row attemptRow
	var ownerID int
	err := s.db.QueryRow(
		"SELECT id, user_id, completed_at FROM quiz_attempts WHERE uuid = ?",
		attemptUUID,
	).Scan(&row.ID, &ownerID, &row.CompletedAt)
	if err == sql.ErrNoRows {
		return nil, ErrAttemptNotFound
	}
	if err != nil {
		return nil, err
	}
	if ownerID != userID {
		return nil, ErrAttemptForbidden
	}
	return &row, nil
}

func (s *QuizSessionService) totalQuestions(attemptID int) (int, error) {
	var total int
	err := s.db.QueryRow("SELECT COUNT(*) FROM quiz_attempt_questions WHERE attempt_id = ?", attemptID).Scan(&total)
	return total, err
}

func (s *QuizSessionService) answeredCount(attemptID int) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM quiz_attempt_answers WHERE attempt_id = ?", attemptID).Scan(&count)
	return count, err
}

// Next returns the question at the attempt's current position (however
// many answers exist so far), stripped of every correct-answer field.
// Repeated calls without an intervening Answer() call return the
// identical question every time - nothing here advances state, only a
// graded or skipped answer does.
func (s *QuizSessionService) Next(userID int, attemptUUID, lang string) (question *QuestionDelivery, position, total int, done bool, err error) {
	attempt, err := s.getOwnedAttempt(userID, attemptUUID)
	if err != nil {
		return nil, 0, 0, false, err
	}
	if attempt.CompletedAt.Valid {
		return nil, 0, 0, false, ErrAttemptCompleted
	}

	total, err = s.totalQuestions(attempt.ID)
	if err != nil {
		return nil, 0, 0, false, err
	}
	answered, err := s.answeredCount(attempt.ID)
	if err != nil {
		return nil, 0, 0, false, err
	}
	if answered >= total {
		return nil, answered, total, true, nil
	}

	var questionID int
	if err := s.db.QueryRow(
		"SELECT question_id FROM quiz_attempt_questions WHERE attempt_id = ? AND position = ?",
		attempt.ID, answered,
	).Scan(&questionID); err != nil {
		return nil, 0, 0, false, err
	}

	q, err := s.loadQuestionDelivery(questionID, lang)
	if err != nil {
		return nil, 0, 0, false, err
	}
	return q, answered, total, false, nil
}

func (s *QuizSessionService) loadQuestionDelivery(questionID int, lang string) (*QuestionDelivery, error) {
	var q QuestionDelivery
	var content string
	err := s.db.QueryRow(`
		SELECT q.uuid, q.type, q.difficulty, qt.prompt, qt.content
		FROM questions q
		JOIN question_translations qt ON qt.question_id = q.id
		WHERE q.id = ? AND qt.lang = ?
	`, questionID, lang).Scan(&q.UUID, &q.Type, &q.Difficulty, &q.Prompt, &content)
	if err != nil {
		return nil, err
	}
	q.Content = json.RawMessage(content)
	return &q, nil
}
