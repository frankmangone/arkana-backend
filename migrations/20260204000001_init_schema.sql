-- +goose Up
CREATE TABLE wallets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    address TEXT UNIQUE NOT NULL,
    system TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_wallets_address ON wallets(address);

CREATE TABLE posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path_identifier TEXT UNIQUE NOT NULL,
    like_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_posts_path ON posts(path_identifier);

CREATE TABLE post_likes (
    post_id INTEGER NOT NULL,
    wallet_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (post_id, wallet_id),
    FOREIGN KEY (post_id) REFERENCES posts(id),
    FOREIGN KEY (wallet_id) REFERENCES wallets(id)
);

CREATE TABLE comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id INTEGER NOT NULL,
    wallet_id INTEGER NOT NULL,
    parent_id INTEGER,
    body TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (post_id) REFERENCES posts(id),
    FOREIGN KEY (wallet_id) REFERENCES wallets(id),
    FOREIGN KEY (parent_id) REFERENCES comments(id)
);
CREATE INDEX idx_comments_post ON comments(post_id);

-- +goose Down
DROP INDEX IF EXISTS idx_wallets_address;
DROP TABLE IF EXISTS wallets;
DROP INDEX IF EXISTS idx_comments_post;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS post_likes;
DROP INDEX IF EXISTS idx_posts_path;
DROP TABLE IF EXISTS posts;
