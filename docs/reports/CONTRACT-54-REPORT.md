# Contract 54 — Soft-delete (deleted_at migration + Restore)

**Estado:** ✅ VERDE (draft → validated → implemented → verified)
**Fecha:** 2026-08-27
**Ciclo KDD:** Extiende C1 (db-migrations) + C51 (Delete) + C43 (HTTP). Capa 3 (migration tests) → Nivel 1 (test_command `go test ./internal/db/... -run TestMigrations`).

## Contexto

C51 implementó `DELETE /posts/{id}` como **hard-delete**. C54 lo evoluciona a **soft-delete** (`deleted_at`) para preservar historial, permitir `Restore`, y evitar borrados accidentales que rompan referencias. Requiere migración reversible + read path que filtre borrados.

## Migración (Capa 1 — `db/migrations/`)

- `003_soft_delete.up.sql`: `ALTER TABLE posts ADD COLUMN deleted_at TIMESTAMP NULL` + `CREATE INDEX idx_posts_deleted`.
- `003_soft_delete.down.sql`: `DROP INDEX idx_posts_deleted` **antes** de `ALTER TABLE ... DROP COLUMN deleted_at` (el índice referencia la columna; orden inverso evita `no such column`).
  - mattn/go-sqlite3 v1.14.50 incluye SQLite ≥3.35 → `ALTER TABLE DROP COLUMN` soportado.

## Store (Capa 1 — `internal/posts/posts.go`)

```go
// Delete → soft-delete: UPDATE posts SET deleted_at = datetime('now') WHERE id = ? AND deleted_at IS NULL.
func (s *Posts) Delete(ctx hooks.Context, id int64) error

// Restore: UPDATE posts SET deleted_at = NULL WHERE id = ? (verifica deleted_at IS NOT NULL).
func (s *Posts) Restore(ctx hooks.Context, id int64) (Post, error)
```

- Ambos disparan `post.validate` (action:"delete"/"restore") — invariante C36.
- Read path filtra `deleted_at IS NULL`:
  - `queryByID` (Get/GetBySlug): `WHERE id = ? AND deleted_at IS NULL`.
  - `getBySlug` (render.go): `WHERE slug = ? AND deleted_at IS NULL`.
  - `buildListQuery`: agrega `deleted_at IS NULL` al WHERE (List/ListRendered excluirán borrados).

## HTTP (Capa 1 — `internal/posts/http.go`)

- `PATCH /posts/{id}` ya registrado (C53).
- `POST /posts/{id}/restore` → `h.AuthRequired(h.Restore)` — 200 (restaurado) / 404 (no existe o no borrado).
- Logging slog `posts.restore.ok/error/bad_id` + Metrics + span (C41).

## Tests (Capa 3)

`internal/posts/http_test.go` (+3):
- `TestHandler_Delete_ThenRestore`: soft-delete (204) → Get/GetBySlug fallan (filter) → Restore (200) → Get OK.
- `TestHandler_Restore_NotFound`: restore id inexistente → 404.
- `TestHandler_Restore_NotDeleted`: restore post activo → 404 (lookup filtra `deleted_at IS NOT NULL`).

Oráculo C1 (`internal/db/migrations_test.go` SHA `f1bf4aac…`) **preservado** — no se tocó; los 5 tests `TestMigrations_*` PASS con la migración 003.

## Verification

```
go test -race -count=1 ./internal/db/... -run TestMigrations -v
→ PASS: TestMigrations_CreateTables/PostsSchema/UserEmailUnique/PostStatusCheck/RollbackDropsTables (5/5)

go test -race -count=1 ./internal/... -timeout 120s
→ ok  gopress/internal/db    (5/5 migraciones, rollback OK)
→ ok  gopress/internal/hooks  (14 tests, 0 races)
→ ok  gopress/internal/posts  (40 posts tests + 2 C50 parallel, 0 races)

python scripts/validate_contracts.py knowledge/contracts
→ OK: todos los contratos son validos (42 contracts; posts-http-api.md SHA 32a5c428…)
```

## Resultado

- Soft-delete completo: Delete → UPDATE deleted_at; Restore → NULL.
- Read path excluye borrados (queryByID/getBySlug/List/ListRendered/Rendered).
- Migración 003 reversible (rollback validado por oráculo C1 congelado).
- `posts-http-api.md` re-sellado: SHA `32a5c428…`.
- Oráculos C1/C2/C36 preservados.

## Gate

- `validate_contracts.py`: PASS (42).
- `validate_specs.py`: PASS (44).
- `preflight.py`: 19/19.
- `validate_observability_findings.py`: 0 findings.
- `-race`: 0 data races.
- KDD suite: 744/744 OK.
