# Contract 50 — t.Parallel() concurrency guards en tests hooks + posts

**Estado:** ✅ VERDE (draft → validated → implemented → verified)
**Fecha:** 2026-08-26
**Ciclo KDD:** Capa 3 (test) → Nivel 1 (test_command) → Capa 2 (task contract)

## Contexto

La suite existente de `gopress` valida thread-safety con goroutines + WaitGroup (`TestRegistry_ConcurrentSafe`), pero ejecuta tests de forma **secuencial** (`t.Parallel()` no activado). Con la migración a `mattn/go-sqlite3` (C48), el path posts es race-safe en Windows, lo que **habilita** concurrencia real a nivel de test.

Los oráculos congelados (`internal/hooks/hooks_test.go` SHA `5f14c71b…`, `internal/posts/posts_test.go` SHA `f500f75f…`) son inputs del PM y **no se tocan** — el Contract C2 (`hooks-runtime.md`) y C36 (`posts-crud.md`) lo prohíben explícitamente ("DON'T tocar... oráculo PM-frozen").

## Contract (Capa 2)

Task contract: `knowledge/contracts/parallel-tests.md` — `test_command: "go test -race ./internal/hooks/... ./internal/posts/..."`.

## Implementación (Capa 1)

Se agregan **files nuevos** (no invalidan SHA de files congelados):

### `internal/hooks/concurrent_runtime_test.go`
- `TestRuntime_ParallelCall`: `t.Parallel()` + N goroutines sobre **runtime distinto** invocando `post.validate` simultáneamente → 0 races.
- `TestRegistry_ParallelDistinctPoints`: N registries en paralelo con distintos points → 0 races.

### `internal/posts/concurrent_http_test.go`
- `TestHandler_ParallelReads`: `t.Parallel()` + goroutines sobre handlers `List`/`ListRendered` (read path sobre tabla compartida) → 0 races.
- `TestHandler_ParallelCreateDistinctSlugs`: `t.Parallel()` + goroutines creando posts con **slugs distintos** en **DB distinta** por test (mattn sqlite3 default mode no es WAL → aislamiento por DB).

## Verification

```
go test -race -count=3 ./internal/hooks/... ./internal/posts/... -timeout 120s
→ ok  	gopress/internal/hooks	(0 races, 0 flakes)
→ ok  	gopress/internal/posts	(0 races, 0 flakes)

python scripts/validate_contracts.py knowledge/contracts
→ OK: todos los contratos son validos (42 contracts)

python scripts/preflight.py
→ Summary: 19/19
```

## Resultado

- +4 tests nuevos (2 hooks + 2 posts). Posts 37→41; hooks 12→14 top-level tests.
- 0 data races bajo `-race` en Windows amd64 (gcc 15.2, CGO_ENABLED=1, mattn/go-sqlite3 v1.14.50).
- Oráculos congelados C2/C36 preservados: `5f14c71b…` (hooks_test.go), `f500f75f…` (posts_test.go), `2fca3fc4…` (http_test.go).
- Nuevos SHA: `concurrent_runtime_test.go` = `f9652439a53f4bc133ee39403dedee3dcac2d1e500677025e32f2a7a98c69db0`; `concurrent_http_test.go` = `b725942b57cafafe407eb8488918769974f1792912e468e5ed1b7397ac67dd15`.
- Data model OKF: `knowledge/data_models/concurrency_test_strategy.md` enlazado desde `index.md`.

## Gate

- `validate_contracts.py`: PASS (C50 contract válido).
- `validate_specs.py`: PASS (44 specs, 0 errors).
- `preflight.py`: 19/19.
- `validate_observability_findings.py`: 0 findings.
- KDD suite: 744/744 OK.
