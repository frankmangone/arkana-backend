package queries

import (
	"arkana/features/quizzes/models"
	dbpkg "arkana/shared/db"
	"arkana/shared/idgen"
	"database/sql"
)

type QuestionQueries interface {
	UpsertQuestion(p models.QuestionPayload) (int, error)
	UpsertTranslations(questionID int, translations map[string]models.QuestionTranslationPayload) error
	RelinkTags(questionID int, slugs []string, tagIDs map[string]int) error
	RelinkPosts(questionID int, paths []string, postIDs map[string]int) error
	WithTx(tx *sql.Tx) QuestionQueries
}

type SQLQuestionQueries struct {
	db dbpkg.DBTX
}

func NewSQLQuestionQueries(db dbpkg.DBTX) *SQLQuestionQueries {
	return &SQLQuestionQueries{db: db}
}

func (q *SQLQuestionQueries) WithTx(tx *sql.Tx) QuestionQueries {
	return NewSQLQuestionQueries(tx)
}

// UpsertQuestion inserts or updates the questions row for one payload,
// generating a fresh uuid only on first insert (an UPDATE via ON
// CONFLICT never touches an existing uuid).
func (q *SQLQuestionQueries) UpsertQuestion(p models.QuestionPayload) (int, error) {
	var existingID sql.NullInt64
	if err := q.db.QueryRow("SELECT id FROM questions WHERE slug = ?", p.Slug).Scan(&existingID); err != nil && err != sql.ErrNoRows {
		return 0, err
	}

	answerKey := string(p.AnswerKey)
	if existingID.Valid {
		if _, err := q.db.Exec(
			`UPDATE questions SET type = ?, difficulty = ?, answer_key = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			p.Type, p.Difficulty, answerKey, existingID.Int64,
		); err != nil {
			return 0, err
		}
		return int(existingID.Int64), nil
	}

	uuid, err := idgen.NewV4()
	if err != nil {
		return 0, err
	}
	result, err := q.db.Exec(
		`INSERT INTO questions (uuid, slug, type, difficulty, answer_key) VALUES (?, ?, ?, ?, ?)`,
		uuid, p.Slug, p.Type, p.Difficulty, answerKey,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

// UpsertTranslations add/update-syncs a question's translations.
func (q *SQLQuestionQueries) UpsertTranslations(questionID int, translations map[string]models.QuestionTranslationPayload) error {
	for lang, t := range translations {
		if _, err := q.db.Exec(
			`INSERT INTO question_translations (question_id, lang, prompt, content) VALUES (?, ?, ?, ?)
			 ON CONFLICT(question_id, lang) DO UPDATE SET prompt = excluded.prompt, content = excluded.content`,
			questionID, lang, t.Prompt, string(t.Content),
		); err != nil {
			return err
		}
	}
	return nil
}

// RelinkTags deletes then reinserts this one question's tag pivot rows.
func (q *SQLQuestionQueries) RelinkTags(questionID int, slugs []string, tagIDs map[string]int) error {
	if _, err := q.db.Exec("DELETE FROM question_tags WHERE question_id = ?", questionID); err != nil {
		return err
	}
	for _, slug := range slugs {
		if _, err := q.db.Exec(
			"INSERT INTO question_tags (question_id, tag_id) VALUES (?, ?)",
			questionID, tagIDs[slug],
		); err != nil {
			return err
		}
	}
	return nil
}

// RelinkPosts deletes then reinserts this one question's post pivot rows.
func (q *SQLQuestionQueries) RelinkPosts(questionID int, paths []string, postIDs map[string]int) error {
	if _, err := q.db.Exec("DELETE FROM question_posts WHERE question_id = ?", questionID); err != nil {
		return err
	}
	for _, path := range paths {
		if _, err := q.db.Exec(
			"INSERT INTO question_posts (question_id, post_id) VALUES (?, ?)",
			questionID, postIDs[path],
		); err != nil {
			return err
		}
	}
	return nil
}
