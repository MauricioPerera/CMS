---
type: 'Data Model'
title: 'Posts render (Markdown→HTML)'
description: 'Read path de posts: render Markdown→HTML via hook post.render, sanitización via content.filter, con fallback de sintaxis mínima. Backward-compatible con C36 (Get/GetBySlug preserva Content raw).'
tags: ['data-model', 'posts', 'render', 'markdown', 'hooks', 'cms']
---

# Data Model: `posts_render`

Evolución del read path de posts (C37). El write path (C36) es inalterado.

## Read path

1. `Get(id)` / `GetBySlug(slug)` (C36) → `Post` con `Content` raw (Markdown). **Backward-compatible.**
2. `GetRendered(ctx, id)` / `GetBySlugRendered(ctx, slug)` → `RenderedPost` con `HTML`.

## `RenderedPost`

```go
type RenderedPost struct {
  Post           // ID, Slug, Title, Content (raw Markdown), Status, timestamps
  HTML string     // contenido renderizado a HTML
}
```

## Cadena de hooks (read path)

### 1. `post.render`

Payload: `{ action: "render", post: { id, slug, title, content, status } }`.

Retorno esperado: `{ html: "<p>...</p>" }` — el HTML renderizado desde el Markdown `post.content`.

- Si el hook NO está registrado → fallback `renderMarkdown(content)` en Go (sin JS).
- Si el hook está registrado → usa su retorno `{ html }`.
- Si el hook rechaza (`{ ok:false, error }`) → se propaga el error, no se retorna HTML.

### 2. `content.filter`

Payload: `{ html: "<p>...</p>" }` (el HTML de `post.render` o fallback).

Retorno esperado: `{ html: "<sanitized html</p>" }` (filtra XSS, etc.).

- Si el hook NO está registrado → HTML sin mutar.
- Si rechaza → se propaga el error.

## Fallback `renderMarkdown` (sintaxis mínima)

Implementado en Go (stdlib `regexp`/`strings`), sin JS ni dependencias externas:

| Input | Output |
|---|---|
| `# Título` | `<h1>Título</h1>` |
| `## Subtítulo` | `<h2>Subtítulo</h2>` |
| `**bold**` | `<strong>bold</strong>` |
| `*italic*` | `<em>italic</em>` |
| `` `code` `` | `<code>code</code>` |
| `- item` | `<ul><li>item</li></ul>` |
| línea en blanco | `</p><p>` (separador) |

## Contexto del hook

- `ctx.Point = "post.render"` / `"content.filter"` (el `Context` de hooks C2).
- Los hooks reciben `ctx` como primer argumento JS.

## Invariantes

- `Get`/`GetBySlug` (C36) preservan `Content` raw — NUNCA mutado por render.
- `post.render` se ejecuta ANTES de `content.filter`.
- Un hook que rechaza aborta y propaga error (`%w`).
- Fallback `renderMarkdown` es determinista y no requiere CGO/JS.

## Backlinks

- Hook points: `hook_points.md` (`post.render`, `content.filter`).
- Posts CRUD: `posts_crud.md` (write path + `Get`/`GetBySlug` base).
