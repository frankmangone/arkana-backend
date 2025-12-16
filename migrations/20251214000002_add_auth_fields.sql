-- +goose Up
-- Add authentication-related fields to users table
ALTER TABLE users ADD COLUMN auth_provider TEXT DEFAULT 'email';
ALTER TABLE users ADD COLUMN provider_user_id TEXT;
ALTER TABLE users ADD COLUMN password_hash TEXT;
ALTER TABLE users ADD COLUMN email_verified BOOLEAN DEFAULT 0;
ALTER TABLE users ADD COLUMN avatar_url TEXT;
ALTER TABLE users ADD COLUMN updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Create index for provider lookups
CREATE INDEX idx_users_provider ON users(auth_provider, provider_user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_users_provider;
ALTER TABLE users DROP COLUMN auth_provider;
ALTER TABLE users DROP COLUMN provider_user_id;
ALTER TABLE users DROP COLUMN password_hash;
ALTER TABLE users DROP COLUMN email_verified;
ALTER TABLE users DROP COLUMN avatar_url;
ALTER TABLE users DROP COLUMN updated_at;
