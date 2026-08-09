-- +goose Up
CREATE TABLE question_flags (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    question_id INTEGER NOT NULL REFERENCES questions(id),
    user_id     INTEGER NOT NULL REFERENCES users(id),
    reason      TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (question_id, user_id)
);
CREATE INDEX idx_question_flags_question_id ON question_flags(question_id);

-- +goose Down
DROP INDEX IF EXISTS idx_question_flags_question_id;
DROP TABLE question_flags;
