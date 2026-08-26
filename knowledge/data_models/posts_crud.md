---
type: 'Data Model'
title: 'Posts CRUD'
description: 'Modelo de datos y operaciones CRUD para posts del CMS GoPress: schema tabla posts, inputs Create/Update, payload del hook post.validate, y ciclo de vida de status.'
tags: ['data-model', 'posts', 'crud', 'cms', 'sqlite', 'hooks']
---

# Data Model: `posts_crud`

Operaciones CRUD de posts sobre SQLite (schema C1), con integración del hook
`post.validate` (C2 QuickJS).

## Schema tabla `posts` (baseline C1)

| Campo | Tipo | Restricciones |
|---|---|---|
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT |
| `slug` | TEXT | NOT NULL, UNIQUE |
| `title` | TEXT | NOT NULL |
| `content` | TEXT | NOT NULL |
| `status` | TEXT | NOT NULL DEFAULT 'draft', CHECK IN ('draft','published','archived') |
| `created_at` | TIMESTAMP | NOT NULL DEFAULT datetime('now') |
| `updated_at` | TIMESTAMP | NOT NULL DEFAULT datetime('now') |

## Inputs

### `CreateInput`
- `Slug` (required, único) — identificador URL-friendly.
- `Title` (required) — título del post.
- `Content` (required) — contenido (Markdown raw).
- `AuthorID` — FK a `users.id` (referencial, no enforcado a nivel DB en C1).

### `UpdateInput`
- `ID` (required) — post a modificar.
- `Title`, `Content` — campos editables.
- `Status` — se pasa al hook; sólo `Publish` cambia el status (Update no lo muta).

## Hook `post.validate` (payload contract)

Se ejecuta **antes** de cada `INSERT`/`UPDATE`. Payload `map[string]any`:

```json
{
  "action": "create" | "update" | "publish",
  "post": { "id"?, "slug", "title", "content", "status" }
}
```

- **Return esperado**: `{ ok: true }` (procede) o `{ ok: false, error: "mensaje" }`
  (aborta la operación, no se escribe).
- El hook recibe `ctx.Point = "post.validate"`.
- `post.validate` rechazando → `Create`/`Update`/`Publish` retornan error wrappeado
  y la tabla `posts` queda inalterada.

## Ciclo de vida de `status`

```
draft → published → archived
   ↑                  ↓
   └──── (Update no muta status) ────┘
```

- `Create` → `status="draft"`.
- `Publish(id)` → `status="published"` (ejecuta hook con `action:"publish"`).
- `Update` → muta `title`/`content` únicamente (no toca `status`).
- `archived` → transición fuera de scope (C5 custom post types).

## Read path

- `Get(id)` — SELECT por PK, read-only (NO hook).
- `GetBySlug(slug)` — SELECT por slug, read-only (NO hook).
- Timestamps parseados a `time.Time` (Go `database/sql` + `modernc.org/sqlite`).

## Invariantes

- Hook `post.validate` siempre corre antes de la escritura; su rechazo aborta.
- Slug único a nivel DB (`UNIQUE` constraint C1).
- `Update` no muta `status` (sólo `Publish` lo hace).
- Errores wrappeados con `%w`.
