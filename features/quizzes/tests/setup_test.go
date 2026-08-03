package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	authsvc "arkana/features/auth/services"

	"github.com/alicebob/miniredis/v2"
	_ "github.com/mattn/go-sqlite3"
	"github.com/redis/go-redis/v9"
)

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
			path_identifier TEXT UNIQUE NOT NULL,
			like_count INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT UNIQUE NOT NULL,
			auth_provider TEXT NOT NULL,
			provider_user_id TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE reading_lists (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT UNIQUE NOT NULL,
			cover_image TEXT,
			ongoing BOOLEAN NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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
			answer_key TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE question_translations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			question_id INTEGER NOT NULL REFERENCES questions(id),
			lang TEXT NOT NULL,
			prompt TEXT NOT NULL,
			content TEXT NOT NULL,
			UNIQUE (question_id, lang)
		);
		CREATE TABLE question_tags (
			question_id INTEGER NOT NULL REFERENCES questions(id),
			tag_id INTEGER NOT NULL REFERENCES tags(id),
			PRIMARY KEY (question_id, tag_id)
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

const testAdminSecret = "test-admin-secret"
const testJWTSecret = "test-secret-key"

func generateTestJWT(t *testing.T, userID int, email string) string {
	t.Helper()
	token, err := authsvc.GenerateAccessToken(userID, email, testJWTSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

// fakePostChecker/fakeTagChecker mirror features/readinglists/tests'
// fakePostChecker: tests that don't exercise validation failures don't
// need real posts/tags rows, only these small in-memory fakes. GetIDs*
// assigns each distinct path/slug a stable auto-incrementing id the
// first time it's seen and remembers it on the instance - a test that
// calls Publish more than once with overlapping paths (e.g. a republish
// test) gets the same id back both times, instead of an id that depends
// on that path's position within any one call's slice.
type fakePostChecker struct {
	missing []string
	ids     map[string]int
}

func (f *fakePostChecker) MissingPaths(paths []string) ([]string, error) {
	return f.missing, nil
}

func (f *fakePostChecker) GetIDsByPaths(paths []string) (map[string]int, error) {
	if f.ids == nil {
		f.ids = make(map[string]int)
	}
	result := make(map[string]int, len(paths))
	for _, p := range paths {
		id, ok := f.ids[p]
		if !ok {
			id = len(f.ids) + 1
			f.ids[p] = id
		}
		result[p] = id
	}
	return result, nil
}

type fakeTagChecker struct {
	missing []string
	ids     map[string]int
}

func (f *fakeTagChecker) MissingTags(slugs []string) ([]string, error) {
	return f.missing, nil
}

func (f *fakeTagChecker) GetIDsBySlugs(slugs []string) (map[string]int, error) {
	if f.ids == nil {
		f.ids = make(map[string]int)
	}
	result := make(map[string]int, len(slugs))
	for _, s := range slugs {
		id, ok := f.ids[s]
		if !ok {
			id = len(f.ids) + 1
			f.ids[s] = id
		}
		result[s] = id
	}
	return result, nil
}

func insertTestUser(t *testing.T, db *sql.DB, email string) int {
	t.Helper()
	result, err := db.Exec(
		`INSERT INTO users (email, auth_provider, provider_user_id) VALUES (?, 'google', ?)`,
		email, email,
	)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	return int(id)
}

func insertTestPost(t *testing.T, db *sql.DB, path string) int {
	t.Helper()
	result, err := db.Exec("INSERT INTO posts (path_identifier, like_count) VALUES (?, 0)", path)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	return int(id)
}

// insertTestModule seeds a full reading_lists -> reading_list_modules ->
// reading_list_items chain (one item, pointing at postPath) and returns
// the module's id - the exact granularity QuizSessionService queries by.
func insertTestModule(t *testing.T, db *sql.DB, listSlug, moduleSlug, itemSlug, postPath string) int {
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

// insertTestQuestion seeds a questions row plus its question_posts link
// to postID, returning the question's internal id.
func insertTestQuestion(t *testing.T, db *sql.DB, slug string, postID int) int {
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

func insertTestQuestionTranslation(t *testing.T, db *sql.DB, questionID int, lang, prompt, content string) {
	t.Helper()
	if _, err := db.Exec(
		"INSERT INTO question_translations (question_id, lang, prompt, content) VALUES (?, ?, ?, ?)",
		questionID, lang, prompt, content,
	); err != nil {
		t.Fatal(err)
	}
}

// setupTestRedis spins up an in-memory miniredis instance and returns a
// real *redis.Client pointed at it - every quiz session test needing
// Redis-backed attempt state uses this instead of a real Redis server.
func setupTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

// redisAttemptFixture mirrors the subset of RedisQuizSessionQueries's
// internal attempt JSON shape that tests need to assert on directly - a
// deliberate black-box duplication (matching the wire tags, not the
// unexported type) rather than reaching into package queries' internals.
type redisAttemptFixture struct {
	ModuleID  int   `json:"module_id"`
	UserID    int   `json:"user_id"`
	Questions []int `json:"questions"`
}

func getRedisAttempt(t *testing.T, redisClient *redis.Client, attemptUUID string) redisAttemptFixture {
	t.Helper()
	data, err := redisClient.Get(context.Background(), "quiz:attempt:"+attemptUUID).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	var fixture redisAttemptFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}
