package queries

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	_ "github.com/mattn/go-sqlite3"
	"github.com/redis/go-redis/v9"
)

// setupTestDB creates the minimal schema subset RedisQuizSessionQueries's
// bank/reading-list reads need - a smaller, package-local counterpart to
// features/quizzes/tests/setup_test.go's full fixture (that one also seeds
// users/tags/etc. for the feature's service- and handler-level tests,
// which this lower-level package has no reason to depend on).
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path_identifier TEXT UNIQUE NOT NULL
		);
		CREATE TABLE reading_lists (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT UNIQUE NOT NULL
		);
		CREATE TABLE reading_list_modules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			reading_list_id INTEGER NOT NULL REFERENCES reading_lists(id),
			slug TEXT NOT NULL,
			position INTEGER NOT NULL,
			UNIQUE (reading_list_id, slug)
		);
		CREATE TABLE reading_list_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			module_id INTEGER NOT NULL REFERENCES reading_list_modules(id),
			slug TEXT NOT NULL,
			post_path TEXT NOT NULL,
			position INTEGER NOT NULL,
			UNIQUE (module_id, slug)
		);
		CREATE TABLE questions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT UNIQUE NOT NULL,
			slug TEXT UNIQUE NOT NULL,
			type TEXT NOT NULL,
			difficulty INTEGER NOT NULL,
			answer_key TEXT NOT NULL
		);
		CREATE TABLE question_translations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			question_id INTEGER NOT NULL REFERENCES questions(id),
			lang TEXT NOT NULL,
			prompt TEXT NOT NULL,
			content TEXT NOT NULL,
			UNIQUE (question_id, lang)
		);
		CREATE TABLE question_posts (
			question_id INTEGER NOT NULL REFERENCES questions(id),
			post_id INTEGER NOT NULL REFERENCES posts(id),
			PRIMARY KEY (question_id, post_id)
		);
	`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// setupTestRedis spins up an in-memory miniredis instance and returns a
// real *redis.Client pointed at it.
func setupTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func seedPost(t *testing.T, db *sql.DB, path string) int {
	t.Helper()
	res, err := db.Exec("INSERT INTO posts (path_identifier) VALUES (?)", path)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return int(id)
}

// seedModule seeds a full reading_lists -> reading_list_modules ->
// reading_list_items chain (one item, pointing at postPath) and returns
// the module's id.
func seedModule(t *testing.T, db *sql.DB, listSlug, moduleSlug, itemSlug, postPath string) int {
	t.Helper()
	res, err := db.Exec("INSERT INTO reading_lists (slug) VALUES (?)", listSlug)
	if err != nil {
		t.Fatal(err)
	}
	listID, _ := res.LastInsertId()

	res, err = db.Exec("INSERT INTO reading_list_modules (reading_list_id, slug, position) VALUES (?, ?, 1)", listID, moduleSlug)
	if err != nil {
		t.Fatal(err)
	}
	moduleID, _ := res.LastInsertId()

	if _, err := db.Exec(
		"INSERT INTO reading_list_items (module_id, slug, post_path, position) VALUES (?, ?, ?, 1)",
		moduleID, itemSlug, postPath,
	); err != nil {
		t.Fatal(err)
	}
	return int(moduleID)
}

// seedQuestion seeds a questions row (type single_choice, answer_key {})
// plus its question_posts link to postID, returning the question's id.
func seedQuestion(t *testing.T, db *sql.DB, slug string, postID int) int {
	t.Helper()
	res, err := db.Exec(
		`INSERT INTO questions (uuid, slug, type, difficulty, answer_key) VALUES (?, ?, 'single_choice', 1, '{}')`,
		slug+"-uuid", slug,
	)
	if err != nil {
		t.Fatal(err)
	}
	questionID, _ := res.LastInsertId()
	if _, err := db.Exec("INSERT INTO question_posts (question_id, post_id) VALUES (?, ?)", questionID, postID); err != nil {
		t.Fatal(err)
	}
	return int(questionID)
}

// TestRedisQuizSessionQueriesBankReads covers the methods that are pure
// SQL passthroughs, unchanged by the move to Redis.
func TestRedisQuizSessionQueriesBankReads(t *testing.T) {
	t.Run("ResolveModuleID resolves a known list/module slug pair", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		moduleID := seedModule(t, db, "blockchain-101", "bitcoin-and-fundamentals", "how-it-all-began", "blockchain-101/how-it-all-began")

		q := NewRedisQuizSessionQueries(db, redisClient)
		gotID, err := q.ResolveModuleID("blockchain-101", "bitcoin-and-fundamentals")
		if err != nil {
			t.Fatal(err)
		}
		if gotID != moduleID {
			t.Errorf("gotID = %d, want %d", gotID, moduleID)
		}
	})

	t.Run("QuestionPool assembles questions linked through the module's posts", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		moduleID := seedModule(t, db, "blockchain-101", "bitcoin-and-fundamentals", "how-it-all-began", "blockchain-101/how-it-all-began")
		postID := seedPost(t, db, "blockchain-101/how-it-all-began")
		seedQuestion(t, db, "q1", postID)

		q := NewRedisQuizSessionQueries(db, redisClient)
		pool, err := q.QuestionPool(moduleID)
		if err != nil {
			t.Fatal(err)
		}
		if len(pool) != 1 {
			t.Fatalf("pool len = %d, want 1", len(pool))
		}
	})
}

func TestRedisQuizSessionQueriesAttemptLifecycle(t *testing.T) {
	t.Run("CreateAttempt persists the blob and the resume index, both with a TTL", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		q := NewRedisQuizSessionQueries(db, redisClient)

		if err := q.CreateAttempt("attempt-1", 42, 7, []int{101, 102}); err != nil {
			t.Fatal(err)
		}

		ownerID, completedAt, err := q.GetAttemptMeta("attempt-1")
		if err != nil {
			t.Fatal(err)
		}
		if ownerID != 7 {
			t.Errorf("ownerID = %d, want 7", ownerID)
		}
		if completedAt != nil {
			t.Errorf("completedAt = %v, want nil (a fresh attempt isn't completed)", completedAt)
		}

		resumedUUID, err := q.FindActiveAttemptUUID(7, 42)
		if err != nil {
			t.Fatal(err)
		}
		if resumedUUID != "attempt-1" {
			t.Errorf("resumedUUID = %q, want %q", resumedUUID, "attempt-1")
		}

		ctx := context.Background()
		ttl, err := redisClient.TTL(ctx, "quiz:attempt:attempt-1").Result()
		if err != nil {
			t.Fatal(err)
		}
		if ttl <= 0 || ttl > 2*time.Hour {
			t.Fatalf("attempt TTL = %v, want a positive duration <= 2h", ttl)
		}
		indexTTL, err := redisClient.TTL(ctx, "quiz:active-attempt:7:42").Result()
		if err != nil {
			t.Fatal(err)
		}
		if indexTTL <= 0 || indexTTL > 2*time.Hour {
			t.Fatalf("index TTL = %v, want a positive duration <= 2h", indexTTL)
		}
	})

	t.Run("FindActiveAttemptUUID returns ErrNotFound when no attempt exists", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		q := NewRedisQuizSessionQueries(db, redisClient)

		_, err := q.FindActiveAttemptUUID(7, 42)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("FindActiveAttemptUUID treats a dangling index (blob already gone) as not found", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		q := NewRedisQuizSessionQueries(db, redisClient)

		if err := q.CreateAttempt("attempt-1", 42, 7, []int{101}); err != nil {
			t.Fatal(err)
		}
		// Simulate the blob expiring slightly before the index key.
		if err := redisClient.Del(context.Background(), "quiz:attempt:attempt-1").Err(); err != nil {
			t.Fatal(err)
		}

		_, err := q.FindActiveAttemptUUID(7, 42)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("GetAttemptMeta returns ErrNotFound for an unknown uuid", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		q := NewRedisQuizSessionQueries(db, redisClient)

		_, _, err := q.GetAttemptMeta("nonexistent")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("GetAttemptMeta (via load) refreshes the paired resume index's TTL, not just the blob's", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		q := NewRedisQuizSessionQueries(db, redisClient)
		ctx := context.Background()

		if err := q.CreateAttempt("attempt-1", 42, 7, []int{101}); err != nil {
			t.Fatal(err)
		}
		// Shrink the index key's TTL well below attemptTTL, simulating time
		// having passed since CreateAttempt.
		shrunk := 5 * time.Second
		if err := redisClient.Expire(ctx, "quiz:active-attempt:7:42", shrunk).Err(); err != nil {
			t.Fatal(err)
		}

		if _, _, err := q.GetAttemptMeta("attempt-1"); err != nil {
			t.Fatal(err)
		}

		indexTTL, err := redisClient.TTL(ctx, "quiz:active-attempt:7:42").Result()
		if err != nil {
			t.Fatal(err)
		}
		if indexTTL <= shrunk {
			t.Fatalf("index TTL = %v, want it restored above the shrunk %v", indexTTL, shrunk)
		}
	})

	t.Run("FindActiveAttemptUUID refreshes both the index's and the blob's TTL", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		q := NewRedisQuizSessionQueries(db, redisClient)
		ctx := context.Background()

		if err := q.CreateAttempt("attempt-1", 42, 7, []int{101}); err != nil {
			t.Fatal(err)
		}
		shrunk := 5 * time.Second
		if err := redisClient.Expire(ctx, "quiz:active-attempt:7:42", shrunk).Err(); err != nil {
			t.Fatal(err)
		}
		if err := redisClient.Expire(ctx, "quiz:attempt:attempt-1", shrunk).Err(); err != nil {
			t.Fatal(err)
		}

		if _, err := q.FindActiveAttemptUUID(7, 42); err != nil {
			t.Fatal(err)
		}

		indexTTL, err := redisClient.TTL(ctx, "quiz:active-attempt:7:42").Result()
		if err != nil {
			t.Fatal(err)
		}
		if indexTTL <= shrunk {
			t.Fatalf("index TTL = %v, want it restored above the shrunk %v", indexTTL, shrunk)
		}
		attemptTTLResult, err := redisClient.TTL(ctx, "quiz:attempt:attempt-1").Result()
		if err != nil {
			t.Fatal(err)
		}
		if attemptTTLResult <= shrunk {
			t.Fatalf("attempt TTL = %v, want it restored above the shrunk %v", attemptTTLResult, shrunk)
		}
	})
}

func TestRedisQuizSessionQueriesProgress(t *testing.T) {
	t.Run("TotalQuestions and AnsweredCount reflect the created attempt", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		q := NewRedisQuizSessionQueries(db, redisClient)

		if err := q.CreateAttempt("attempt-1", 42, 7, []int{101, 102}); err != nil {
			t.Fatal(err)
		}

		total, err := q.TotalQuestions("attempt-1")
		if err != nil {
			t.Fatal(err)
		}
		if total != 2 {
			t.Errorf("total = %d, want 2", total)
		}

		answered, err := q.AnsweredCount("attempt-1")
		if err != nil {
			t.Fatal(err)
		}
		if answered != 0 {
			t.Errorf("answered = %d, want 0 (nothing recorded yet)", answered)
		}
	})

	t.Run("QuestionIDAtPosition returns the question id at a given position", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		q := NewRedisQuizSessionQueries(db, redisClient)
		if err := q.CreateAttempt("attempt-1", 42, 7, []int{101, 102}); err != nil {
			t.Fatal(err)
		}

		got, err := q.QuestionIDAtPosition("attempt-1", 1)
		if err != nil {
			t.Fatal(err)
		}
		if got != 102 {
			t.Errorf("got = %d, want 102", got)
		}
	})

	t.Run("QuestionIDAtPosition returns ErrNotFound for an out-of-range position", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		q := NewRedisQuizSessionQueries(db, redisClient)
		if err := q.CreateAttempt("attempt-1", 42, 7, []int{101}); err != nil {
			t.Fatal(err)
		}

		_, err := q.QuestionIDAtPosition("attempt-1", 5)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("QuestionAtPosition joins the Redis order with the SQL question row", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		postID := seedPost(t, db, "list-1/item-1")
		questionID := seedQuestion(t, db, "q1", postID)
		q := NewRedisQuizSessionQueries(db, redisClient)
		if err := q.CreateAttempt("attempt-1", 42, 7, []int{questionID}); err != nil {
			t.Fatal(err)
		}

		gotID, gotUUID, gotType, gotAnswerKey, err := q.QuestionAtPosition("attempt-1", 0)
		if err != nil {
			t.Fatal(err)
		}
		if gotID != questionID {
			t.Errorf("gotID = %d, want %d", gotID, questionID)
		}
		if gotUUID != "q1-uuid" {
			t.Errorf("gotUUID = %q, want %q", gotUUID, "q1-uuid")
		}
		if gotType != "single_choice" {
			t.Errorf("gotType = %q, want %q", gotType, "single_choice")
		}
		if gotAnswerKey != "{}" {
			t.Errorf("gotAnswerKey = %q, want %q", gotAnswerKey, "{}")
		}
	})

	t.Run("load refreshes the attempt's TTL on every read, not only on write", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		q := NewRedisQuizSessionQueries(db, redisClient)
		if err := q.CreateAttempt("attempt-1", 42, 7, []int{101}); err != nil {
			t.Fatal(err)
		}

		ctx := context.Background()
		key := "quiz:attempt:attempt-1"
		if err := redisClient.Expire(ctx, key, 5*time.Second).Err(); err != nil {
			t.Fatal(err)
		}

		if _, err := q.TotalQuestions("attempt-1"); err != nil {
			t.Fatal(err)
		}

		ttl, err := redisClient.TTL(ctx, key).Result()
		if err != nil {
			t.Fatal(err)
		}
		if ttl <= 5*time.Second {
			t.Fatalf("TTL after a read = %v, want > 5s (a read must slide the TTL back up)", ttl)
		}
	})
}

func TestRedisQuizSessionQueriesAnswerAndComplete(t *testing.T) {
	t.Run("RecordAnswer stores the answer and CountCorrectAnswers counts it", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		q := NewRedisQuizSessionQueries(db, redisClient)
		if err := q.CreateAttempt("attempt-1", 42, 7, []int{101, 102}); err != nil {
			t.Fatal(err)
		}

		if err := q.RecordAnswer("attempt-1", 101, `{"selectedOptionIds":["b"]}`, true, false); err != nil {
			t.Fatal(err)
		}
		if err := q.RecordAnswer("attempt-1", 102, "null", false, true); err != nil {
			t.Fatal(err)
		}

		answered, err := q.AnsweredCount("attempt-1")
		if err != nil {
			t.Fatal(err)
		}
		if answered != 2 {
			t.Fatalf("answered = %d, want 2", answered)
		}

		correct, err := q.CountCorrectAnswers("attempt-1")
		if err != nil {
			t.Fatal(err)
		}
		if correct != 1 {
			t.Fatalf("correct = %d, want 1", correct)
		}
	})

	t.Run("save (via RecordAnswer) refreshes the paired resume index's TTL, not just the blob's", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		q := NewRedisQuizSessionQueries(db, redisClient)
		ctx := context.Background()

		if err := q.CreateAttempt("attempt-1", 42, 7, []int{101}); err != nil {
			t.Fatal(err)
		}
		// Shrink the index key's TTL well below attemptTTL, simulating time
		// having passed since CreateAttempt.
		shrunk := 5 * time.Second
		if err := redisClient.Expire(ctx, "quiz:active-attempt:7:42", shrunk).Err(); err != nil {
			t.Fatal(err)
		}

		if err := q.RecordAnswer("attempt-1", 101, "null", true, false); err != nil {
			t.Fatal(err)
		}

		indexTTL, err := redisClient.TTL(ctx, "quiz:active-attempt:7:42").Result()
		if err != nil {
			t.Fatal(err)
		}
		if indexTTL <= shrunk {
			t.Fatalf("index TTL = %v, want it restored above the shrunk %v", indexTTL, shrunk)
		}
	})

	t.Run("MarkAttemptCompleted sets completed_at, score, and passed", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		q := NewRedisQuizSessionQueries(db, redisClient)
		if err := q.CreateAttempt("attempt-1", 42, 7, []int{101}); err != nil {
			t.Fatal(err)
		}

		if err := q.MarkAttemptCompleted("attempt-1", 100, true); err != nil {
			t.Fatal(err)
		}

		_, completedAt, err := q.GetAttemptMeta("attempt-1")
		if err != nil {
			t.Fatal(err)
		}
		if completedAt == nil {
			t.Fatal("completedAt is nil, want set")
		}
	})

	t.Run("MarkAttemptCompleted removes the resume index so a completed attempt is never resumed", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		q := NewRedisQuizSessionQueries(db, redisClient)
		if err := q.CreateAttempt("attempt-1", 42, 7, []int{101}); err != nil {
			t.Fatal(err)
		}

		if err := q.MarkAttemptCompleted("attempt-1", 100, true); err != nil {
			t.Fatal(err)
		}

		_, err := q.FindActiveAttemptUUID(7, 42)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound (a completed attempt must not be resumable)", err)
		}
	})

	t.Run("FindActiveAttemptUUID still refuses a completed attempt even if the index-key Del was lost", func(t *testing.T) {
		// Simulates MarkAttemptCompleted's save() succeeding but its
		// subsequent Del() failing (e.g. a transient connection error) - the
		// resume-index key survives even though the attempt is completed.
		// FindActiveAttemptUUID must still authoritatively refuse to hand
		// this attempt back as active/resumable, since it now loads the
		// full blob and checks CompletedAt itself rather than trusting the
		// index key's mere existence.
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		q := NewRedisQuizSessionQueries(db, redisClient)
		ctx := context.Background()

		if err := q.CreateAttempt("attempt-1", 42, 7, []int{101}); err != nil {
			t.Fatal(err)
		}
		if err := q.MarkAttemptCompleted("attempt-1", 100, true); err != nil {
			t.Fatal(err)
		}

		// Re-create the resume-index key, simulating the lost Del - the
		// completed attempt blob is still there, but its index key is back
		// too, exactly as if the Del call had failed.
		if err := redisClient.Set(ctx, "quiz:active-attempt:7:42", "attempt-1", attemptTTL).Err(); err != nil {
			t.Fatal(err)
		}

		_, err := q.FindActiveAttemptUUID(7, 42)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound (a completed attempt must never be resumable, even with a dangling index key)", err)
		}
	})

	t.Run("ReviewPostPaths aggregates missed questions' posts, deduped, in miss order", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		postA := seedPost(t, db, "list-1/a")
		postB := seedPost(t, db, "list-1/b")
		q1 := seedQuestion(t, db, "q1", postA)
		q2 := seedQuestion(t, db, "q2", postB)
		// q2 is also linked to postA - postA must appear only once overall.
		if _, err := db.Exec("INSERT INTO question_posts (question_id, post_id) VALUES (?, ?)", q2, postA); err != nil {
			t.Fatal(err)
		}

		qr := NewRedisQuizSessionQueries(db, redisClient)
		if err := qr.CreateAttempt("attempt-1", 42, 7, []int{q1, q2}); err != nil {
			t.Fatal(err)
		}
		if err := qr.RecordAnswer("attempt-1", q1, "null", false, false); err != nil {
			t.Fatal(err)
		}
		if err := qr.RecordAnswer("attempt-1", q2, "null", false, false); err != nil {
			t.Fatal(err)
		}

		paths, err := qr.ReviewPostPaths("attempt-1")
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != 2 || paths[0] != "list-1/a" || paths[1] != "list-1/b" {
			t.Fatalf("paths = %v, want [list-1/a list-1/b] in miss order, deduped", paths)
		}
	})

	t.Run("ReviewPostPaths returns nothing for a perfect attempt", func(t *testing.T) {
		db := setupTestDB(t)
		redisClient := setupTestRedis(t)
		postID := seedPost(t, db, "list-1/a")
		questionID := seedQuestion(t, db, "q1", postID)

		qr := NewRedisQuizSessionQueries(db, redisClient)
		if err := qr.CreateAttempt("attempt-1", 42, 7, []int{questionID}); err != nil {
			t.Fatal(err)
		}
		if err := qr.RecordAnswer("attempt-1", questionID, `{"selectedOptionIds":["b"]}`, true, false); err != nil {
			t.Fatal(err)
		}

		paths, err := qr.ReviewPostPaths("attempt-1")
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != 0 {
			t.Fatalf("paths = %v, want empty", paths)
		}
	})
}
