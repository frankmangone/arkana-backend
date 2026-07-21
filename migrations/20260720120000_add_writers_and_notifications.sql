-- +goose Up
CREATE TABLE writers (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    user_id    INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_writers_user_id ON writers(user_id);

ALTER TABLE posts ADD COLUMN writer_id INTEGER REFERENCES writers(id);
CREATE INDEX idx_posts_writer_id ON posts(writer_id);

CREATE TABLE notifications (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    recipient_user_id INTEGER NOT NULL REFERENCES users(id),
    actor_user_id     INTEGER NOT NULL REFERENCES users(id),
    type              TEXT NOT NULL CHECK(type IN ('comment_reply', 'post_liked', 'post_commented')),
    post_id           INTEGER REFERENCES posts(id),
    comment_id        INTEGER REFERENCES comments(id),
    read_at           TIMESTAMP,
    created_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_notifications_recipient ON notifications(recipient_user_id, read_at);

-- +goose Down
DROP INDEX IF EXISTS idx_notifications_recipient;
DROP TABLE IF EXISTS notifications;
DROP INDEX IF EXISTS idx_posts_writer_id;
ALTER TABLE posts DROP COLUMN writer_id;
DROP INDEX IF EXISTS idx_writers_user_id;
DROP TABLE IF EXISTS writers;
