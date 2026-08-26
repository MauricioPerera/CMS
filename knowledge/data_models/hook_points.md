---
type: 'Data Model'
title: 'CMS hook points'
description: 'Hook points del CMS GoPress: points, firma, payload contract y ejemplos de uso. El runtime QuickJS los expone a plugins JS.'
tags: ['data-model', 'hooks', 'plugin-system', 'cms', 'quickjs']
---

# Data Model: `hook_points`

Hook points del CMS GoPress (contrato C2: hooks). Cada point es una extensión bien
definida del core que un plugin JS puede interceptar. El runtime QuickJS
(`internal/hooks/runtime.go`) carga el JS; el registry
(`internal/hooks/registry.go`) lo dispatcha por point + prioridad.

## Hook points

### `post.validate`

- **Cuándo**: antes de `INSERT/UPDATE` en `posts` (CREATE, UPDATE, PUBLISH).
- **Payload**: `{ action: "create"|"update"|"publish", post: { id?, slug, title, content, status } }`.
- **Return**: `{ ok: true }` o `{ ok: false, error: "mensaje" }`.
- **Hook que falla**: aborta la operación con el error.

### `post.render`

- **Cuándo**: después de leer `content` de `posts` y antes de servir.
- **Payload**: `{ html: "<p>raw markdown rendered</p>", post_id: 1 }`.
- **Return**: `{ html: "<p>mutated html</p>" }` (reemplaza el `html`).

### `user.authenticate`

- **Cuándo**: en el handler de login (`POST /login`).
- **Payload**: `{ email: "x@y.z", password: "plaintext" }`.
- **Return**: `{ ok: true }` o `{ ok: false, error: "mensaje" }`.
- **Hook que falla**: rechaza el login (HTTP 401).

### `content.filter`

- **Cuándo**: después de `post.render`, antes de servir al cliente.
- **Payload**: `{ html: "<p>...</p>" }` (el HTML post-render).
- **Return**: `{ html: "<sanitized html</p>" }` (filtra XSS, etc.).

## Prioridades

- Lower priority = runs first (default 100).
- Ejemplo pipeline `content.filter`: `lower`(10) → `reverse`(50) → `upper`(100).

## Sandbox

- Memory limit (default 64MB; configurable via `WithMemoryLimit`).
- Timeout (default 5s; configurable via `WithTimeout`) vía interrupt handler.
- No `network`, no `subprocess`, no `unsafe` (enforced by task contract forbids).

## Invariantes

- `post.validate` fallando → operación aborta.
- `user.authenticate` fallando → login rechazado (401).
- `content.filter` siempre corre sobre HTML renderizado (nunca sobre Markdown raw).
- Hooks DISABLED no se ejecutan (flag `enabled=false`).
- Registry es thread-safe (mutex sobre map de hooks).
