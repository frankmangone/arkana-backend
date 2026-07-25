package services

import (
	"arkana/features/writers/models"
	"database/sql"
	"encoding/json"
)

// AdminWriterService handles admin/CI-driven writer profile publishing.
type AdminWriterService struct {
	db *sql.DB
}

func NewAdminWriterService(db *sql.DB) *AdminWriterService {
	return &AdminWriterService{db: db}
}

// Publish upserts a writer row by slug: creates it if the slug is new, or
// updates the existing row's profile fields otherwise. user_id is never
// part of this statement, so it's left untouched (NULL on insert, preserved
// on update) - it's an internal notification-routing link set elsewhere,
// not part of the writer's public profile.
func (s *AdminWriterService) Publish(payload models.WriterPayload) error {
	visible := true
	if payload.Visible != nil {
		visible = *payload.Visible
	}

	organization, err := toNullJSON(payload.Organization, payload.Organization == nil)
	if err != nil {
		return err
	}
	bio, err := toNullJSON(payload.Bio, len(payload.Bio) == 0)
	if err != nil {
		return err
	}
	social, err := toNullJSON(payload.Social, payload.Social == nil)
	if err != nil {
		return err
	}

	var walletAddress sql.NullString
	if payload.WalletAddress != "" {
		walletAddress = sql.NullString{String: payload.WalletAddress, Valid: true}
	}

	_, err = s.db.Exec(
		`INSERT INTO writers (slug, name, image_url, avatar_url, visible, organization, bio, social, wallet_address)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(slug) DO UPDATE SET
		   name = excluded.name,
		   image_url = excluded.image_url,
		   avatar_url = excluded.avatar_url,
		   visible = excluded.visible,
		   organization = excluded.organization,
		   bio = excluded.bio,
		   social = excluded.social,
		   wallet_address = excluded.wallet_address,
		   updated_at = CURRENT_TIMESTAMP`,
		payload.Slug, payload.Name, payload.ImageURL, payload.AvatarURL, visible,
		organization, bio, social, walletAddress,
	)
	return err
}

func toNullJSON(v any, isEmpty bool) (sql.NullString, error) {
	if isEmpty {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}
