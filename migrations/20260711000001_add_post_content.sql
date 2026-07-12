-- +goose Up
CREATE TABLE post_content (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id    INTEGER NOT NULL,
    lang       TEXT NOT NULL,
    path       TEXT NOT NULL,
    content    TEXT NOT NULL,
    visible    INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (post_id) REFERENCES posts(id),
    UNIQUE (lang, path)
);
CREATE INDEX idx_post_content_post_id ON post_content(post_id);

-- +goose Down
DROP INDEX IF EXISTS idx_post_content_post_id;
DROP TABLE IF EXISTS post_content;
