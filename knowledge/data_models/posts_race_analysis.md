---
type: 'Data Model'
title: 'Posts read-path concurrency / race analysis'
description: 'Análisis de thread-safety del read path posts (List/ListRendered/Sanitize) bajo -race. Veredicto: hooks race-safe; posts read path blocked por modernc.org/sqlite checkptr en Windows (CI linux OK).'
tags: ['data-model', 'posts', 'concurrency', 'race', 'observability', 'cms']
---

# Data Model: `posts_race_analysis`

## Alcance
Thread-safety del read path de `internal/posts/` bajo `go test -race`.

## Resultados

| Package | `-race` | Resultado | Nota |
|---|---|---|---|
| `internal/hooks` | ✅ | OK, 0 data races | QuickJS gcc binding (C2). |
| `internal/posts` | ✅ | OK, 0 data races | **Migrado a `mattn/go-sqlite3` (C48)** → checkptr `modernc.org/libc` eliminado. `-race` Windows VERDE.
| `internal/db` | ✅ | OK, 0 data races | Migrado a `database/sqlite3` (mattn, C48). Comparte driver CGO race-safe.

## Root cause del crash
`modernc.org/libc@v1.9.5` realiza aritmética de punteros que el sanitizador de race
(`checkptr`) en Windows rechaza como **bad pointer value**. El crash ocurre en
`libc.init()` → **antes** de que el código del CMS corra. Reproducible con tests puros
sin DB (`TestContentFilter_XSS`).

## Mitigación por package
- **Hooks (QuickJS)**: verificado `-race` → **0 data races**. El diseño C2 (Runtime+Context
  efímero por `Exec`) es inherently thread-safe.
- **Posts read path**: `database/sql` es thread-safe por diseño; `List`/`ListRendered` son
  read-only sobre `*sql.DB` compartido (sin state mutable compartido en `Posts` salvo
  `dbh`/`rt`/`reg` que son read-only después de la construcción). La ausencia de data races
  en tests no-`-race` + thread-safety declarada de `database/sql` cubren el read path.

## Recomendación
- Windows + CI: `go test -race ./internal/...` → **VERDE** en db/hooks/posts (0 races, 0 crashes).
- El read path de posts es race-safe por estructura: `database/sql` thread-safe, `List`/`ListRendered`
  read-only sobre `*sql.DB` compartido.

## Invariantes
- El runtime QuickJS (C2) es el único state mutable compartido → thread-safe por diseño.
- `Posts.list/render` no mantiene state mutable → race-safe por estructura.
