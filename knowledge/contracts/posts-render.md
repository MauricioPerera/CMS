---
type: 'Task Contract'
title: 'Posts render (Markdown→HTML + hooks)'
description: 'Read path de posts con render Markdown→HTML via hook post.render, seguido de content.filter para sanitizar. Evolución del read path de C36.'
tags: ['ccdd', 'posts', 'render', 'markdown', 'hooks', 'cms', 'resellado']

task: posts-render
intent: "Proveer un read path de posts que convierta el contenido Markdown a HTML via hook post.render, luego aplye content.filter para sanitizar, preservando el Content raw como fallback cuando no hay hooks."
target: internal/posts/render.go
signature: |
  type RenderedPost struct { Post; HTML string }
  func (s *Posts) GetRendered(ctx hooks.Context, id int64) (RenderedPost, error)
  func (s *Posts) GetBySlugRendered(ctx hooks.Context, slug string) (RenderedPost, error)
  func renderMarkdown(content string) string
test_command: "go test ./internal/posts/... -v"
budget:
  cyclomatic_max: 10
  nesting_max: 4
  lines_max: 100
  params_max: 4
tests: "internal/posts/posts_test.go"
tests_sha256: "a0a0107b684c99de674426896c5c2bc302321b5f7eab537c63054ef2fd1c3dfa"
touch_only: ['internal/posts/render.go', 'internal/posts/posts.go']
deps_allowed: ['modernc.org/sqlite', 'github.com/buke/quickjs-go']
forbids: ['network', 'subprocess', 'llm', 'unsafe']
---

# Contract: posts-render

## Intent
Read path de posts que convierte el contenido Markdown a HTML. El hook `post.render`
(C2) recibe `{ post: { content, title }, ... }` y retorna `{ html: "<p>..." } }`; si no
está registrado, Go usa un render Markdown→HTML de fallback (sintaxis mínima: `#`, `**`,
`` ` ``, listas). Luego `content.filter` sanitiza el HTML resultante (el hook recibe
`{ html }` y retorna `{ html }`).

El `Post.Content` raw (Markdown) se preserva; el HTML renderizado va en un campo
`HTML` nuevo (`RenderedPost`).

## Interface
```go
type RenderedPost struct {
  Post
  HTML string // contenido renderizado a HTML
}

func (s *Posts) GetRendered(ctx hooks.Context, id int64) (RenderedPost, error)
func (s *Posts) GetBySlugRendered(ctx hooks.Context, slug string) (RenderedPost, error)
func renderMarkdown(content string) string
```

## Invariants
- `Get`/`GetBySlug` (C36) SIGUEN retornando `Post` con `Content` raw (backward-compatible).
- `GetRendered`/`GetBySlugRendered` ejecutan `post.render` → `content.filter` en cadena;
  si el hook rechaza (`{ ok:false }`), se propaga el error.
- Sin hook `post.render` → `renderMarkdown` de fallback (no requiere JS).
- Sin hook `content.filter` → HTML pasa sin mutar.
- Errores wrappeados con `%w`.

## Examples
- `GetRendered(ctx, 1)` con hook `post.render` que envuelve `<p>` → `RenderedPost.HTML = "<p>texto</p>"`.
- `GetRendered(ctx, 1)` sin hooks → `renderMarkdown("# Hola")` → `<h1>Hola</h1>`.
- `content.filter` rechazando → `GetRendered` propaga el error y no retorna HTML parcial.

## Do / Don't
- DO: preservar C36 — `Get`/`GetBySlug` no cambian de comportamiento.
- DO: `renderMarkdown` maneja `#`, `##`, `**bold**`, `*italic*`, `` `code` ``, listas `-`.
- DO: envolver errores con `%w`.
- DON'T: tocar `internal/hooks/*`, `internal/db/*`, `db/migrations/*`.

## Tests
(oráculo RE-SELLADO en `internal/posts/posts_test.go`; SHA256
`6d72f54a4688ade36feeb61bac2675d421043f35517f43157d218f3fab9b3197`. Re-sellado
respecto a C36 con tests C37 añadidos: `TestRenderMarkdown_Fallback`,
`TestRenderMarkdown_Empty`, `TestGetRendered_FallbackMarkdown`,
`TestGetRendered_WithHook`, `TestGetRendered_HookRejects`, `TestContentFilterApplied`,
`TestGet_StillReturnsRawContent`. Los 7 tests C36 preservados siguen verdes.)

## Restricciones
- Tocar SOLO: `internal/posts/render.go`, `internal/posts/posts.go`,
  `internal/posts/posts_test.go` (re-sellado), `knowledge/contracts/posts-crud.md`
  (actualizar `tests_sha256`), `docs/reports/CONTRACT-36-REPORT.md` (documentar re-sellado),
  `knowledge/data_models/posts_render.md` (nuevo OKF node).
- No tocar: `internal/hooks/*`, `internal/db/*`, `db/migrations/*`, contratos/specs
  anteriores (C1-C36) salvo el re-sellado documentado.
- CGO_ENABLED=1 (heredado de C2).
- ABORTAR SI: render de fallback falla en sintaxis mínima, o `post.render` no se ejecuta
  antes de `content.filter`, o un hook rechazado no propaga error.

## Constraints
- PARAR y reportar si: el read path `Get`/`GetBySlug` (C36) deja de retornar `Content`
  raw (backward-compat rota), o `renderMarkdown` no produce `<h1>`/`<p>`/`<ul>`/`<strong>`
  para la sintaxis mínima, o un hook `post.render` rechazado no aborta `GetRendered`.

## Tests
(oráculo congelado SHA256 `…` en `internal/posts/posts_test.go`. RE-SELLADO respecto a
C36: el read path de `Get`/`GetBySlug` se preserva, pero se añaden `GetRendered`/
`GetBySlugRendered` + `renderMarkdown`; los tests C36 existentes siguen verdes. El
re-sellado se documenta en `docs/reports/CONTRACT-36-REPORT.md` como cambio legítimo
de scope C37 sobre el mismo paquete `internal/posts`.)

## Restricciones
- Tocar SOLO: `internal/posts/render.go`, `internal/posts/posts.go`,
  `internal/posts/posts_test.go` (re-sellado), `docs/reports/CONTRACT-36-REPORT.md`.
- No tocar: `internal/hooks/*`, `internal/db/*`, `db/migrations/*`, contratos/specs
  anteriores (C1-C36) salvo el re-sellado documentado.
- CGO_ENABLED=1 (heredado de C2).
- ABORTAR SI: el render de fallback falla en sintaxis mínima, o `post.render` hook no
  se ejecuta antes de `content.filter`, o un hook rechazado no propaga error.
