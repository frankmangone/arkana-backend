package services

import (
	"arkana/features/writers/models"
	"database/sql"
	"encoding/json"
	"errors"
)

var ErrWriterNotFound = errors.New("writer not found")

type WriterService struct {
	db *sql.DB
}

func NewWriterService(db *sql.DB) *WriterService {
	return &WriterService{db: db}
}

// GetBySlug returns the full public profile for a visible writer, or
// ErrWriterNotFound if no visible writer has that slug.
func (s *WriterService) GetBySlug(slug string) (*models.WriterResponse, error) {
	var resp models.WriterResponse
	var organization, bio, social, walletAddress sql.NullString

	err := s.db.QueryRow(
		`SELECT slug, name, image_url, avatar_url, organization, bio, social, wallet_address
		 FROM writers WHERE slug = ? AND visible = 1`,
		slug,
	).Scan(&resp.Slug, &resp.Name, &resp.ImageURL, &resp.AvatarURL, &organization, &bio, &social, &walletAddress)
	if err == sql.ErrNoRows {
		return nil, ErrWriterNotFound
	}
	if err != nil {
		return nil, err
	}

	if organization.Valid {
		resp.Organization = &models.Organization{}
		if err := json.Unmarshal([]byte(organization.String), resp.Organization); err != nil {
			return nil, err
		}
	}
	if bio.Valid {
		if err := json.Unmarshal([]byte(bio.String), &resp.Bio); err != nil {
			return nil, err
		}
	}
	if social.Valid {
		resp.Social = &models.Social{}
		if err := json.Unmarshal([]byte(social.String), resp.Social); err != nil {
			return nil, err
		}
	}
	if walletAddress.Valid {
		resp.WalletAddress = walletAddress.String
	}

	return &resp, nil
}

// List returns minimal profile info for every visible writer, ordered by name.
func (s *WriterService) List() ([]models.WriterSummary, error) {
	rows, err := s.db.Query(
		`SELECT slug, name, image_url, avatar_url FROM writers WHERE visible = 1 ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	writers := []models.WriterSummary{}
	for rows.Next() {
		var w models.WriterSummary
		if err := rows.Scan(&w.Slug, &w.Name, &w.ImageURL, &w.AvatarURL); err != nil {
			return nil, err
		}
		writers = append(writers, w)
	}

	return writers, rows.Err()
}
