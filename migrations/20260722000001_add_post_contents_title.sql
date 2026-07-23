-- +goose Up
ALTER TABLE post_contents ADD COLUMN title TEXT;

-- +goose Down
ALTER TABLE post_contents DROP COLUMN title;
