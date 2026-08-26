---
type: 'Data Model'
title: 'Posts list/search (paginado + status)'
description: 'Listado paginado de posts con search por slug/title y filtro por status. Read path público filtra a published. ListRendered aplica post.render/content.filter por item (C37).'
tags: ['data-model', 'posts', 'list', 'search', 'paginado', 'cms']
---

# Data Model: `posts_list`

Listado de posts (C38), evolución del read path. El write path (C36) no cambia.

## `ListInput`

| Campo | Tipo | Default/Clamp | Descripción |
|---|---|---|---|
| `Limit` | int | [1, 100], default 20 | tamaño de página |
| `Offset` | int | ≥ 0 | offset para paginar |
| `Status` | string | `""` (published) | filtro por status; `""` = público (solo published) |
| `Query` | string | `""` (sin search) | substring en `slug`/`title` (LIKE) |

## `ListResult`

| Campo | Descripción |
|---|---|
| `Items` | `[]Post` de la página. |
| `Total` | count total (sin LIMIT/OFFSET). |
| `HasMore` | `(offset + len(items)) < Total`. |

## `RenderedListResult`

Igual que `ListResult` pero `Items []RenderedPost` (con `HTML` renderizado C37).

## Read path público vs admin

- **Público** (`Status == ""`): `WHERE status='published'` — NUNCA devuelve drafts/archived.
- **Admin** (`Status != ""`): `WHERE status='<value>'` — permite filtrar por cualquier status
  válido (`draft`, `published`, `archived`).

## Search

`Query` non-empty → `AND (slug LIKE '%' || Query || '%' OR title LIKE '%' || Query || '%')`.
El `%` y `_` del Query se tratan como literales en la app (escaping mínico con ESCAPE);
el search NO es full-text (simple LIKE).

## ListRendered

- Ejecuta el chain `post.render` → `content.filter` (C37) sobre el `Content` de cada post
  en la página.
- Si un item falla el hook → se propaga el error y aborta el listado (no retorna parcial).
- Si no hay hooks registrados → `renderMarkdown` fallback por item.

## Invariantes

- `Limit` clampeado a `[1, 100]`; `Offset ≥ 0`.
- `HasMore` es consistente: `offset + len(items) < Total`.
- Read path público NUNCA devuelve `status != 'published'`.
- `Total` se calcula en query separado (no confiable como `len(items)`).

## Backlinks

- Posts CRUD: `posts_crud.md`.
- Posts render: `posts_render.md` (`RenderedPost`, hooks de read path).
- Hook points: `hook_points.md`.
