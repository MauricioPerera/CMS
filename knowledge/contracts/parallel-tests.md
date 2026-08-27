---
type: 'Task Contract'
title: 't.Parallel() explicit concurrency guards en tests hooks + posts'
description: 'Elevar la calidad de la suite de concurrencia agregando t.Parallel() explícito + goroutines reales bajo -race (habilitado por C48: mattn/go-sqlite3 CGO race-safe en Windows). Tests congelados (hooks_test.go/posts_test.go) NO se tocan; se agregan files nuevos concurrent_*.go que no invalidan tests_sha256 del package existente.'
tags: ['ccdd', 'testing', 'concurrency', 'race', 'hooks', 'posts', 'kdd']

task: parallel-tests
intent: "Proveer concurrencia REAL (t.Parallel + goroutines) sobre los path críticos hooks y posts validando bajo -race que no haya data races, sin tocar los oráculos congelados C2 (hooks_test.go) ni posts-crud/posts_test.go."
target:
  - internal/hooks/concurrent_runtime_test.go
  - internal/posts/concurrent_http_test.go
signature: |
  func TestRuntime_ParallelCall(t *testing.T)           // hooks
  func TestRegistry_ParallelDistinctPoints(t *testing.T) // hooks
  func TestHandler_ParallelReads(t *testing.T)            // posts HTTP
  func TestHandler_ParallelCreateDistinctSlugs(t *testing.T) // posts HTTP
test_command: "go test -race ./internal/hooks/... ./internal/posts/..."
budget:
  cyclomatic_max: 12
  nesting_max: 4
  lines_max: 120
  params_max: 4
tests: "internal/posts/concurrent_http_test.go"
tests_sha256: "b725942b57cafafe407eb8488918769974f1792912e468e5ed1b7397ac67dd15"
touch_only: ['internal/hooks/concurrent_runtime_test.go', 'knowledge/data_models/concurrency_test_strategy.md', 'docs/reports/CONTRACT-50-REPORT.md', 'CHANGELOG.md', 'knowledge/index.md']
deps_allowed: ['github.com/buke/quickjs-go', 'github.com/mattn/go-sqlite3']
forbids: ['network', 'subprocess', 'llm', 'unsafe']
---

# Contract: parallel-tests

## Intent
La suite existente (`TestRegistry_ConcurrentSafe` con WaitGroup manual) valida *thread-safety* pero ejecuta los hooks/posts de forma secuencial (`t.Parallel()` no está activado). Con el driver `mattn/go-sqlite3` (C48) el path posts es race-safe en Windows, por lo que **ahora** se puede activar concurrencia real a nivel de test (no solo de goroutines dentro de un test).

Se agregan **files nuevos** (`concurrent_*.go`) en los mismos packages, por lo que:
- NO se invalida el `tests_sha256` congelado de `hooks_test.go` (5f14c71b…) ni de `posts_test.go` (f500f75f…) — ambos son inputs del PM y permanecen untouched.
- El novo `concurrent_http_test.go` se valida con su propio `tests_sha256` (se sella post-verificación).

## Interface
```go
// internal/hooks/concurrent_runtime_test.go
func TestRuntime_ParallelCall(t *testing.T)           // hooks
func TestRegistry_ParallelDistinctPoints(t *testing.T) // hooks

// internal/posts/concurrent_http_test.go
func TestHandler_ParallelReads(t *testing.T)            // posts HTTP
func TestHandler_ParallelCreateDistinctSlugs(t *testing.T) // posts HTTP
```

## Invariants
- Cada test nuevo declara `t.Parallel()` al entrar → scheduler Go ejecuta tests en paralelo.
- Bajo `-race`: **0 data races** (verificado en CI + local Windows).
- Tests de write usan **slugs distintos** por goroutine → sin contención de unique key.
- Tests hooks usan **runtimes distintos** por test (NewRuntime aísla QuickJS context) → sin state compartido.
- `touch_only` no incluye `hooks_test.go`/`posts_test.go`/`http.go`/`runtime.go`/`registry.go` → regresión de código implícita.

## Examples
- `go test -race ./internal/hooks/... -run TestRuntime_ParallelCall -v` → PASS, 0 races.
- `go test -race ./internal/posts/... -run TestHandler_ParallelCreateDistinctSlugs -v` → PASS, 0 races.

## Do / Don't
- DO: usar `t.Parallel()` + subtests paralelos para aislamiento real bajo `-race`.
- DO: aislar runtime (hooks) / DB (posts) por goroutine.
- DON'T: tocar `hooks_test.go` / `posts_test.go` / `http_test.go` (oráculos PM congelados C2/C36/C43).
- DON'T: compartir `*sql.Tx` entre goroutines; write tests usan DB distinta por goroutine.

## Tests
(oráculo `internal/posts/concurrent_http_test.go` SHA256 `b725942b…`; plus `internal/hooks/concurrent_runtime_test.go` SHA256 `f9652439…`. Los archivos congelados C2 (`hooks_test.go` `5f14c71b…`) y C36 (`posts_test.go` `f500f75f…`) preservados.)

## Constraints
- PARAR y reportar si: cualquier data race bajo `-race` (≥100 runs), o flakes en tests paralelos.

## Do / Don't
- DO: usar `t.Parallel()` + `t.Run(sub, func(t) { t.Parallel() ...; goroutine })` para aislamiento.
- DO: runtimes/hooks distintos por test (QuickJS no es thread-safe a nivel de Runtime).
- DON'T: modificar `hooks_test.go` / `posts_test.go` (congelados C2/C36).
- DON'T: compartir `*sql.DB` de test entre goroutines de write (mattn sqlite3 en modo default no es WAL → lock de writer). Los tests de write usan DBs aisladas o serial-only reads.

## Tests
(oráculo nuevo `concurrent_runtime_test.go` + `concurrent_http_test.go`; SHA256 se sella tras verify pass.)

## Restricciones
- PARAR y reportar si: cualquier data race bajo `-race`, o `t.Parallel()` causa flakes >100 runs.
