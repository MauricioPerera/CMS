-- Contrato C40: rollback de 002_add_post_authors.
DROP INDEX IF EXISTS idx_post_tags_tag;
DROP TABLE IF EXISTS post_tags;
DROP INDEX IF EXISTS idx_posts_author;
-- SQLite no soporta DROP COLUMN de forma portátil pre-3.35; revertimos author_id
-- recreando la tabla (compat con older SQLite).
CREATE TABLE posts_compat (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    slug        TEXT NOT NULL UNIQUE,
    title       TEXT NOT NULL,
    content     TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    created_at  TIMESTAMP NOT NULL DEFAULT (datetime('now')),
    updated_at  TIMESTAMP NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO posts_compat (id, slug, title, content, status, created_at, updated_at)
    SELECT id, slug, title, content, status, created_at, updated_at FROM posts;
DROP TABLE posts;
ALTER TABLE posts_compat RENAME TO posts;
