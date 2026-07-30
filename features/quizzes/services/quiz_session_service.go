package services

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

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
var ErrWrongQuestion = errors.New("questionId does not match the attempt's current question")
var ErrAttemptIncomplete = errors.New("not every question has been answered or skipped")

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
	moduleID, err := s.resolveModuleID(listSlug, moduleSlug)
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

// resolveModuleID looks up a reading_list_modules.id from its public
// listSlug/moduleSlug pair, shared by Start and Availability so there's
// exactly one place that translates "not found" into ErrModuleNotFound.
func (s *QuizSessionService) resolveModuleID(listSlug, moduleSlug string) (int, error) {
	var moduleID int
	err := s.db.QueryRow(`
		SELECT rlm.id FROM reading_list_modules rlm
		JOIN reading_lists rl ON rl.id = rlm.reading_list_id
		WHERE rl.slug = ? AND rlm.slug = ?
	`, listSlug, moduleSlug).Scan(&moduleID)
	if err == sql.ErrNoRows {
		return 0, ErrModuleNotFound
	}
	if err != nil {
		return 0, err
	}
	return moduleID, nil
}

// Availability reports whether a module currently has any quiz questions
// at all, and if so, which languages have full translation coverage over
// the *current* pool - every question in questionPool(moduleID) must have
// a question_translations row for a language to count as available in it.
// This mirrors questionPool exactly (same query, same source of truth as
// Start/Next) so this can never drift from what a real attempt would try
// to serve, and avoids the untranslated-question failure loadQuestionDelivery
// would otherwise hit mid-session.
func (s *QuizSessionService) Availability(listSlug, moduleSlug string) (available bool, languages []string, err error) {
	moduleID, err := s.resolveModuleID(listSlug, moduleSlug)
	if err != nil {
		return false, nil, err
	}

	pool, err := s.questionPool(moduleID)
	if err != nil {
		return false, nil, err
	}
	if len(pool) == 0 {
		return false, []string{}, nil
	}

	ids := make([]any, len(pool))
	placeholders := make([]string, len(pool))
	for i, q := range pool {
		ids[i] = q.ID
		placeholders[i] = "?"
	}

	query := `
		SELECT lang FROM question_translations
		WHERE question_id IN (` + strings.Join(placeholders, ",") + `)
		GROUP BY lang
		HAVING COUNT(DISTINCT question_id) = ?
	`
	rows, err := s.db.Query(query, append(ids, len(pool))...)
	if err != nil {
		return false, nil, err
	}
	defer rows.Close()

	languages = []string{}
	for rows.Next() {
		var lang string
		if err := rows.Scan(&lang); err != nil {
			return false, nil, err
		}
		languages = append(languages, lang)
	}
	return true, languages, rows.Err()
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
	stripped, err := stripExplanation([]byte(content))
	if err != nil {
		return nil, err
	}
	q.Content = stripped
	return &q, nil
}

// stripExplanation removes the top-level "explanation" key (if present)
// from a question_translations.content JSON blob before it's served over
// the correctness-stripped Next() read path - that key is reveal-only
// content, meant to appear solely in the answers response once a
// question is wrong or skipped (see reinforcement(), which reads this
// same column independently and is unaffected by this stripping).
func stripExplanation(content []byte) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(content, &fields); err != nil {
		return nil, err
	}
	if _, ok := fields["explanation"]; !ok {
		return json.RawMessage(content), nil
	}
	delete(fields, "explanation")
	stripped, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(stripped), nil
}

type AnswerResult struct {
	Correct       bool
	Skipped       bool
	CorrectReveal json.RawMessage
	Explanation   *string
	PostPaths     []string
	AttemptDone   bool
}

// Answer grades questionUUID's response (or records a skip) against the
// question at the attempt's current position, advancing it. Rejects if
// questionUUID isn't the question actually at that position - this, plus
// UNIQUE(attempt_id, question_id) on quiz_attempt_answers, is what stops
// answering out of order or twice.
func (s *QuizSessionService) Answer(userID int, attemptUUID, questionUUID string, response json.RawMessage, skipped bool, lang string) (*AnswerResult, error) {
	attempt, err := s.getOwnedAttempt(userID, attemptUUID)
	if err != nil {
		return nil, err
	}
	if attempt.CompletedAt.Valid {
		return nil, ErrAttemptCompleted
	}

	answered, err := s.answeredCount(attempt.ID)
	if err != nil {
		return nil, err
	}
	total, err := s.totalQuestions(attempt.ID)
	if err != nil {
		return nil, err
	}
	if answered >= total {
		return nil, ErrAttemptCompleted
	}

	var questionID int
	var questionUUIDAtPosition, qType, answerKey string
	if err := s.db.QueryRow(`
		SELECT q.id, q.uuid, q.type, q.answer_key
		FROM quiz_attempt_questions qaq
		JOIN questions q ON q.id = qaq.question_id
		WHERE qaq.attempt_id = ? AND qaq.position = ?
	`, attempt.ID, answered).Scan(&questionID, &questionUUIDAtPosition, &qType, &answerKey); err != nil {
		return nil, err
	}
	if questionUUIDAtPosition != questionUUID {
		return nil, ErrWrongQuestion
	}

	var correct bool
	var reveal json.RawMessage
	responseToStore := response
	if skipped {
		responseToStore = json.RawMessage("null")
		// reveal depends only on answerKey, so grading against an empty
		// response still produces the right "what was correct" payload;
		// correctness is forced to false regardless, since a skip never
		// counts as correct.
		_, reveal, err = grade(qType, json.RawMessage(answerKey), json.RawMessage(`{}`))
		if err != nil {
			return nil, err
		}
		correct = false
	} else {
		correct, reveal, err = grade(qType, json.RawMessage(answerKey), response)
		if err != nil {
			return nil, err
		}
	}

	if _, err := s.db.Exec(
		`INSERT INTO quiz_attempt_answers (attempt_id, question_id, response, correct, skipped) VALUES (?, ?, ?, ?, ?)`,
		attempt.ID, questionID, string(responseToStore), correct, skipped,
	); err != nil {
		return nil, err
	}

	result := &AnswerResult{Correct: correct, Skipped: skipped, AttemptDone: answered+1 >= total}
	if !correct {
		result.CorrectReveal = reveal
		explanation, postPaths, err := s.reinforcement(questionID, lang)
		if err != nil {
			return nil, err
		}
		result.Explanation = explanation
		result.PostPaths = postPaths
	}
	return result, nil
}

// reinforcement loads the optional explanation string embedded in this
// question's translated content, plus every post linked via
// question_posts (no "primary" post - reinforcement surfaces all of
// them).
func (s *QuizSessionService) reinforcement(questionID int, lang string) (explanation *string, postPaths []string, err error) {
	var content string
	if err := s.db.QueryRow(
		"SELECT content FROM question_translations WHERE question_id = ? AND lang = ?",
		questionID, lang,
	).Scan(&content); err != nil {
		return nil, nil, err
	}
	var parsed struct {
		Explanation *string `json:"explanation"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, nil, err
	}

	rows, err := s.db.Query(`
		SELECT p.path_identifier
		FROM question_posts qp
		JOIN posts p ON p.id = qp.post_id
		WHERE qp.question_id = ?
	`, questionID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, nil, err
		}
		postPaths = append(postPaths, path)
	}
	return parsed.Explanation, postPaths, rows.Err()
}

type CompleteResult struct {
	Score  int
	Passed bool
}

// Complete finalizes an attempt, requiring every question already have a
// row in quiz_attempt_answers (answered or skipped, either counts as
// "resolved"). The client always learns exactly when it's reached this
// point from the last Answer() call's AttemptDone, so a premature
// Complete call is a client bug, not a state this method papers over -
// no auto-skip-the-rest behavior.
func (s *QuizSessionService) Complete(userID int, attemptUUID string) (*CompleteResult, error) {
	attempt, err := s.getOwnedAttempt(userID, attemptUUID)
	if err != nil {
		return nil, err
	}
	if attempt.CompletedAt.Valid {
		return nil, ErrAttemptCompleted
	}

	total, err := s.totalQuestions(attempt.ID)
	if err != nil {
		return nil, err
	}
	answered, err := s.answeredCount(attempt.ID)
	if err != nil {
		return nil, err
	}
	if answered < total {
		return nil, ErrAttemptIncomplete
	}

	var correctCount int
	if err := s.db.QueryRow(
		"SELECT COUNT(*) FROM quiz_attempt_answers WHERE attempt_id = ? AND correct = 1",
		attempt.ID,
	).Scan(&correctCount); err != nil {
		return nil, err
	}

	score := 0
	passed := false
	if total > 0 {
		score = correctCount * 100 / total
		passed = float64(correctCount)/float64(total) >= passThreshold
	}

	if _, err := s.db.Exec(
		"UPDATE quiz_attempts SET completed_at = CURRENT_TIMESTAMP, score = ?, passed = ? WHERE id = ?",
		score, passed, attempt.ID,
	); err != nil {
		return nil, err
	}

	return &CompleteResult{Score: score, Passed: passed}, nil
}
