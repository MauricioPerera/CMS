---
type: 'Task Contract'
title: 'Posts list/search (paginado + filtro)'
description: 'Listado paginado de posts (LIMIT/OFFSET) con search por slug/title y filtro por status. El read path público filtra a published; post.render opcional sobre cada item.'
tags: ['ccdd', 'posts', 'list', 'search', 'paginado', 'cms']

task: posts-list
intent: "Proveer un listado paginado y buscador de posts con filtro por status, reutilizando el read path de C37 (render opcional por item)."
target: internal/posts/posts.go
signature: |
  type ListInput struct { Limit, Offset int; Status string; Query string }
  type ListResult struct { Posts []Post; Total int; HasMore bool }
  type RenderedListResult struct { Items []RenderedPost; Total int; HasMore bool }
  func (s *Posts) List(in ListInput) (ListResult, error)
  func (s *Posts) ListRendered(ctx hooks.Context, in ListInput) (RenderedListResult, error)
test_command: "go test ./internal/posts/... -v"
budget:
  cyclomatic_max: 12
  nesting_max: 4
  lines_max: 90
  params_max: 3
tests: "internal/posts/posts_test.go"
tests_sha256: "396582c08367fb6e7b9ce011f6122898aed3c441facf2b20a4b9fa0d3e50d411"
touch_only: ['internal/posts/posts.go']
deps_allowed: ['modernc.org/sqlite', 'github.com/buke/quickjs-go']
forbids: ['network', 'subprocess', 'llm', 'unsafe']
---

# Contract: posts-list

## Intent
Listado paginado (`LIMIT/OFFSET`) de posts con search por `slug`/`title` (LIKE) y filtro
por `status`. El read path "público" por defecto filtra a `published` (no sirve drafts/
archived). `ListRendered` aplica el chain `post.render`→`content.filter` (C37) sobre cada
post del page.

## Interface
```go
type ListInput struct {
  Limit  int
  Offset int
  Status string // "" = published (público); "draft"/"archived" para admin.
  Query  string // search en slug/title (LIKE).
}
type ListResult struct {
  Items   []Post
  Total   int  // count total (sin paginar).
  HasMore bool // si hay más páginas.
}
type RenderedListResult struct {
  Items   []RenderedPost
  Total   int
  HasMore bool
}
func (s *Posts) List(in ListInput) (ListResult, error)
func (s *Posts) ListRendered(ctx hooks.Context, in ListInput) (RenderedListResult, error)
```

## Invariants
- `ListInput.Status == ""` → filtra a `status='published'` (read path público); cualquier
  otro valor filtra exactamente por ese status (admin).
- `Limit` clamped a `[1, 100]` (default 20). `Offset >= 0`.
- `Query` (non-empty) → `WHERE slug LIKE %q% OR title LIKE %q%`.
- `ListRendered` ejecuta `post.render`/`content.filter` (C37) por cada item; un hook que
  rechaza un item → se propaga error y aborta el listado.
- `Total` = count sin OFFSET/LIMIT; `HasMore = (offset + len(items)) < Total`.

## Examples
- `List(ListInput{Limit:10, Offset:0, Status:""})` → solo `published`, 10 items, `HasMore`.
- `List(ListInput{Query:"ola", Status:"draft"})` → drafts cuyo slug o title contienen "ola".
- `ListRendered(ctx, ListInput{Limit:5})` → 5 `RenderedPost` con `HTML` renderizado.

## Do / Don't
- DO: clamp `Limit`; usar `LIKE` con escaping mínico (el `_`/`%` del query se tratan literal
  en la app — el search es simple, no full-text).
- DO: count total en query separado para `HasMore`/`Total`.
- DO: wrappear errores con `%w`.
- DON'T: hardcodear `status='published'` como único path (debe permitir filtrar por otro
  status para admin).

## Tests
(oráculo RE-SELLADO en `internal/posts/posts_test.go`; SHA256 `…`. Re-sellado de C36/C37
con tests C38: `TestList_Empty`, `TestList_PublishedOnly`, `TestList_Pagination`,
`TestList_Search`, `TestListRendered_AppliesHooks`.)

## Restricciones
- Tocar SOLO: `internal/posts/posts.go`, `internal/posts/posts_test.go` (re-sellado),
  `knowledge/contracts/posts-crud.md` (actualizar `tests_sha256`).
- No tocar: `internal/hooks/*`, `internal/db/*`, `db/migrations/*`, contratos/specs C1-C37
  salvo re-sellado documentado.
- CGO_ENABLED=1 (heredado de C2).
- ABORTAR SI: el read path público filtra mal (devuelve drafts), o `Limit` clamp falla, o
  `ListRendered` no aplica `post.render` por item.

## Constraints
- PARAR y reportar si: `List` retorna posts con `status != 'published'` cuando `Status==""`,
  o `HasMore`/`Total` inconsistentes, o `ListRendered` no ejecuta hooks por item.
