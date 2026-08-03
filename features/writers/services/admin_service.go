package services

import (
	"arkana/features/writers/models"
	"arkana/features/writers/queries"
	"database/sql"
)

// AdminWriterService handles admin/CI-driven writer profile publishing.
type AdminWriterService struct {
	db      *sql.DB
	queries queries.AdminWriterQueries
}

// NewAdminWriterService creates an AdminWriterService backed by the given database connection.
func NewAdminWriterService(db *sql.DB) *AdminWriterService {
	return &AdminWriterService{db: db, queries: queries.NewSQLAdminWriterQueries(db)}
}

// Publish upserts a writer row by slug.
func (s *AdminWriterService) Publish(payload models.WriterPayload) error {
	return s.queries.Publish(payload)
}
