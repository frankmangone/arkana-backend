package services

import (
	"database/sql"
	"errors"

	"arkana/features/questionflags/models"
	"arkana/features/questionflags/queries"
)

// MaxFlagReasonLength is the maximum allowed length for a flag's reason.
const MaxFlagReasonLength = 1000

var (
	ErrQuestionNotFound = errors.New("question not found")
	ErrReasonTooLong    = errors.New("reason exceeds maximum length")
)

type QuestionFlagService struct {
	db      *sql.DB
	queries queries.QuestionFlagQueries
}

func NewQuestionFlagService(db *sql.DB) *QuestionFlagService {
	return &QuestionFlagService{db: db, queries: queries.NewSQLQuestionFlagQueries(db)}
}

// Create records userID's feedback on the question identified by
// questionUUID. A second flag from the same user on the same question
// overwrites the first rather than creating a duplicate.
func (s *QuestionFlagService) Create(questionUUID string, userID int, reason string) (*models.QuestionFlag, error) {
	if len(reason) > MaxFlagReasonLength {
		return nil, ErrReasonTooLong
	}

	questionID, err := s.queries.ResolveQuestionID(questionUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrQuestionNotFound
	}
	if err != nil {
		return nil, err
	}

	return s.queries.Upsert(questionID, userID, reason)
}

// List returns every flag, newest first.
func (s *QuestionFlagService) List() ([]models.QuestionFlag, error) {
	return s.queries.List()
}

// DeleteAll removes every flag, returning how many were removed.
func (s *QuestionFlagService) DeleteAll() (int64, error) {
	return s.queries.DeleteAll()
}

// Delete removes a single flag by id, returning how many were removed.
func (s *QuestionFlagService) Delete(id int) (int64, error) {
	return s.queries.Delete(id)
}
