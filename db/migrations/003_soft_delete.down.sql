-- Contrato C54: rollback de soft-delete (003).
-- mattn/go-sqlite3 v1.14.50 incluye SQLite >=3.35 (ALTER TABLE DROP COLUMN soportado).
-- Importante: drop INDEX antes de DROP COLUMN (el índice referencia deleted_at).
DROP INDEX IF EXISTS idx_posts_deleted;
ALTER TABLE posts DROP COLUMN deleted_at;
