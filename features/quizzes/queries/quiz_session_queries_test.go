package queries

import (
	"database/sql"
	"testing"

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
