---
type: 'Data Model'
title: 'Posts filter por author + tags'
description: 'Extiende posts con author_id (FK users) + tabla join post_tags; ListInput con AuthorID/Tag. Read path admin con combinación AND.'
tags: ['data-model', 'posts', 'filter', 'author', 'tags', 'db', 'migraciones']
---

# Data Model: `posts_filter`

Listado filtrado de posts (C40), evolución de C38 (`posts_list`).

## Migración 002

### `posts.author_id`
| Columna | Tipo | Restricciones |
|---|---|---|
| `author_id` | INTEGER | nullable, `REFERENCES users(id)` (FK). |

- Nullable: posts creados sin author (`AuthorID == 0`) → `author_id IS NULL`.
- Índice `idx_posts_author` sobre `author_id` para filtrado.

### `post_tags` (tabla join)
| Columna | Tipo | Restricciones |
|---|---|---|
| `post_id` | INTEGER | PK, `REFERENCES posts(id) ON DELETE CASCADE`. |
| `tag` | TEXT | PK. |

- Índice `idx_post_tags_tag` sobre `tag` para búsquedas por tag.

## `ListInput` extendido
| Campo | Tipo | Default | Descripción |
|---|---|---|---|
| `Limit` | int | [1, 100], 20 | tamaño de página. |
| `Offset` | int | ≥ 0 | offset. |
| `Status` | string | `""` (published) | filtro por status. |
| `Query` | string | `""` | search slug/title. |
| `AuthorID` | int64 | 0 | filtro por author; 0 = sin filtro. |
| `Tag` | string | `""` | filtro por tag; `""` = sin filtro. |

## Combinación AND
- Read path público (`Status==""`): `published` + (author_id si >0) + (tag si "") + (query si "").
- Read path admin (`Status!=""`): status + autor + tag + query.
- Todos los filtros son AND (un post debe cumplir todos).

## `Create` con author_id
- Si `CreateInput.AuthorID > 0` → `INSERT … (author_id)`.
- Si `AuthorID == 0` → `INSERT` sin `author_id` (columna nullable).

## Invariantes
- `author_id` FK → `users(id)`; inserción de author_id inexistente falla (FK constraint).
- `post_tags.post_id` ON DELETE CASCADE: borrar post borra sus tags.
- Read path público NUNCA expone drafts/archived (filtrado `published`-only).

## Backlinks
- Schema baseline: `cms_schema.md`.
- Posts CRUD: `posts_crud.md`.
- Posts list/search: `posts_list.md`.
- Hook points: `hook_points.md`.
