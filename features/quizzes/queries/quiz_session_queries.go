package queries

import (
	"arkana/features/quizzes/models"
	dbpkg "arkana/shared/db"
	"database/sql"
)

type QuizSessionQueries interface {
	ResolveModuleID(listSlug, moduleSlug string) (int, error)
	FindActiveAttempt(userID, moduleID int) (attemptID int, attemptUUID string, err error)
	QuestionPool(moduleID int) ([]models.Question, error)
	LanguagesWithFullCoverage(questionIDs []int) ([]string, error)
	InsertAttempt(uuid string, moduleID, userID int) (int64, error)
	InsertAttemptQuestion(attemptID int64, questionID, position int) error
	GetAttemptByUUID(attemptUUID string) (attemptID, ownerID int, completedAt sql.NullTime, err error)
	TotalQuestions(attemptID int) (int, error)
	AnsweredCount(attemptID int) (int, error)
	QuestionIDAtPosition(attemptID, position int) (int, error)
	LoadQuestionDelivery(questionID int, lang string) (uuid, qType, prompt, content string, difficulty int, err error)
	QuestionAtPosition(attemptID, position int) (questionID int, questionUUID, qType, answerKey string, err error)
	InsertAnswer(attemptID, questionID int, response string, correct, skipped bool) error
	ReinforcementContent(questionID int, lang string) (string, error)
	ReinforcementPostPaths(questionID int) ([]string, error)
	CountCorrectAnswers(attemptID int) (int, error)
	MarkAttemptCompleted(attemptID int, score int, passed bool) error
	ReviewPostPaths(attemptID int) ([]string, error)
	WithTx(tx *sql.Tx) QuizSessionQueries
}

type SQLQuizSessionQueries struct {
	db dbpkg.DBTX
}

func NewSQLQuizSessionQueries(db dbpkg.DBTX) *SQLQuizSessionQueries {
	return &SQLQuizSessionQueries{db: db}
}

func (q *SQLQuizSessionQueries) WithTx(tx *sql.Tx) QuizSessionQueries {
	return NewSQLQuizSessionQueries(tx)
}

// ResolveModuleID looks up a reading_list_modules.id from its public
// listSlug/moduleSlug pair. Returns sql.ErrNoRows (unmodified) if not found.
func (q *SQLQuizSessionQueries) ResolveModuleID(listSlug, moduleSlug string) (int, error) {
	var moduleID int
	err := q.db.QueryRow(`
		SELECT rlm.id FROM reading_list_modules rlm
		JOIN reading_lists rl ON rl.id = rlm.reading_list_id
		WHERE rl.slug = ? AND rlm.slug = ?
	`, listSlug, moduleSlug).Scan(&moduleID)
	return moduleID, err
}

// FindActiveAttempt returns the most recent in-progress attempt for a
// user/module pair, if any. Returns sql.ErrNoRows (unmodified) if there
// is none.
func (q *SQLQuizSessionQueries) FindActiveAttempt(userID, moduleID int) (attemptID int, attemptUUID string, err error) {
	err = q.db.QueryRow(`
		SELECT id, uuid FROM quiz_attempts
		WHERE user_id = ? AND module_id = ? AND completed_at IS NULL
		ORDER BY id DESC LIMIT 1
	`, userID, moduleID).Scan(&attemptID, &attemptUUID)
	return
}

// QuestionPool assembles every question linked (via question_posts) to a
// post referenced by any item in this module - the "quiz" for a module is
// this query, not a stored entity.
func (q *SQLQuizSessionQueries) QuestionPool(moduleID int) ([]models.Question, error) {
	rows, err := q.db.Query(`
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

	var pool []models.Question
	for rows.Next() {
		var item models.Question
		var answerKey string
		if err := rows.Scan(&item.ID, &item.UUID, &item.Type, &item.Difficulty, &answerKey); err != nil {
			return nil, err
		}
		item.AnswerKey = []byte(answerKey)
		pool = append(pool, item)
	}
	return pool, rows.Err()
}

// LanguagesWithFullCoverage returns every lang that has a
// question_translations row for all of questionIDs.
func (q *SQLQuizSessionQueries) LanguagesWithFullCoverage(questionIDs []int) ([]string, error) {
	query := `
		SELECT lang FROM question_translations
		WHERE question_id IN (` + dbpkg.Placeholders(len(questionIDs)) + `)
		GROUP BY lang
		HAVING COUNT(DISTINCT question_id) = ?
	`
	rows, err := q.db.Query(query, append(dbpkg.ToAny(questionIDs), len(questionIDs))...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	languages := []string{}
	for rows.Next() {
		var lang string
		if err := rows.Scan(&lang); err != nil {
			return nil, err
		}
		languages = append(languages, lang)
	}
	return languages, rows.Err()
}

// InsertAttempt creates a new quiz_attempts row and returns its id.
func (q *SQLQuizSessionQueries) InsertAttempt(uuid string, moduleID, userID int) (int64, error) {
	result, err := q.db.Exec(`INSERT INTO quiz_attempts (uuid, module_id, user_id) VALUES (?, ?, ?)`, uuid, moduleID, userID)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// InsertAttemptQuestion persists one position in an attempt's pick-order.
func (q *SQLQuizSessionQueries) InsertAttemptQuestion(attemptID int64, questionID, position int) error {
	_, err := q.db.Exec(
		`INSERT INTO quiz_attempt_questions (attempt_id, question_id, position) VALUES (?, ?, ?)`,
		attemptID, questionID, position,
	)
	return err
}

// GetAttemptByUUID loads a quiz_attempts row by its public uuid. Returns
// sql.ErrNoRows (unmodified) if it doesn't exist.
func (q *SQLQuizSessionQueries) GetAttemptByUUID(attemptUUID string) (attemptID, ownerID int, completedAt sql.NullTime, err error) {
	err = q.db.QueryRow(
		"SELECT id, user_id, completed_at FROM quiz_attempts WHERE uuid = ?",
		attemptUUID,
	).Scan(&attemptID, &ownerID, &completedAt)
	return
}

// TotalQuestions returns how many questions an attempt has in its pick-order.
func (q *SQLQuizSessionQueries) TotalQuestions(attemptID int) (int, error) {
	var total int
	err := q.db.QueryRow("SELECT COUNT(*) FROM quiz_attempt_questions WHERE attempt_id = ?", attemptID).Scan(&total)
	return total, err
}

// AnsweredCount returns how many questions an attempt has answered or skipped.
func (q *SQLQuizSessionQueries) AnsweredCount(attemptID int) (int, error) {
	var count int
	err := q.db.QueryRow("SELECT COUNT(*) FROM quiz_attempt_answers WHERE attempt_id = ?", attemptID).Scan(&count)
	return count, err
}

// QuestionIDAtPosition returns the question_id at a given position in an
// attempt's pick-order.
func (q *SQLQuizSessionQueries) QuestionIDAtPosition(attemptID, position int) (int, error) {
	var questionID int
	err := q.db.QueryRow(
		"SELECT question_id FROM quiz_attempt_questions WHERE attempt_id = ? AND position = ?",
		attemptID, position,
	).Scan(&questionID)
	return questionID, err
}

// LoadQuestionDelivery returns the raw scanned fields needed to build a
// correctness-stripped question delivery payload; the caller strips the
// "explanation" key out of content itself (that's a pure JSON transform,
// not a query concern).
func (q *SQLQuizSessionQueries) LoadQuestionDelivery(questionID int, lang string) (uuid, qType, prompt, content string, difficulty int, err error) {
	err = q.db.QueryRow(`
		SELECT q.uuid, q.type, q.difficulty, qt.prompt, qt.content
		FROM questions q
		JOIN question_translations qt ON qt.question_id = q.id
		WHERE q.id = ? AND qt.lang = ?
	`, questionID, lang).Scan(&uuid, &qType, &difficulty, &prompt, &content)
	return
}

// QuestionAtPosition returns the question at an attempt's given position,
// for answer validation.
func (q *SQLQuizSessionQueries) QuestionAtPosition(attemptID, position int) (questionID int, questionUUID, qType, answerKey string, err error) {
	err = q.db.QueryRow(`
		SELECT q.id, q.uuid, q.type, q.answer_key
		FROM quiz_attempt_questions qaq
		JOIN questions q ON q.id = qaq.question_id
		WHERE qaq.attempt_id = ? AND qaq.position = ?
	`, attemptID, position).Scan(&questionID, &questionUUID, &qType, &answerKey)
	return
}

// InsertAnswer records a graded (or skipped) answer.
func (q *SQLQuizSessionQueries) InsertAnswer(attemptID, questionID int, response string, correct, skipped bool) error {
	_, err := q.db.Exec(
		`INSERT INTO quiz_attempt_answers (attempt_id, question_id, response, correct, skipped) VALUES (?, ?, ?, ?, ?)`,
		attemptID, questionID, response, correct, skipped,
	)
	return err
}

// ReinforcementContent returns a question's translated content blob, for
// pulling out its optional explanation.
func (q *SQLQuizSessionQueries) ReinforcementContent(questionID int, lang string) (string, error) {
	var content string
	err := q.db.QueryRow(
		"SELECT content FROM question_translations WHERE question_id = ? AND lang = ?",
		questionID, lang,
	).Scan(&content)
	return content, err
}

// ReinforcementPostPaths returns every post linked to a question via
// question_posts.
func (q *SQLQuizSessionQueries) ReinforcementPostPaths(questionID int) ([]string, error) {
	rows, err := q.db.Query(`
		SELECT p.path_identifier
		FROM question_posts qp
		JOIN posts p ON p.id = qp.post_id
		WHERE qp.question_id = ?
	`, questionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var postPaths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		postPaths = append(postPaths, path)
	}
	return postPaths, rows.Err()
}

// CountCorrectAnswers returns how many of an attempt's answers were correct.
func (q *SQLQuizSessionQueries) CountCorrectAnswers(attemptID int) (int, error) {
	var correctCount int
	err := q.db.QueryRow(
		"SELECT COUNT(*) FROM quiz_attempt_answers WHERE attempt_id = ? AND correct = 1",
		attemptID,
	).Scan(&correctCount)
	return correctCount, err
}

// MarkAttemptCompleted finalizes an attempt's score.
func (q *SQLQuizSessionQueries) MarkAttemptCompleted(attemptID int, score int, passed bool) error {
	_, err := q.db.Exec(
		"UPDATE quiz_attempts SET completed_at = CURRENT_TIMESTAMP, score = ?, passed = ? WHERE id = ?",
		score, passed, attemptID,
	)
	return err
}

// ReviewPostPaths aggregates the reinforcement posts of every missed
// (wrong or skipped) answer in an attempt, deduped, ordered by when the
// miss happened.
func (q *SQLQuizSessionQueries) ReviewPostPaths(attemptID int) ([]string, error) {
	rows, err := q.db.Query(`
		SELECT p.path_identifier
		FROM quiz_attempt_answers qaa
		JOIN question_posts qp ON qp.question_id = qaa.question_id
		JOIN posts p ON p.id = qp.post_id
		WHERE qaa.attempt_id = ? AND qaa.correct = 0
		GROUP BY p.path_identifier
		ORDER BY MIN(qaa.id)
	`, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	paths := []string{}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}
