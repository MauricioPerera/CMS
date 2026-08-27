---
type: 'Data Model'
title: 'Concurrency test strategy'
description: 'Estrategia de concurrencia para tests hooks + posts: aislamiento por runtime/DB, t.Parallel(), distintos slugs, -race gate. Habilitada por C48 (mattn CGO race-safe).'
tags: ['data-model', 'testing', 'concurrency', 'race', 'hooks', 'posts', 'kdd']
parent: 'knowledge/index.md'
---

# Data Model: `concurrency_test_strategy`

## Principio
La correctitud de concurrencia se valida con **concurrencia real a nivel de test** (no solo goroutines dentro de un test). `t.Parallel()` aísla tests en goroutines distintas → el scheduler Go ejecuta tests en paralelo.

## Reglas por package

### Hooks (QuickJS)
- `Runtime` (C2) no es thread-safe a nivel de instancia (QuickJS C state).
- **Isolamiento:** 1 `Runtime` por goroutine (`NewRuntime()` aísla context).
- `t.Parallel()` en tests de registry con distintos points.

### Posts (mattn/go-sqlite3, C48)
- `database/sql` es thread-safe (pool de conexiones).
- **Read path (`List`/`ListRendered`):** safe sobre `*sql.DB` compartido → DB única suficiente.
- **Write path (`Create`):** `mattn/go-sqlite3` default mode no es WAL → lock de writer. Aislamiento: DB distinta por goroutine + **slugs distintos** (unique key sin contention).

## Test commands
- `go test -race ./internal/hooks/... ./internal/posts/...` (C50 verify)
- `-count=10` para flakiness (CI opcional)

## Invariantes
- Oráculos congelados (`hooks_test.go`, `posts_test.go`) NO se tocan → SHA256 preserved.
- `t.Parallel()` en cada test nuevo → concurrencia del scheduler.
- Write tests: `t.Parallel()` + DB aislada + slug distinto.
