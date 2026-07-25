-- +goose Up
ALTER TABLE writers ADD COLUMN slug TEXT;
ALTER TABLE writers ADD COLUMN image_url TEXT;
ALTER TABLE writers ADD COLUMN avatar_url TEXT;
ALTER TABLE writers ADD COLUMN visible BOOLEAN NOT NULL DEFAULT 1;
ALTER TABLE writers ADD COLUMN organization TEXT;
ALTER TABLE writers ADD COLUMN bio TEXT;
ALTER TABLE writers ADD COLUMN social TEXT;
ALTER TABLE writers ADD COLUMN wallet_address TEXT;

CREATE UNIQUE INDEX idx_writers_slug ON writers(slug);

-- +goose Down
DROP INDEX IF EXISTS idx_writers_slug;
ALTER TABLE writers DROP COLUMN wallet_address;
ALTER TABLE writers DROP COLUMN social;
ALTER TABLE writers DROP COLUMN bio;
ALTER TABLE writers DROP COLUMN organization;
ALTER TABLE writers DROP COLUMN visible;
ALTER TABLE writers DROP COLUMN avatar_url;
ALTER TABLE writers DROP COLUMN image_url;
ALTER TABLE writers DROP COLUMN slug;
