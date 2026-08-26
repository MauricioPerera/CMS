---
type: 'Task Contract'
title: 'Posts CRUD'
description: 'CRUD de posts (CREATE, READ, UPDATE, PUBLISH) sobre el schema baseline de C1, con integración del hook point post.validate (QuickJS) antes de cada INSERT/UPDATE.'
tags: ['ccdd', 'posts', 'crud', 'sqlite', 'hooks', 'cms']

task: posts-crud
intent: "Proveer operaciones CRUD para posts sobre SQLite (modernc.org/sqlite) integrando el hook point post.validate del runtime QuickJS (C2) antes de cada escritura."
target: internal/posts/posts.go
signature: |
  type Post struct { ID int64; Slug, Title, Content, Status string; CreatedAt, UpdatedAt time.Time }
  func NewPosts(dbh *sql.DB, rt *hooks.Runtime, reg *hooks.Registry) *Posts
  func (s *Posts) Create(ctx hooks.Context, in CreateInput) (Post, error)
  func (s *Posts) Update(ctx hooks.Context, in UpdateInput) (Post, error)
  func (s *Posts) Publish(ctx hooks.Context, id int64) (Post, error)
  func (s *Posts) Get(id int64) (Post, error)
  func (s *Posts) GetBySlug(slug string) (Post, error)
test_command: "go test ./internal/posts/... -v"
budget:
  cyclomatic_max: 12
  nesting_max: 4
  lines_max: 120
  params_max: 4
tests: "internal/posts/posts_test.go"
tests_sha256: "e6e09e40835ab86b41b76a9371e608c0b78617be36f4292ea5ee43fd053f9375"
touch_only: ['internal/posts/posts.go']
deps_allowed: ['modernc.org/sqlite', 'github.com/buke/quickjs-go']
forbids: ['network', 'subprocess', 'llm', 'unsafe']
---

# Contract: posts-crud

## Intent
CRUD de posts sobre el schema baseline (C1 — tabla `posts` con `id`, `slug`, `title`,
`content`, `status` CHECK en draft/published/archived, timestamps). Antes de cada
`INSERT`/`UPDATE`, se ejecuta el hook `post.validate` (C2 — `hooks.Runtime`/`Registry`)
que puede rechazar la operación con `{ ok: false, error: "..." }`.

## Interface
```go
type Post struct {
  ID        int64
  Slug      string
  Title     string
  Content   string
  Status    string    // 'draft' | 'published' | 'archived'
  CreatedAt time.Time
  UpdatedAt time.Time
}

type CreateInput struct {
  Slug    string
  Title   string
  Content string
  AuthorID int64
}
type UpdateInput struct {
  ID      int64
  Title   string
  Content string
  Status  string
}

type Posts struct { dbh *sql.DB; rt *hooks.Runtime; reg *hooks.Registry }
func NewPosts(dbh *sql.DB, rt *hooks.Runtime, reg *hooks.Registry) *Posts
func (s *Posts) Create(ctx hooks.Context, in CreateInput) (Post, error)
func (s *Posts) Update(ctx hooks.Context, in UpdateInput) (Post, error)
func (s *Posts) Publish(ctx hooks.Context, id int64) (Post, error)
func (s *Posts) Get(id int64) (Post, error)
func (s *Posts) GetBySlug(slug string) (Post, error)
```

## Invariants
- `post.validate` se ejecuta ANTES de cada escritura (Create/Update/Publish). Si el hook
  retorna `{ ok: false, error }` → la operación aborta con ese error (no se escribe).
- `Create` requiere `Slug`, `Title`, `Content` no vacíos; `Slug` único (UNIQUE).
- `Update` solo modifica `title`/`content`; `Status` se cambia solo vía `Publish`.
- `Publish` setea `status='published'` y ejecuta `post.validate`.
- `Get`/`GetBySlug` son read-only (NO disparan hook).
- Timestamps `created_at`/`updated_at` gestionados por el DB (DEFAULT `datetime('now')`).
- Errores wrappeados con `%w` para trazabilidad.

## Examples
- `NewPosts(db, rt, reg).Create(ctx, {Slug:"hola", Title:"Hola", Content:"..."})` → Post con
  `Status="draft"`, `ID` autoincrement.
- Si `post.validate` retorna `{ok:false, error:"titulo muy corto"}` → `Create` falla sin
  insertar.
- `Publish(ctx, 1)` → `Status="published"`.
- `Get(1)` → Post con `CreatedAt` parseado como `time.Time`.

## Do / Don't
- DO: usar `modernc.org/sqlite` (puro Go) y el DB de C1 (migrado con `db.ApplyMigrations`).
- DO: ejecutar `post.validate` vía `reg.Call("post.validate", payload)` donde
  `payload = { action, post }` y `ctx.Point = "post.validate"`.
- DO: wrappear errores con `%w`.
- DON'T: tocar `internal/posts/posts_test.go` (oráculo congelado).
- DON'T: tocar `db/migrations/`, `go.mod`, nodos OKF existentes, contratos C1/C2.

## Tests
(oráculo congelado SHA256 `…` en `internal/posts/posts_test.go`. El PM lo autoriza antes de
delegar; el implementador no lo toca ni modifica.)

## Constraints
- PARAR y reportar si: `modernc.org/sqlite` no compila, o `post.validate` no se ejecuta
  antes de INSERT, o un hook que rechaza no aborta la operación.
- CGO_ENABLED=1 (heredado de C2 por QuickJS).
