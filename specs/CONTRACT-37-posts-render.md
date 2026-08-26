# Contrato 37 — Posts render (Markdown→HTML + hooks)

Prerrequisitos: C1 (schema), C2 (hooks), C36 (posts CRUD). El `post.render` y
`content.filter` hook points están definidos en `knowledge/data_models/hook_points.md`.
Este contrato evoluciona el read path de `internal/posts/`.

> Capa: contrato de ejecución. Task contract CCDD: `knowledge/contracts/posts-render.md`
> (validado por `scripts/validate_contracts.py`). NOTE: re-sella el oráculo de C36
> (`tests_sha256` en `posts-crud.md`) — cambio de scope justificado en el reporte C36.

## T1-POSTS-RENDER — read path con render + hooks

OBJETIVO: `internal/posts/render.go` — `GetRendered(id)` / `GetBySlugRendered(slug)`
que retornan `RenderedPost` con `HTML` (Markdown→HTML). Cadena de hooks:
1. `post.render` — payload `{ action:"render", post:{ id, slug, title, content, status } }`,
   retorno esperado `{ html: "<p>...</p>" }`. Si no hay hook → fallback `renderMarkdown`.
2. `content.filter` — payload `{ html: "..." }`, retorno `{ html: "<sanitized>" }`.
   Si no hay hook → HTML sin mutar.

## T1-RESELLO-ORACULO-C36

El oráculo `internal/posts/posts_test.go` (de C36) se RE-SELLA con tests nuevos para
C37 (`TestGetRendered_WithHook`, `TestGetRendered_FallbackMarkdown`, `TestContentFilterApplied`,
`TestGet_StillReturnsRawContent` para garantizar backward-compat). El SHA viejo
(`443abba5…`, C36) se actualiza en `knowledge/contracts/posts-crud.md` → `tests_sha256`
nuevo, y el reporte C36 documenta el re-sellado.

## T1-TESTS — oráculo congelado (re-sellado)

Verifica:
- `GetRendered` con hook `post.render` → `HTML` retornado por el hook.
- `GetRendered` sin hooks → `renderMarkdown("# Hola")` = `<h1>Hola</h1>`.
- `content.filter` aplicado después de `post.render` → HTML sanitizado/mutado.
- `Get`/`GetBySlug` (C36) siguen retornando `Post.Content` raw (backward-compat).
- `post.render` rechazando → `GetRendered` propaga error.

## Criterios de aceptación

- [ ] `python scripts/validate_contracts.py knowledge/contracts` exit 0.
- [ ] `python -m unittest discover -s tests -p "test_*.py"` verde (744/744).
- [ ] `go build ./...` exit 0.
- [ ] `go vet ./...` limpio.
- [ ] `go test ./internal/posts/... -v` → N/N OK.
- [ ] `python scripts/preflight.py` → 19/19.

## Restricciones

### Tocar SOLO

- `internal/posts/render.go` (nuevo — render + hooks de read path).
- `internal/posts/posts.go` (añade `GetRendered`/`GetBySlugRendered`/`RenderedPost`).
- `internal/posts/posts_test.go` (re-sellado con tests C37).
- `knowledge/contracts/posts-crud.md` (actualizar `tests_sha256`).
- `docs/reports/CONTRACT-36-REPORT.md` (documentar re-sellado).
- `knowledge/data_models/posts_render.md` (nuevo OKF node).

- No tocar: `internal/hooks/*`, `internal/db/*`, `db/migrations/*`, specs/contracts C1-C36
  (salvo re-sellado documentado).
- `deps_allowed`: `github.com/buke/quickjs-go` (heredado), stdlib Go (`regexp`, `strings`).
- CGO_ENABLED=1 (heredado de C2).

### Tocar SOLO (C37)
- Ver lista completa arriba.

- ABORTAR SI: render de fallback falla en sintaxis mínima; o `post.render` no se ejecuta
  antes de `content.filter`; o un hook rechazado no propaga error.

## Backlinks OKF

- Hook points: `knowledge/data_models/hook_points.md` (`post.render`, `content.filter`).
- Data model posts CRUD: `knowledge/data_models/posts_crud.md`.
- Data model posts render: `knowledge/data_models/posts_render.md` (nuevo).
- Spec C36: `specs/CONTRACT-36-posts-crud.md`.
