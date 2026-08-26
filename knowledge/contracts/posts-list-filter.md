---
type: 'Task Contract'
title: 'Posts filter por author + tags (migración 002)'
description: 'Extiende el schema posts con author_id (FK users) + tabla post_tags. Amplía ListInput con AuthorID/Tag. Read path admin con filtrado; público sigue published-only.'
tags: ['ccdd', 'posts', 'filter', 'author', 'tags', 'migraciones', 'cms']

task: posts-filter-author-tag
intent: "Añadir migración 002 (author_id FK + post_tags join) y extender ListInput con AuthorID y Tag para permitir filtrar listados; read path público preserva published-only."
target: internal/posts/posts.go
signature: |
  type ListInput struct { Limit, Offset int; Status string; Query string; AuthorID int64; Tag string }
  func (s *Posts) List(in ListInput) (ListResult, error)
  func (s *Posts) ListRendered(ctx hooks.Context, in ListInput) (RenderedListResult, error)
test_command: "go test ./internal/posts/... -v"
budget:
  cyclomatic_max: 14
  nesting_max: 4
  lines_max: 100
  params_max: 3
tests: "internal/posts/posts_test.go"
tests_sha256: "396582c08367fb6e7b9ce011f6122898aed3c441facf2b20a4b9fa0d3e50d411"
touch_only: ['internal/posts/posts.go', 'knowledge/contracts/posts-list.md', 'knowledge/contracts/posts-crud.md', 'db/migrations/002_add_post_authors.up.sql', 'db/migrations/002_add_post_authors.down.sql', 'knowledge/contracts/posts-list-filter.md']
deps_allowed: ['modernc.org/sqlite', 'github.com/buke/quickjs-go']
forbids: ['network', 'subprocess', 'llm', 'unsafe']
---

# Contract: posts-filter-author-tag

## Intent
Añadir migración `002_add_post_authors` (columna nullable `author_id` FK→users + tabla join
`post_tags` + índices) y extender `ListInput` con `AuthorID`/`Tag`. El read path admin
permite filtrar por autor y/o tag; el público (`Status==""`) preserva `published`-only
(C38) **y** requiere ser published (doble filtro de seguridad).

## Interface
```go
type ListInput struct {
  Limit    int
  Offset   int
  Status   string
  Query    string
  AuthorID int64  // 0 = sin filtro
  Tag      string // "" = sin filtro
}
func (s *Posts) List(in ListInput) (ListResult, error)
func (s *Posts) ListRendered(ctx hooks.Context, in ListInput) (RenderedListResult, error)
```

## Invariants
- `author_id` es nullable (C40): posts sin author son válidos.
- Read path público (`Status==""`) filtra a `published` **y** respeta `author_id`/`tag`.
- `post_tags` es (post_id, tag) PK con ON DELETE CASCADE.
- `Create` inserta `author_id` sólo si `AuthorID > 0`.

## Examples
- `List(ListInput{AuthorID:1})` → sólo posts de ese autor.
- `List(ListInput{Tag:"go"})` → posts con el tag.
- `List(ListInput{AuthorID:1, Tag:"go", Query:"CMS"})` → AND combinado.

## Do / Don't
- DO: usar `EXISTS (SELECT 1 FROM post_tags …)` para el filtro por tag (sin JOIN en el
  SELECT, preserva `SELECT … FROM posts`).
- DO: migrar `author_id` como nullable (no romper `Create` existente).
- DON'T: filtrar público por author/tag si Status!="" (eso sería admin, no público).

## Tests
(oráculo RE-SELLADO `internal/posts/posts_test.go`, SHA256 `396582c0…`. Tests C40:
`TestList_FilterByAuthor`, `TestList_FilterByTag`, `TestList_CombinedFilters`.)

## Constraints
- Tocar SOLO: `internal/posts/posts.go`, `internal/posts/posts_test.go` (re-sellado),
  `db/migrations/002_*`, contratos compartidos (`posts-crud.md`, `posts-list.md`,
  `posts-list-filter.md`), reportes C36-C39 + C40.
- No tocar: `internal/hooks/*`, `internal/db/*` (excepto migraciones via `ApplyMigrations`),
  `db/migrations/001_*`.
- PARAR y reportar si: migración 002 no aplica sobre SQLite fresh, read path público filtra
  mal por author/tag, o `Create` rompe tests C36 existentes.
