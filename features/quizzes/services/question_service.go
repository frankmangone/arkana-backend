package services

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"arkana/features/quizzes/models"
	"arkana/shared/idgen"
)

var ErrUnknownPosts = errors.New("unknown post path(s)")
var ErrUnknownTags = errors.New("unknown tag(s)")

// PostChecker validates post paths and resolves them to posts.id. Both
// methods use only primitive types (no *posts/models.Post) so this
// interface never forces an import of features/posts/models. Satisfied
// structurally by *posts/services.PostService.
type PostChecker interface {
	MissingPaths(paths []string) ([]string, error)
	GetIDsByPaths(paths []string) (map[string]int, error)
}

// TagChecker validates tag slugs and resolves them to tags.id. Satisfied
// structurally by *tags/services.TagService.
type TagChecker interface {
	MissingTags(slugs []string) ([]string, error)
	GetIDsBySlugs(slugs []string) (map[string]int, error)
}

type QuestionService struct {
	db   *sql.DB
	posts PostChecker
	tags  TagChecker
}

func NewQuestionService(db *sql.DB, posts PostChecker, tags TagChecker) *QuestionService {
	return &QuestionService{db: db, posts: posts, tags: tags}
}

// Publish upserts a batch of questions in one transaction, add/update
// only (mirrors TagService.Sync) — there's no persisted "quiz" entity to
// fully replace, so a question absent from a later payload is left
// untouched, never deleted. All post paths and tags across the whole
// batch are validated before any write; one bad reference anywhere fails
// the entire call.
func (s *QuestionService) Publish(payloads []models.QuestionPayload) (int, error) {
	var allPaths, allTags []string
	for _, p := range payloads {
		allPaths = append(allPaths, p.PostPaths...)
		allTags = append(allTags, p.Tags...)
	}

	if missing, err := s.posts.MissingPaths(dedupe(allPaths)); err != nil {
		return 0, err
	} else if len(missing) > 0 {
		return 0, fmt.Errorf("%w: %s", ErrUnknownPosts, strings.Join(missing, ", "))
	}
	if missing, err := s.tags.MissingTags(dedupe(allTags)); err != nil {
		return 0, err
	} else if len(missing) > 0 {
		return 0, fmt.Errorf("%w: %s", ErrUnknownTags, strings.Join(missing, ", "))
	}

	postIDs, err := s.posts.GetIDsByPaths(allPaths)
	if err != nil {
		return 0, err
	}
	tagIDs, err := s.tags.GetIDsBySlugs(allTags)
	if err != nil {
		return 0, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	for _, p := range payloads {
		questionID, err := upsertQuestion(tx, p)
		if err != nil {
			return 0, err
		}
		if err := upsertTranslations(tx, questionID, p.Translations); err != nil {
			return 0, err
		}
		if err := relinkTags(tx, questionID, p.Tags, tagIDs); err != nil {
			return 0, err
		}
		if err := relinkPosts(tx, questionID, p.PostPaths, postIDs); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(payloads), nil
}

func dedupe(items []string) []string {
	seen := make(map[string]bool, len(items))
	var out []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

// upsertQuestion inserts or updates the questions row for one payload,
// generating a fresh uuid only on first insert (an UPDATE via ON
// CONFLICT never touches an existing uuid).
func upsertQuestion(tx *sql.Tx, p models.QuestionPayload) (int, error) {
	var existingID sql.NullInt64
	if err := tx.QueryRow("SELECT id FROM questions WHERE slug = ?", p.Slug).Scan(&existingID); err != nil && err != sql.ErrNoRows {
		return 0, err
	}

	answerKey := string(p.AnswerKey)
	if existingID.Valid {
		if _, err := tx.Exec(
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
	result, err := tx.Exec(
		`INSERT INTO questions (uuid, slug, type, difficulty, answer_key) VALUES (?, ?, ?, ?, ?)`,
		uuid, p.Slug, p.Type, p.Difficulty, answerKey,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func upsertTranslations(tx *sql.Tx, questionID int, translations map[string]models.QuestionTranslationPayload) error {
	for lang, t := range translations {
		if _, err := tx.Exec(
			`INSERT INTO question_translations (question_id, lang, prompt, content) VALUES (?, ?, ?, ?)
			 ON CONFLICT(question_id, lang) DO UPDATE SET prompt = excluded.prompt, content = excluded.content`,
			questionID, lang, t.Prompt, string(t.Content),
		); err != nil {
			return err
		}
	}
	return nil
}

// relinkTags/relinkPosts delete-then-reinsert this one question's own
// pivot rows only (scoped by question_id) — safe and cheap since these
// tables have no ownership ambiguity: a question_posts row belongs
// unambiguously to the question_id it names.
func relinkTags(tx *sql.Tx, questionID int, slugs []string, tagIDs map[string]int) error {
	if _, err := tx.Exec("DELETE FROM question_tags WHERE question_id = ?", questionID); err != nil {
		return err
	}
	for _, slug := range slugs {
		if _, err := tx.Exec(
			"INSERT INTO question_tags (question_id, tag_id) VALUES (?, ?)",
			questionID, tagIDs[slug],
		); err != nil {
			return err
		}
	}
	return nil
}

func relinkPosts(tx *sql.Tx, questionID int, paths []string, postIDs map[string]int) error {
	if _, err := tx.Exec("DELETE FROM question_posts WHERE question_id = ?", questionID); err != nil {
		return err
	}
	for _, path := range paths {
		if _, err := tx.Exec(
			"INSERT INTO question_posts (question_id, post_id) VALUES (?, ?)",
			questionID, postIDs[path],
		); err != nil {
			return err
		}
	}
	return nil
}
