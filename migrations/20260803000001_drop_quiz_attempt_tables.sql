-- +goose Up
DROP TABLE IF EXISTS quiz_attempt_answers;
DROP TABLE IF EXISTS quiz_attempt_questions;
DROP INDEX IF EXISTS idx_quiz_attempts_user_module;
DROP TABLE IF EXISTS quiz_attempts;

-- +goose Down
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
