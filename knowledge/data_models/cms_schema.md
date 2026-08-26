---
type: 'Data Model'
title: 'Schema baseline del CMS'
description: 'Esquema SQL baseline del CMS GoPress: tablas posts, users, sessions, options, con constraints e índices. Migrado versionado en db/migrations/.'
tags: ['data-model', 'db', 'schema', 'cms', 'foundation']
---

# Data Model: `cms_schema`

Schema baseline del CMS GoPress (contrato C1: db-migrations). Las reglas de
dominio que este schema persiste viven en los task contracts respectivos
(posts: `knowledge/contracts/posts-crud.md`; users: `knowledge/contracts/auth-users.md`);
este nodo solo es la definición de tabla, no la lógica.

## Migraciones

- Motor: `golang-migrate/migrate/v4` (source `file://`, driver `sqlite`).
- Location: `db/migrations/`.
- Convención de archivos: `<NNN>_<name>.up.sql` / `<NNN>_<name>.down.sql`.
- El driver SQLite es `modernc.org/sqlite` (puro Go; CGO_ENABLED=1 provisto por QuickJS).
- `001_init.up.sql` crea las 4 tablas baseline; `001_init.down.sql` las dropea en orden
  inverso (respetando foreign keys: sessions → users primero).

## Tablas

### `posts`

| Columna | Tipo | Restricciones |
|---|---|---|
| `id` | INTEGER PK AUTOINCREMENT | PK. Identificador único. |
| `slug` | TEXT NOT NULL UNIQUE | Identificador URL-friendly. |
| `title` | TEXT NOT NULL | Título del post. |
| `content` | TEXT NOT NULL | Contenido (Markdown). |
| `status` | TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','archived')) | Estado del post. |
| `created_at` | TIMESTAMP NOT NULL DEFAULT (datetime('now')) | UTC. |
| `updated_at` | TIMESTAMP NOT NULL DEFAULT (datetime('now')) | UTC, actualizado en cada modificación. |

### `users`

| Columna | Tipo | Restricciones |
|---|---|---|
| `id` | INTEGER PK AUTOINCREMENT | PK. |
| `email` | TEXT NOT NULL UNIQUE | Único a nivel de tabla. |
| `password_hash` | TEXT NOT NULL | bcrypt (60 chars). Nunca plaintext. |
| `display_name` | TEXT | Opcional. |
| `created_at` | TIMESTAMP NOT NULL DEFAULT (datetime('now')) | UTC. |
| `updated_at` | TIMESTAMP NOT NULL DEFAULT (datetime('now')) | UTC. |

### `sessions`

| Columna | Tipo | Restricciones |
|---|---|---|
| `id` | TEXT PK | UUID de la sesión. |
| `user_id` | INTEGER NOT NULL REFERENCES users(id) | FK a users. |
| `expires_at` | TIMESTAMP NOT NULL | Expiración. |
| `created_at` | TIMESTAMP NOT NULL DEFAULT (datetime('now')) | UTC. |

### `options`

| Columna | Tipo | Restricciones |
|---|---|---|
| `key` | TEXT PK | Clave de configuración. |
| `value` | TEXT NOT NULL | Valor (serializado). |

## Invariantes

- `posts.slug` y `users.email` son únicos a nivel de tabla.
- `posts.status` está limitado al checklist (`draft`/`published`/`archived`) vía CHECK.
- `sessions.user_id` referencia `users(id)` (FK).
- `password_hash` nunca se serializa en respuestas de API ni se loguea.
- `created_at` es inmutable tras la inserción; `updated_at` se actualiza en cada modificación.
