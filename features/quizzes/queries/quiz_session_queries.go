package queries

import (
	"database/sql"
	"errors"
	"time"

	"arkana/features/quizzes/models"
	dbpkg "arkana/shared/db"

	"github.com/redis/go-redis/v9"
)

// ErrNotFound is the not-found sentinel for every attempt-related lookup -
// today's equivalent of sql.ErrNoRows, since attempt state now lives in
// Redis (redis.Nil) rather than SQL. Bank reads (ResolveModuleID, etc.)
// still return sql.ErrNoRows directly, unchanged.
var ErrNotFound = errors.New("not found")

// QuizSessionQueries is implemented by RedisQuizSessionQueries. Bank/
// reading-list reads are pure SQL; attempt/session state reads and writes
// are Redis-backed - see RedisQuizSessionQueries's doc comment.
type QuizSessionQueries interface {
	// Bank / reading-list reads - pure SQL, unchanged from before this
	// feature moved to Redis.
	ResolveModuleID(listSlug, moduleSlug string) (int, error)
	QuestionPool(moduleID int) ([]models.Question, error)
	LanguagesWithFullCoverage(questionIDs []int) ([]string, error)
	LoadQuestionDelivery(questionID int, lang string) (uuid, qType, prompt, content string, difficulty int, err error)
	ReinforcementContent(questionID int, lang string) (string, error)
	ReinforcementPostPaths(questionID int) ([]string, error)

	// Attempt/session state - Redis-backed, keyed by attempt uuid (Redis
	// has no autoincrement id, and the uuid was already the only
	// identifier ever exposed publicly).
	FindActiveAttemptUUID(userID, moduleID int) (string, error)
	CreateAttempt(uuid string, moduleID, userID int, questionOrder []int) error
	GetAttemptMeta(attemptUUID string) (ownerID int, completedAt *time.Time, err error)
	TotalQuestions(attemptUUID string) (int, error)
	AnsweredCount(attemptUUID string) (int, error)
	QuestionIDAtPosition(attemptUUID string, position int) (int, error)
	QuestionAtPosition(attemptUUID string, position int) (questionID int, questionUUID, qType, answerKey string, err error)
	RecordAnswer(attemptUUID string, questionID int, response string, correct, skipped bool) error
	CountCorrectAnswers(attemptUUID string) (int, error)
	MarkAttemptCompleted(attemptUUID string, score int, passed bool) error
	ReviewPostPaths(attemptUUID string) ([]string, error)
}

// RedisQuizSessionQueries is a hybrid: db serves the question bank and
// reading-list joins (unchanged, persistent content), redisClient serves
// attempt/session state (ephemeral, TTL-bound). See
// docs/superpowers/specs/2026-08-03-quiz-attempts-redis-design.md.
type RedisQuizSessionQueries struct {
	db          *sql.DB
	redisClient *redis.Client
}

// NewRedisQuizSessionQueries constructs a RedisQuizSessionQueries. db
// first, matching this codebase's constructor convention.
func NewRedisQuizSessionQueries(db *sql.DB, redisClient *redis.Client) *RedisQuizSessionQueries {
	return &RedisQuizSessionQueries{db: db, redisClient: redisClient}
}

// ResolveModuleID looks up a reading_list_modules.id from its public
// listSlug/moduleSlug pair. Returns sql.ErrNoRows (unmodified) if not found.
func (q *RedisQuizSessionQueries) ResolveModuleID(listSlug, moduleSlug string) (int, error) {
	var moduleID int
	err := q.db.QueryRow(`
		SELECT rlm.id FROM reading_list_modules rlm
		JOIN reading_lists rl ON rl.id = rlm.reading_list_id
		WHERE rl.slug = ? AND rlm.slug = ?
	`, listSlug, moduleSlug).Scan(&moduleID)
	return moduleID, err
}

// QuestionPool assembles every question linked (via question_posts) to a
// post referenced by any item in this module - the "quiz" for a module is
// this query, not a stored entity.
func (q *RedisQuizSessionQueries) QuestionPool(moduleID int) ([]models.Question, error) {
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
func (q *RedisQuizSessionQueries) LanguagesWithFullCoverage(questionIDs []int) ([]string, error) {
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

// LoadQuestionDelivery returns the raw scanned fields needed to build a
// correctness-stripped question delivery payload.
func (q *RedisQuizSessionQueries) LoadQuestionDelivery(questionID int, lang string) (uuid, qType, prompt, content string, difficulty int, err error) {
	err = q.db.QueryRow(`
		SELECT q.uuid, q.type, q.difficulty, qt.prompt, qt.content
		FROM questions q
		JOIN question_translations qt ON qt.question_id = q.id
		WHERE q.id = ? AND qt.lang = ?
	`, questionID, lang).Scan(&uuid, &qType, &difficulty, &prompt, &content)
	return
}

// ReinforcementContent returns a question's translated content blob, for
// pulling out its optional explanation.
func (q *RedisQuizSessionQueries) ReinforcementContent(questionID int, lang string) (string, error) {
	var content string
	err := q.db.QueryRow(
		"SELECT content FROM question_translations WHERE question_id = ? AND lang = ?",
		questionID, lang,
	).Scan(&content)
	return content, err
}

// ReinforcementPostPaths returns every post linked to a question via
// question_posts.
func (q *RedisQuizSessionQueries) ReinforcementPostPaths(questionID int) ([]string, error) {
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
