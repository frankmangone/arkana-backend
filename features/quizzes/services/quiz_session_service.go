package services

import "database/sql"

// QuizSessionService owns the attempt/next/answer/complete flow. Methods
// are added in a later implementation task; this file exists now only so
// handlers.RegisterRoutes has a concrete type to accept.
type QuizSessionService struct {
	db *sql.DB
}

func NewQuizSessionService(db *sql.DB) *QuizSessionService {
	return &QuizSessionService{db: db}
}
