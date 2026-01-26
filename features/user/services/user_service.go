package services

import (
	"arkana/features/user/models"
	"database/sql"
)

// UserService handles user business logic
type UserService struct {
	db *sql.DB
}

// NewUserService creates a new user service
func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}

// GetByID retrieves a user by ID
func (s *UserService) GetByID(id int) (*models.User, error) {
	user := &models.User{}
	err := s.db.QueryRow(
		"SELECT id, email, username, created_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Email, &user.Username, &user.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return user, nil
}

// GetByEmail retrieves a user by email
func (s *UserService) GetByEmail(email string) (*models.User, error) {
	user := &models.User{}
	err := s.db.QueryRow(
		"SELECT id, email, username, password, created_at FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.Email, &user.Username, &user.Password, &user.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return user, nil
}

// Create creates a new user
func (s *UserService) Create(email, username, password string) (*models.User, error) {
	result, err := s.db.Exec(
		"INSERT INTO users (email, username, password) VALUES (?, ?, ?)",
		email, username, password,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return s.GetByID(int(id))
}
