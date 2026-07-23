-- +goose Up
CREATE TABLE subscribers (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER REFERENCES users(id),
    email           TEXT NOT NULL UNIQUE,
    status          TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'confirmed', 'unsubscribed')),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    confirmed_at    TIMESTAMP,
    unsubscribed_at TIMESTAMP
);
CREATE INDEX idx_subscribers_email ON subscribers(email);
CREATE INDEX idx_subscribers_user_id ON subscribers(user_id);
CREATE INDEX idx_subscribers_status ON subscribers(status);

-- +goose Down
DROP INDEX IF EXISTS idx_subscribers_status;
DROP INDEX IF EXISTS idx_subscribers_user_id;
DROP INDEX IF EXISTS idx_subscribers_email;
DROP TABLE IF EXISTS subscribers;
