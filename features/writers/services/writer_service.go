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
		`SELECT slug, name, COALESCE(image_url, ''), COALESCE(avatar_url, ''), organization, bio, social, wallet_address
		 FROM writers WHERE slug = ? AND visible = 1`,
		slug,
	).Scan(&resp.Slug, &resp.Name, &resp.ImageURL, &resp.AvatarURL, &organization, &bio, &social, &walletAddress)
	if err == sql.ErrNoRows {
		return nil, ErrWriterNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := hydrateWriter(&resp, organization, bio, social, walletAddress); err != nil {
		return nil, err
	}

	return &resp, nil
}

// ListAll returns the full profile for every writer that has a slug,
// regardless of visibility - unlike List/GetBySlug, which only ever expose
// visible writers to public callers. This is for the admin-authenticated
// CI/build pipeline, which needs hidden writers too (e.g. a hidden writer
// can still be a published post's author) and would otherwise have no way
// to enumerate them, since the public endpoints deliberately can't.
func (s *WriterService) ListAll() ([]models.WriterResponse, error) {
	rows, err := s.db.Query(
		`SELECT slug, name, COALESCE(image_url, ''), COALESCE(avatar_url, ''), organization, bio, social, wallet_address
		 FROM writers WHERE slug IS NOT NULL ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	writers := []models.WriterResponse{}
	for rows.Next() {
		var resp models.WriterResponse
		var organization, bio, social, walletAddress sql.NullString

		if err := rows.Scan(&resp.Slug, &resp.Name, &resp.ImageURL, &resp.AvatarURL, &organization, &bio, &social, &walletAddress); err != nil {
			return nil, err
		}
		if err := hydrateWriter(&resp, organization, bio, social, walletAddress); err != nil {
			return nil, err
		}
		writers = append(writers, resp)
	}

	return writers, rows.Err()
}

// hydrateWriter unmarshals the nullable JSON/text columns shared by
// GetBySlug and ListAll into resp, leaving fields unset when NULL.
func hydrateWriter(resp *models.WriterResponse, organization, bio, social, walletAddress sql.NullString) error {
	if organization.Valid {
		resp.Organization = &models.Organization{}
		if err := json.Unmarshal([]byte(organization.String), resp.Organization); err != nil {
			return err
		}
	}
	if bio.Valid {
		if err := json.Unmarshal([]byte(bio.String), &resp.Bio); err != nil {
			return err
		}
	}
	if social.Valid {
		resp.Social = &models.Social{}
		if err := json.Unmarshal([]byte(social.String), resp.Social); err != nil {
			return err
		}
	}
	if walletAddress.Valid {
		resp.WalletAddress = walletAddress.String
	}
	return nil
}

// List returns minimal profile info for every visible writer, ordered by name.
func (s *WriterService) List() ([]models.WriterSummary, error) {
	rows, err := s.db.Query(
		`SELECT slug, name, COALESCE(image_url, ''), COALESCE(avatar_url, '') FROM writers WHERE visible = 1 AND slug IS NOT NULL ORDER BY name`,
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
