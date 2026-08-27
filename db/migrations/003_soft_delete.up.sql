-- Contrato C54: soft-delete con deleted_at (preserva historial + restore).
-- Agrega columna nullable deleted_at a posts; los read paths filtran deleted_at IS NULL.
ALTER TABLE posts ADD COLUMN deleted_at TIMESTAMP NULL;
CREATE INDEX IF NOT EXISTS idx_posts_deleted ON posts(deleted_at);
