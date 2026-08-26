# Contrato 40 — Posts filter por author + tags

Prerrequisitos: C1 (schema baseline), C2 (hooks), C36-C39 (posts CRUD/render/list/sanitize).

> Capa: contrato de ejecución. Task contract CCDD: `knowledge/contracts/posts-list-filter.md`.

## T1-MIGRACION-002

- `db/migrations/002_add_post_authors.up.sql`: `ALTER TABLE posts ADD COLUMN author_id` +
  índice + tabla `post_tags` (PK post_id+tag, ON DELETE CASCADE) + índice.
- `db/migrations/002_add_post_authors.down.sql`: DROP en orden inverso + restore de tabla
  posts (SQLite <3.35 no soporta DROP COLUMN).

## T1-LIST-FILTERS

- `internal/posts/posts.go`: `ListInput` extiende con `AuthorID int64` y `Tag string`.
- `buildListQuery`: añade `author_id = ?` (si >0) y `EXISTS (SELECT 1 FROM post_tags …)`.
- `Create`: inserta `author_id` condicional (`AuthorID > 0`).

## T1-RESELLO-ORACULO

SHA posts_test.go → `396582c0…` (20 tests previos + 3 C40 = 23). Actualizado en
`posts-crud.md`, `posts-render.md`, `posts-list.md`, `posts-list-filter.md`.

## T1-TESTS

- `TestList_FilterByAuthor`: 2 posts distinto author → filtro author_id.
- `TestList_FilterByTag`: post con tags "go","cms" → filtro "go" y filtro inexistente.
- `TestList_CombinedFilters`: AND de author+tag+query.

## Criterios de aceptación

- [ ] `python scripts/validate_contracts.py knowledge/contracts` exit 0 (40 archivo(s)).
- [ ] `go build ./...` exit 0.
- [ ] `go vet ./...` limpio.
- [ ] `go test ./internal/... -v` → 5/5 db + 12/12 hooks + 23/23 posts = **40/40 OK**.
- [ ] `python scripts/preflight.py` → 19/19.
- [ ] `python -m unittest discover -s tests -p "test_*.py"` → 744/744.

## Restricciones

### Tocar SOLO
- `internal/posts/posts.go`, `internal/posts/posts_test.go` (re-sellado),
  `db/migrations/002_*`, `knowledge/contracts/{posts-crud,posts-render,posts-list,
  posts-list-filter}.md` (tests_sha256), `docs/reports/CONTRACT-40-REPORT.md`, `CHANGELOG.md`.

- No tocar: `internal/hooks/*`, `internal/db/*`, `db/migrations/001_*`, specs/contratos C1-C39.
- ABORTAR SI: migración 002 falla, read path público filtra mal, o tests C36/C37 rotos.
