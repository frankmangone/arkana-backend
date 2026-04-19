-- +goose Up

-- Drop wallet-dependent tables in dependency order
DROP TABLE IF EXISTS post_likes;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS wallets;

-- Users table with OIDC provider support and optional wallet association
CREATE TABLE users (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    email            TEXT UNIQUE NOT NULL,
    username         TEXT,
    avatar_url       TEXT,
    auth_provider    TEXT NOT NULL CHECK(auth_provider IN ('google', 'github', 'apple', 'discord')),
    provider_user_id TEXT NOT NULL,
    email_verified   INTEGER NOT NULL DEFAULT 0,
    wallet_address   TEXT,
    wallet_system    TEXT,
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(auth_provider, provider_user_id)
);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_provider ON users(auth_provider, provider_user_id);

-- Refresh tokens for JWT rotation
CREATE TABLE refresh_tokens (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens(token_hash);

-- Recreate post_likes with user_id
CREATE TABLE post_likes (
    post_id    INTEGER NOT NULL,
    user_id    INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (post_id, user_id),
    FOREIGN KEY (post_id)  REFERENCES posts(id),
    FOREIGN KEY (user_id)  REFERENCES users(id)
);

-- Recreate comments with user_id
CREATE TABLE comments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id    INTEGER NOT NULL,
    user_id    INTEGER NOT NULL,
    parent_id  INTEGER,
    body       TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (post_id)   REFERENCES posts(id),
    FOREIGN KEY (user_id)   REFERENCES users(id),
    FOREIGN KEY (parent_id) REFERENCES comments(id)
);
CREATE INDEX idx_comments_post ON comments(post_id);

-- +goose Down

DROP INDEX IF EXISTS idx_comments_post;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS post_likes;
DROP INDEX IF EXISTS idx_refresh_tokens_hash;
DROP TABLE IF EXISTS refresh_tokens;
DROP INDEX IF EXISTS idx_users_provider;
DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;

CREATE TABLE wallets (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    address    TEXT UNIQUE NOT NULL,
    system     TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_wallets_address ON wallets(address);

CREATE TABLE post_likes (
    post_id    INTEGER NOT NULL,
    wallet_id  INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (post_id, wallet_id),
    FOREIGN KEY (post_id)   REFERENCES posts(id),
    FOREIGN KEY (wallet_id) REFERENCES wallets(id)
);

CREATE TABLE comments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id    INTEGER NOT NULL,
    wallet_id  INTEGER NOT NULL,
    parent_id  INTEGER,
    body       TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (post_id)   REFERENCES posts(id),
    FOREIGN KEY (wallet_id) REFERENCES wallets(id),
    FOREIGN KEY (parent_id) REFERENCES comments(id)
);
CREATE INDEX idx_comments_post ON comments(post_id);
