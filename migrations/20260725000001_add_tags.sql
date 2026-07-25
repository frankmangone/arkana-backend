-- +goose Up
CREATE TABLE tags (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    slug       TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tag_translations (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    tag_id     INTEGER NOT NULL REFERENCES tags(id),
    lang       TEXT NOT NULL,
    name       TEXT NOT NULL,
    UNIQUE (tag_id, lang)
);

CREATE INDEX idx_tag_translations_tag_id ON tag_translations(tag_id);

-- +goose Down
DROP INDEX IF EXISTS idx_tag_translations_tag_id;
DROP TABLE tag_translations;
DROP TABLE tags;
