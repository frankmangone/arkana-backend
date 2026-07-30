-- +goose Up
CREATE TABLE questions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid        TEXT UNIQUE NOT NULL,
    slug        TEXT UNIQUE NOT NULL,
    type        TEXT NOT NULL,
    difficulty  INTEGER NOT NULL,
    answer_key  TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE question_translations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    question_id INTEGER NOT NULL REFERENCES questions(id),
    lang        TEXT NOT NULL,
    prompt      TEXT NOT NULL,
    content     TEXT NOT NULL,
    UNIQUE (question_id, lang)
);
CREATE INDEX idx_question_translations_question_id ON question_translations(question_id);

CREATE TABLE question_tags (
    question_id INTEGER NOT NULL REFERENCES questions(id),
    tag_id      INTEGER NOT NULL REFERENCES tags(id),
    PRIMARY KEY (question_id, tag_id)
);

CREATE TABLE question_posts (
    question_id INTEGER NOT NULL REFERENCES questions(id),
    post_id     INTEGER NOT NULL REFERENCES posts(id),
    PRIMARY KEY (question_id, post_id)
);
CREATE INDEX idx_question_posts_post_id ON question_posts(post_id);

CREATE TABLE quiz_attempts (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid         TEXT UNIQUE NOT NULL,
    module_id    INTEGER NOT NULL REFERENCES reading_list_modules(id),
    user_id      INTEGER NOT NULL REFERENCES users(id),
    tier         TEXT NOT NULL DEFAULT 'standard' CHECK(tier IN ('standard', 'certificate')),
    started_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    score        INTEGER,
    passed       BOOLEAN
);
CREATE INDEX idx_quiz_attempts_user_module ON quiz_attempts(user_id, module_id);

CREATE TABLE quiz_attempt_questions (
    attempt_id  INTEGER NOT NULL REFERENCES quiz_attempts(id),
    question_id INTEGER NOT NULL REFERENCES questions(id),
    position    INTEGER NOT NULL,
    PRIMARY KEY (attempt_id, position)
);

CREATE TABLE quiz_attempt_answers (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    attempt_id  INTEGER NOT NULL REFERENCES quiz_attempts(id),
    question_id INTEGER NOT NULL REFERENCES questions(id),
    response    TEXT NOT NULL,
    correct     BOOLEAN NOT NULL,
    skipped     BOOLEAN NOT NULL DEFAULT 0,
    answered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (attempt_id, question_id)
);

-- +goose Down
DROP TABLE IF EXISTS quiz_attempt_answers;
DROP TABLE IF EXISTS quiz_attempt_questions;
DROP INDEX IF EXISTS idx_quiz_attempts_user_module;
DROP TABLE IF EXISTS quiz_attempts;
DROP INDEX IF EXISTS idx_question_posts_post_id;
DROP TABLE IF EXISTS question_posts;
DROP TABLE IF EXISTS question_tags;
DROP INDEX IF EXISTS idx_question_translations_question_id;
DROP TABLE IF EXISTS question_translations;
DROP TABLE IF EXISTS questions;
