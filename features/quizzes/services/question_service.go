package services

import (
	"arkana/features/quizzes/models"
	"arkana/features/quizzes/queries"
	dbpkg "arkana/shared/db"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrUnknownPosts = errors.New("unknown post path(s)")
var ErrUnknownTags = errors.New("unknown tag(s)")

// knownQuestionTypes mirrors the set grade() (grading.go) actually
// dispatches on - kept in sync with it so a type that would fail at
// answer-time with ErrUnknownQuestionType is instead rejected here, at
// publish-time.
var knownQuestionTypes = map[string]bool{
	"single_choice": true,
	"multi_choice":  true,
	"matching":      true,
	"range":         true,
	"sequencing":    true,
	"bucket_sort":   true,
	"fill_blank":    true,
}

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
	db      *sql.DB
	queries queries.QuestionQueries
	posts   PostChecker
	tags    TagChecker
}

// NewQuestionService constructs a QuestionService backed by db, using posts
// and tags to validate post paths and tag slugs referenced by published
// questions.
func NewQuestionService(db *sql.DB, posts PostChecker, tags TagChecker) *QuestionService {
	return &QuestionService{db: db, queries: queries.NewSQLQuestionQueries(db), posts: posts, tags: tags}
}

// Publish upserts a batch of questions in one transaction, add/update
// only (mirrors TagService.Sync) — there's no persisted "quiz" entity to
// fully replace, so a question absent from a later payload is left
// untouched, never deleted. All post paths and tags across the whole
// batch are validated before any write; one bad reference anywhere fails
// the entire call.
func (s *QuestionService) Publish(payloads []models.QuestionPayload) (int, error) {
	var allPaths, allTags, unknownTypes []string
	for _, p := range payloads {
		allPaths = append(allPaths, p.PostPaths...)
		allTags = append(allTags, p.Tags...)
		if !knownQuestionTypes[p.Type] {
			unknownTypes = append(unknownTypes, p.Type)
		}
	}

	if len(unknownTypes) > 0 {
		return 0, fmt.Errorf("%w: %s", ErrUnknownQuestionType, strings.Join(dedupe(unknownTypes), ", "))
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

	err = dbpkg.Transact(s.db, func(tx *sql.Tx) error {
		qtx := s.queries.WithTx(tx)

		for _, p := range payloads {
			questionID, err := qtx.UpsertQuestion(p)
			if err != nil {
				return err
			}
			if err := qtx.UpsertTranslations(questionID, p.Translations); err != nil {
				return err
			}
			if err := qtx.RelinkTags(questionID, p.Tags, tagIDs); err != nil {
				return err
			}
			if err := qtx.RelinkPosts(questionID, p.PostPaths, postIDs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return len(payloads), nil
}

// dedupe returns items with duplicates removed, preserving first-occurrence
// order.
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
