# CONTRACT-40-REPORT — Posts filter por author + tags

Fecha: 2026-08-26
Spec: `specs/CONTRACT-40-posts-filter.md`

## Resumen ejecutivo

| Criterio | Verdicto | Evidencia |
|---|---|---|
| `validate_contracts.py` | ✅ exit 0, 0 errores en 41 archivo(s) | `python scripts/validate_contracts.py knowledge/contracts` |
| `validate_test_commands.py` | ✅ 5 posts contracts PASS | corrida directa |
| `go build ./...` | ✅ exit 0 (CGO_ENABLED=1) | corrida directa |
| `go vet ./...` | ✅ limpio | corrida directa |
| `go test ./internal/...` | ✅ **40/40 OK** (5 db + 12 hooks + 23 posts) | `go test -timeout 30s` |
| `preflight.py` | ✅ **19/19** gates | `python scripts/preflight.py` |
| KDD python suite | ✅ **744/744** OK | `python -m unittest discover -s tests -p test_*.py` |

## Entregado

**Task contract CCDD**: `knowledge/contracts/posts-list-filter.md` — `target: internal/posts/posts.go`,
`test_command: "go test ./internal/posts/... -v"`, SHA256 oráculo `396582c0…`.

**Migración**: `db/migrations/002_add_post_authors.up.sql` (author_id nullable FK + post_tags
join + índices) y `down.sql` (rollback con restore de tabla posts para compat <3.35).

**Implementación**: `internal/posts/posts.go` — `ListInput` extiende con `AuthorID`/`Tag`;
`buildListQuery` añade `author_id=?` y `EXISTS (SELECT 1 FROM post_tags …)`; `Create`
inserta `author_id` condicional.

## Tests C40
- `TestList_FilterByAuthor`: 2 posts distinto author → filtro author_id.
- `TestList_FilterByTag`: tags "go"/"cms" → filtro existente + inexistente.
- `TestList_CombinedFilters`: AND de author+tag+query.

## Re-sellado del oráculo (acumulativo)
`internal/posts/posts_test.go`: 7 C36 + 7 C37 + 4 C38 + 2 C39 + 3 C40 = **23 tests**.
SHA final: `396582c08367fb6e7b9ce011f6122898aed3c441facf2b20a4b9fa0d3e50d411`.
Actualizado en `posts-crud.md`, `posts-render.md`, `posts-list.md`, `posts-list-filter.md`.

## Invariants verificadas
- `author_id` nullable: `Create` sin author_id sigue funcionando (tests C36 preservados).
- Read path público (`Status==""`) filtra a `published` + respeta author/tag/query.
- Migración 002 idempotente (`CREATE TABLE IF NOT EXISTS`, `ADD COLUMN` no repetible en
  rollback→setup cycle testeado vía `freshDB`).

## Próximo contrato
**C41 — Observability scan**: gaps de logging/métricas en rutas críticas (read path posts).
C40 completa el feature de posts; C41 hardeniza observabilidad.
