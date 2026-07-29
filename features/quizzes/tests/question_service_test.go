package tests

import (
	"encoding/json"
	"errors"
	"testing"

	"arkana/features/quizzes/models"
	"arkana/features/quizzes/services"
)

func TestQuestionServicePublish(t *testing.T) {
	t.Run("creates a question with translations, tags, and post links", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewQuestionService(db, &fakePostChecker{}, &fakeTagChecker{})

		n, err := svc.Publish([]models.QuestionPayload{
			{
				Slug:       "pow-difficulty-basics",
				Type:       "single_choice",
				Difficulty: 1,
				PostPaths:  []string{"blockchain-101/how-it-all-began"},
				Tags:       []string{"cryptography"},
				AnswerKey:  json.RawMessage(`{"correctOptionIds":["b"]}`),
				Translations: map[string]models.QuestionTranslationPayload{
					"en": {Prompt: "What is PoW?", Content: json.RawMessage(`{"options":[]}`)},
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("Publish returned %d, want 1", n)
		}

		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM questions WHERE slug = 'pow-difficulty-basics'").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("questions row count = %d, want 1", count)
		}
		if err := db.QueryRow(`
			SELECT COUNT(*) FROM question_translations qt
			JOIN questions q ON q.id = qt.question_id
			WHERE q.slug = 'pow-difficulty-basics' AND qt.lang = 'en'
		`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("question_translations row count = %d, want 1", count)
		}
		if err := db.QueryRow(`
			SELECT COUNT(*) FROM question_posts qp
			JOIN questions q ON q.id = qp.question_id
			WHERE q.slug = 'pow-difficulty-basics'
		`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("question_posts row count = %d, want 1", count)
		}
	})

	t.Run("republishing the same slug updates in place, no duplicates", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewQuestionService(db, &fakePostChecker{}, &fakeTagChecker{})
		payload := models.QuestionPayload{
			Slug:       "pow-difficulty-basics",
			Type:       "single_choice",
			Difficulty: 1,
			PostPaths:  []string{"blockchain-101/how-it-all-began"},
			AnswerKey:  json.RawMessage(`{"correctOptionIds":["b"]}`),
			Translations: map[string]models.QuestionTranslationPayload{
				"en": {Prompt: "v1", Content: json.RawMessage(`{}`)},
			},
		}
		if _, err := svc.Publish([]models.QuestionPayload{payload}); err != nil {
			t.Fatal(err)
		}
		payload.Translations["en"] = models.QuestionTranslationPayload{Prompt: "v2", Content: json.RawMessage(`{}`)}
		if _, err := svc.Publish([]models.QuestionPayload{payload}); err != nil {
			t.Fatal(err)
		}

		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM questions").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("questions row count = %d, want 1 (no duplicate)", count)
		}
		var prompt string
		if err := db.QueryRow("SELECT prompt FROM question_translations").Scan(&prompt); err != nil {
			t.Fatal(err)
		}
		if prompt != "v2" {
			t.Fatalf("prompt = %q, want %q (updated in place)", prompt, "v2")
		}
	})

	t.Run("rejects the whole batch when a post path is unregistered", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewQuestionService(db, &fakePostChecker{missing: []string{"nonexistent/path"}}, &fakeTagChecker{})

		_, err := svc.Publish([]models.QuestionPayload{
			{Slug: "q1", Type: "single_choice", Difficulty: 1, PostPaths: []string{"nonexistent/path"}, AnswerKey: json.RawMessage(`{}`)},
		})
		if !errors.Is(err, services.ErrUnknownPosts) {
			t.Fatalf("err = %v, want ErrUnknownPosts", err)
		}
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM questions").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("questions row count = %d, want 0 (nothing written)", count)
		}
	})

	t.Run("rejects the whole batch when a tag is unregistered", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewQuestionService(db, &fakePostChecker{}, &fakeTagChecker{missing: []string{"nonexistent-tag"}})

		_, err := svc.Publish([]models.QuestionPayload{
			{Slug: "q1", Type: "single_choice", Difficulty: 1, Tags: []string{"nonexistent-tag"}, AnswerKey: json.RawMessage(`{}`)},
		})
		if !errors.Is(err, services.ErrUnknownTags) {
			t.Fatalf("err = %v, want ErrUnknownTags", err)
		}
	})

	t.Run("re-publishing without a previously-linked post removes that link only", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewQuestionService(db, &fakePostChecker{}, &fakeTagChecker{})
		payload := models.QuestionPayload{
			Slug: "q1", Type: "single_choice", Difficulty: 1,
			PostPaths:    []string{"a/one", "a/two"},
			AnswerKey:    json.RawMessage(`{}`),
			Translations: map[string]models.QuestionTranslationPayload{"en": {Prompt: "p", Content: json.RawMessage(`{}`)}},
		}
		if _, err := svc.Publish([]models.QuestionPayload{payload}); err != nil {
			t.Fatal(err)
		}
		payload.PostPaths = []string{"a/one"}
		if _, err := svc.Publish([]models.QuestionPayload{payload}); err != nil {
			t.Fatal(err)
		}

		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM question_posts").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("question_posts row count = %d, want 1", count)
		}
	})
}
