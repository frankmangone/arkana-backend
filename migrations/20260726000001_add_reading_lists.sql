-- +goose Up
CREATE TABLE reading_lists (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    slug        TEXT UNIQUE NOT NULL,
    cover_image TEXT,
    ongoing     BOOLEAN NOT NULL DEFAULT 0,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE reading_list_translations (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    reading_list_id INTEGER NOT NULL REFERENCES reading_lists(id),
    lang            TEXT NOT NULL,
    title           TEXT NOT NULL,
    description     TEXT NOT NULL,
    UNIQUE (reading_list_id, lang)
);
CREATE INDEX idx_reading_list_translations_list_id ON reading_list_translations(reading_list_id);

CREATE TABLE reading_list_modules (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    reading_list_id INTEGER NOT NULL REFERENCES reading_lists(id),
    slug            TEXT NOT NULL,
    position        INTEGER NOT NULL,
    UNIQUE (reading_list_id, slug)
);
CREATE INDEX idx_reading_list_modules_list_id ON reading_list_modules(reading_list_id);

CREATE TABLE reading_list_module_translations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    module_id   INTEGER NOT NULL REFERENCES reading_list_modules(id),
    lang        TEXT NOT NULL,
    title       TEXT NOT NULL,
    description TEXT NOT NULL,
    UNIQUE (module_id, lang)
);
CREATE INDEX idx_reading_list_module_translations_module_id ON reading_list_module_translations(module_id);

CREATE TABLE reading_list_items (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    module_id INTEGER NOT NULL REFERENCES reading_list_modules(id),
    slug      TEXT NOT NULL,
    post_path TEXT NOT NULL,
    position  INTEGER NOT NULL,
    UNIQUE (module_id, slug)
);
CREATE INDEX idx_reading_list_items_module_id ON reading_list_items(module_id);
CREATE INDEX idx_reading_list_items_post_path ON reading_list_items(post_path);

-- +goose Down
DROP INDEX IF EXISTS idx_reading_list_items_post_path;
DROP INDEX IF EXISTS idx_reading_list_items_module_id;
DROP TABLE IF EXISTS reading_list_items;
DROP INDEX IF EXISTS idx_reading_list_module_translations_module_id;
DROP TABLE IF EXISTS reading_list_module_translations;
DROP INDEX IF EXISTS idx_reading_list_modules_list_id;
DROP TABLE IF EXISTS reading_list_modules;
DROP INDEX IF EXISTS idx_reading_list_translations_list_id;
DROP TABLE IF EXISTS reading_list_translations;
DROP TABLE IF EXISTS reading_lists;
