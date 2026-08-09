package queries

import (
	"arkana/features/writers/models"
	dbpkg "arkana/shared/db"
	"database/sql"
	"encoding/json"
)

type WriterQueries interface {
	GetBySlug(slug string) (*models.WriterResponse, error)
	GetIDBySlug(slug string) (int64, error)
	ListAll() ([]models.WriterResponse, error)
	List() ([]models.WriterSummary, error)
}

type SQLWriterQueries struct {
	db dbpkg.DBTX
}

func NewSQLWriterQueries(db dbpkg.DBTX) *SQLWriterQueries {
	return &SQLWriterQueries{db: db}
}

// GetBySlug returns the full public profile for a visible writer. Returns
// sql.ErrNoRows (unmodified) if no visible writer has that slug — the
// service maps this to ErrWriterNotFound.
func (q *SQLWriterQueries) GetBySlug(slug string) (*models.WriterResponse, error) {
	var resp models.WriterResponse
	var organization, bio, social, walletAddress sql.NullString

	err := q.db.QueryRow(
		`SELECT slug, name, COALESCE(image_url, ''), COALESCE(avatar_url, ''), organization, bio, social, wallet_address
		 FROM writers WHERE slug = ? AND visible = 1`,
		slug,
	).Scan(&resp.Slug, &resp.Name, &resp.ImageURL, &resp.AvatarURL, &organization, &bio, &social, &walletAddress)
	if err != nil {
		return nil, err
	}

	if err := hydrateWriter(&resp, organization, bio, social, walletAddress); err != nil {
		return nil, err
	}

	return &resp, nil
}

// GetIDBySlug returns a writer's internal id by slug, regardless of
// visibility - for internal linking (e.g. resolving a post's author to
// writers.id), not public-facing lookups. Returns sql.ErrNoRows
// (unmodified) if no writer has that slug.
func (q *SQLWriterQueries) GetIDBySlug(slug string) (int64, error) {
	var id int64
	err := q.db.QueryRow(`SELECT id FROM writers WHERE slug = ?`, slug).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ListAll returns the full profile for every writer that has a slug,
// regardless of visibility - for the admin-authenticated CI/build pipeline.
func (q *SQLWriterQueries) ListAll() ([]models.WriterResponse, error) {
	rows, err := q.db.Query(
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
func (q *SQLWriterQueries) List() ([]models.WriterSummary, error) {
	rows, err := q.db.Query(
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
