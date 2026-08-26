CREATE TABLE posts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    slug        TEXT NOT NULL UNIQUE,
    title       TEXT NOT NULL,
    content     TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    created_at  TIMESTAMP NOT NULL DEFAULT (datetime('now')),
    updated_at  TIMESTAMP NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE users (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    email       TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name TEXT,
    created_at  TIMESTAMP NOT NULL DEFAULT (datetime('now')),
    updated_at  TIMESTAMP NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id),
    expires_at  TIMESTAMP NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE options (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL
);
