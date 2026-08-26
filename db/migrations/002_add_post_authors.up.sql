-- Contrato C40: posts filter por author + tags.
ALTER TABLE posts ADD COLUMN author_id INTEGER REFERENCES users(id);
CREATE INDEX IF NOT EXISTS idx_posts_author ON posts(author_id);
CREATE TABLE IF NOT EXISTS post_tags (
    post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    tag     TEXT    NOT NULL,
    PRIMARY KEY (post_id, tag)
);
CREATE INDEX IF NOT EXISTS idx_post_tags_tag ON post_tags(tag);
