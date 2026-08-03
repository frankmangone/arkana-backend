package queries

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

// attemptTTL is the sliding TTL for all attempt-related Redis keys - reset
// on every read or write, including read-only calls (see the load/save
// helpers below). Two hours is generous for a distracted/interrupted
// session while still cleaning up same-day.
const attemptTTL = 2 * time.Hour

func attemptKey(uuid string) string {
	return "quiz:attempt:" + uuid
}

func activeAttemptKey(userID, moduleID int) string {
	return fmt.Sprintf("quiz:active-attempt:%d:%d", userID, moduleID)
}

// redisAttempt is the whole-attempt JSON blob stored at attemptKey(uuid).
// encoding/json handles the int-keyed Answers map natively (stringifying
// keys on marshal, parsing back to int on unmarshal).
type redisAttempt struct {
	UUID        string              `json:"uuid"`
	ModuleID    int                 `json:"module_id"`
	UserID      int                 `json:"user_id"`
	Tier        string              `json:"tier"`
	StartedAt   time.Time           `json:"started_at"`
	CompletedAt *time.Time          `json:"completed_at,omitempty"`
	Score       *int                `json:"score,omitempty"`
	Passed      *bool               `json:"passed,omitempty"`
	Questions   []int               `json:"questions"` // position -> question_id, set once at CreateAttempt
	Answers     map[int]redisAnswer `json:"answers"`   // question_id -> answer
}

// redisAnswer records one graded (or skipped) answer. AnsweredAt gives
// ReviewPostPaths (Task 5) a chronological order to walk missed answers
// in, since a Go map has none - the same role quiz_attempt_answers.id's
// insertion order played in SQL.
type redisAnswer struct {
	Response   string    `json:"response"`
	Correct    bool      `json:"correct"`
	Skipped    bool      `json:"skipped"`
	AnsweredAt time.Time `json:"answered_at"`
}

// load fetches and decodes an attempt blob, sliding its TTL forward - read
// paths advance the TTL exactly like write paths do, per this feature's
// TTL policy.
func (q *RedisQuizSessionQueries) load(attemptUUID string) (*redisAttempt, error) {
	ctx := context.Background()
	data, err := q.redisClient.Get(ctx, attemptKey(attemptUUID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var attempt redisAttempt
	if err := json.Unmarshal(data, &attempt); err != nil {
		return nil, err
	}
	if err := q.redisClient.Expire(ctx, attemptKey(attemptUUID), attemptTTL).Err(); err != nil {
		return nil, err
	}
	return &attempt, nil
}

// save re-serializes and writes an attempt blob back, refreshing its TTL
// (SET with an expiry both writes the value and resets the TTL in one
// command).
func (q *RedisQuizSessionQueries) save(attempt *redisAttempt) error {
	data, err := json.Marshal(attempt)
	if err != nil {
		return err
	}
	return q.redisClient.Set(context.Background(), attemptKey(attempt.UUID), data, attemptTTL).Err()
}

// CreateAttempt writes a brand-new attempt's blob and its resume index in
// one Redis pipeline (MULTI/EXEC), so there's no window where one key
// exists without the other.
func (q *RedisQuizSessionQueries) CreateAttempt(uuid string, moduleID, userID int, questionOrder []int) error {
	attempt := &redisAttempt{
		UUID:      uuid,
		ModuleID:  moduleID,
		UserID:    userID,
		Tier:      "standard",
		StartedAt: time.Now(),
		Questions: questionOrder,
		Answers:   map[int]redisAnswer{},
	}
	data, err := json.Marshal(attempt)
	if err != nil {
		return err
	}

	ctx := context.Background()
	pipe := q.redisClient.TxPipeline()
	pipe.Set(ctx, attemptKey(uuid), data, attemptTTL)
	pipe.Set(ctx, activeAttemptKey(userID, moduleID), uuid, attemptTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// FindActiveAttemptUUID looks up the resume index for a user/module pair.
// It defensively confirms the attempt blob itself still exists before
// trusting the index - guards the rare case where the index outlives the
// blob by a beat; if the blob is gone, this is treated as "no active
// attempt" so Start begins a fresh one instead of resuming a ghost.
func (q *RedisQuizSessionQueries) FindActiveAttemptUUID(userID, moduleID int) (string, error) {
	ctx := context.Background()
	uuid, err := q.redisClient.Get(ctx, activeAttemptKey(userID, moduleID)).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}

	exists, err := q.redisClient.Exists(ctx, attemptKey(uuid)).Result()
	if err != nil {
		return "", err
	}
	if exists == 0 {
		return "", ErrNotFound
	}
	return uuid, nil
}

// GetAttemptMeta returns the two fields getOwnedAttempt needs: who owns
// this attempt, and whether it's already completed.
func (q *RedisQuizSessionQueries) GetAttemptMeta(attemptUUID string) (ownerID int, completedAt *time.Time, err error) {
	attempt, err := q.load(attemptUUID)
	if err != nil {
		return 0, nil, err
	}
	return attempt.UserID, attempt.CompletedAt, nil
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
