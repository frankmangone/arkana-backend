-- +goose Up
ALTER TABLE post_content RENAME TO post_contents;
ALTER TABLE post_contents ADD COLUMN thumbnail TEXT;
DROP INDEX idx_post_content_post_id;
CREATE INDEX idx_post_contents_post_id ON post_contents(post_id);

-- +goose Down
DROP INDEX idx_post_contents_post_id;
ALTER TABLE post_contents DROP COLUMN thumbnail;
ALTER TABLE post_contents RENAME TO post_content;
CREATE INDEX idx_post_content_post_id ON post_content(post_id);
