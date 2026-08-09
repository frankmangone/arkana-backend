package tests

import (
	"errors"
	"strings"
	"testing"

	"arkana/features/questionflags/services"
)

func TestQuestionFlagServiceCreate(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewQuestionFlagService(db)
	userID := insertTestUser(t, db, "service@example.com")
	insertTestQuestion(t, db, "service-question-uuid", "service-question-slug")

	t.Run("creates a flag for a known question", func(t *testing.T) {
		flag, err := svc.Create("service-question-uuid", userID, "unclear wording")
		if err != nil {
			t.Fatal(err)
		}
		if flag.Reason != "unclear wording" {
			t.Errorf("reason = %q, want %q", flag.Reason, "unclear wording")
		}
	})

	t.Run("returns ErrQuestionNotFound for an unknown uuid", func(t *testing.T) {
		_, err := svc.Create("no-such-uuid", userID, "wrong")
		if !errors.Is(err, services.ErrQuestionNotFound) {
			t.Errorf("err = %v, want ErrQuestionNotFound", err)
		}
	})

	t.Run("returns ErrReasonTooLong past the max length", func(t *testing.T) {
		_, err := svc.Create("service-question-uuid", userID, strings.Repeat("x", services.MaxFlagReasonLength+1))
		if !errors.Is(err, services.ErrReasonTooLong) {
			t.Errorf("err = %v, want ErrReasonTooLong", err)
		}
	})
}

func TestQuestionFlagServiceDelete(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewQuestionFlagService(db)
	userID := insertTestUser(t, db, "deleteservice@example.com")
	insertTestQuestion(t, db, "delete-question-uuid", "delete-question-slug")

	if _, err := svc.Create("delete-question-uuid", userID, "reason"); err != nil {
		t.Fatal(err)
	}

	t.Run("DeleteAll removes every flag", func(t *testing.T) {
		deleted, err := svc.DeleteAll()
		if err != nil {
			t.Fatal(err)
		}
		if deleted != 1 {
			t.Errorf("deleted = %d, want 1", deleted)
		}

		flags, err := svc.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(flags) != 0 {
			t.Errorf("len(flags) = %d, want 0", len(flags))
		}
	})
}
