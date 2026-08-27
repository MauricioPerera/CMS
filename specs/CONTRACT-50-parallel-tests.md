---
type: 'Spec'
title: 'Contract 50 — t.Parallel() concurrency guards en tests hooks + posts'
description: 'Spec de ejecución del task contract C50: agregar t.Parallel() explícito en tests hooks/posts bajo -race, sin tocar oráculos congelados C2/C36.'
tags: ['spec', 'testing', 'concurrency', 'race', 'hooks', 'posts']
contract: knowledge/contracts/parallel-tests.md
---

# Spec: Contract 50 — t.Parallel() concurrency guards

## Objetivo
Elevar la calidad de la suite de concurrencia de `gopress` activando `t.Parallel()` explícito en tests de hooks y posts. El driver `mattn/go-sqlite3` (C48) ya garantiza race-safety; C50 agrega **concurrencia real** (no solo goroutines dentro de un test) para forzar detección de data races.

## Alcance
- `internal/hooks/`: +2 tests en `concurrent_runtime_test.go` (NO toca `hooks_test.go`).
- `internal/posts/`: +2 tests en `concurrent_http_test.go` (NO toca `posts_test.go`).
- Oráculos congelados preservados (SHA256 unchanged).

## Test plan
| Test | Package | Parallel | Goroutines | DB/Runtime | Expectativo |
|---|---|---|---|---|---|
| `TestRuntime_ParallelCall` | hooks | ✅ | 50 | runtime distinto por goroutine | 0 races |
| `TestRegistry_ParallelDistinctPoints` | hooks | ✅ | 20 | registry distinto por goroutine | 0 races |
| `TestHandler_ParallelReads` | posts | ✅ | 50 | DB compartida (read-only) | 0 races |
| `TestHandler_ParallelCreateDistinctSlugs` | posts | ✅ | 10 | DB distinta por goroutine, slugs distintos | 0 races |

## Invariants de test
- `t.Parallel()` al entrar + `t.Parallel()` en cada subtest → scheduler real.
- Write tests: DB aislada por goroutine (mattn sqlite3 default mode no es WAL → serialización por DB es correcta). Slugs distintos → sin unique-key contention.
- Hooks: QuickJS `Runtime` no es thread-safe a nivel de instancia → 1 runtime por goroutine (NewRuntime aísla context).
- `-race` obligatorio en verify.

## Do / Don't
- DO: usar `t.Run(name, func(t) { t.Parallel(); ... })` para aislamiento subtest-level.
- DON'T: tocar `hooks_test.go` / `posts_test.go` (congelados C2/C36).
- DON'T: compartir `*sql.Tx` entre goroutines.

## Criterios de aceptación
- [x] `go test -race ./internal/hooks/... ./internal/posts/... -timeout 120s` → 0 races, todos PASS.
- [x] Verificado con `-count=3` → 0 flakes.
- [x] Oráculos congelados preservados: `5f14c71b…` (hooks_test.go) + `f500f75f…` (posts_test.go).
- [x] `python scripts/validate_contracts.py knowledge/contracts` → 42 contracts OK.

## Restricciones
- Tocar SOLO: `internal/hooks/concurrent_runtime_test.go`, `internal/posts/concurrent_http_test.go`, `knowledge/contracts/parallel-tests.md`, `docs/reports/CONTRACT-50-REPORT.md`, `CHANGELOG.md`, `knowledge/index.md`.
- No tocar: `hooks_test.go`, `posts_test.go`, `http_test.go`, `runtime.go`, `registry.go` (congelados C2/C36/C43).

## Verification gates
1. `go test -race -count=10 ./internal/hooks/... ./internal/posts/... -timeout 180s` → 0 races.
2. `python scripts/validate_contracts.py knowledge/contracts` → 42 contracts OK.
3. `python scripts/preflight.py` → 19/19 (incluye test_command C50).
4. Oráculos SHA `5f14c71b…` (hooks) y `f500f75f…` (posts) unchanged.
