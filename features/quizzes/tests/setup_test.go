package tests

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
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
		CREATE TABLE quiz_attempts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT UNIQUE NOT NULL,
			module_id INTEGER NOT NULL REFERENCES reading_list_modules(id),
			user_id INTEGER NOT NULL REFERENCES users(id),
			tier TEXT NOT NULL DEFAULT 'standard',
			started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP,
			score INTEGER,
			passed BOOLEAN
		);
		CREATE TABLE quiz_attempt_questions (
			attempt_id INTEGER NOT NULL REFERENCES quiz_attempts(id),
			question_id INTEGER NOT NULL REFERENCES questions(id),
			position INTEGER NOT NULL,
			PRIMARY KEY (attempt_id, position)
		);
		CREATE TABLE quiz_attempt_answers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			attempt_id INTEGER NOT NULL REFERENCES quiz_attempts(id),
			question_id INTEGER NOT NULL REFERENCES questions(id),
			response TEXT NOT NULL,
			correct BOOLEAN NOT NULL,
			skipped BOOLEAN NOT NULL DEFAULT 0,
			answered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (attempt_id, question_id)
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

const testAdminSecret = "test-admin-secret"

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
