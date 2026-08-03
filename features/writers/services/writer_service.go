package services

import (
	"arkana/features/writers/models"
	"arkana/features/writers/queries"
	"database/sql"
	"errors"
)

var ErrWriterNotFound = errors.New("writer not found")

type WriterService struct {
	db      *sql.DB
	queries queries.WriterQueries
}

func NewWriterService(db *sql.DB) *WriterService {
	return &WriterService{db: db, queries: queries.NewSQLWriterQueries(db)}
}

// GetBySlug returns the full public profile for a visible writer, or
// ErrWriterNotFound if no visible writer has that slug.
func (s *WriterService) GetBySlug(slug string) (*models.WriterResponse, error) {
	resp, err := s.queries.GetBySlug(slug)
	if err == sql.ErrNoRows {
		return nil, ErrWriterNotFound
	}
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ListAll returns the full profile for every writer that has a slug,
// regardless of visibility.
func (s *WriterService) ListAll() ([]models.WriterResponse, error) {
	return s.queries.ListAll()
}

// List returns minimal profile info for every visible writer, ordered by name.
func (s *WriterService) List() ([]models.WriterSummary, error) {
	return s.queries.List()
}
