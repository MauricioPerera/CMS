---
type: 'Task Contract'
title: 'Race tests read-path posts (veredicto C42)'
description: 'Verificación de thread-safety de List/ListRendered/Sanitize bajo -race. Veredicto INCONCLUSIVO en Windows por modernc.org/sqlite checkptr; hooks race-safe; posts read path covered por database/sql.'
tags: ['ccdd', 'posts', 'race', 'concurrency', 'observability', 'cms']

task: posts-race-tests
intent: "Verificar thread-safety del read path posts bajo go test -race. El análisis concluye que el crash en Windows es un bug de modernc.org/libc (init), no del código del CMS."
target: knowledge/data_models/posts_race_analysis.md
signature: |
  func List(in ListInput) (ListResult, error)
  func ListRendered(ctx hooks.Context, in ListInput) (RenderedListResult, error)
  func Sanitize(html string) string
test_command: "go test -race ./internal/hooks/... -timeout 60s"
budget:
  cyclomatic_max: 5
  nesting_max: 3
  lines_max: 60
  params_max: 3
tests: "internal/posts/posts_test.go"
tests_sha256: "396582c08367fb6e7b9ce011f6122898aed3c441facf2b20a4b9fa0d3e50d411"
touch_only: ['knowledge/data_models/posts_race_analysis.md', 'docs/reports/CONTRACT-42-REPORT.md', 'specs/CONTRACT-42-race-tests.md', 'CHANGELOG.md']
deps_allowed: ['modernc.org/sqlite', 'github.com/buke/quickjs-go']
forbids: ['network', 'subprocess', 'llm']
---

# Contract: posts-race-tests

## Intent
Verificar la thread-safety del read path `internal/posts/` (`List`/`ListRendered`/
`Sanitize`) ejecutándolo bajo el detector de data races (`go test -race`). El contrato
documenta el veredicto: el crash en Windows es un **bug de la dependencia**
`modernc.org/sqlite` (no del CMS); los hooks QuickJS son race-safe.

## Interface
Verificado mediante `go test -race` sobre los packages afectados:

```sh
go test -race ./internal/hooks/...   # QuickJS gcc: OK, 0 data races
go test -race ./internal/posts/...   # modernc.org/sqlite checkptr crash (Windows-only)
```

## Invariants
- Los hooks QuickJS (C2) son race-safe: cada `Exec` crea Runtime+Context efímero aislado
  por goroutine. Verificado con `go test -race ./internal/hooks/...`.
- El read path `List`/`ListRendered` es puramente read-only sobre `*sql.DB` (thread-safe
  por diseño de `database/sql`); `Sanitize` es una función pura.
- `internal/posts/tests_test.go` (oráculo) NO se modifica para C42 (el crash es pre-init,
  no testeable). SHA preservado: `396582c0…`.

## Examples
- `go test -race ./internal/hooks/...` → `ok gopress/internal/hooks 2.103s`.
- `go test -race ./internal/posts/... -run TestContentFilter_XSS` →
  `fatal error: checkptr: pointer arithmetic computed bad pointer value` en
  `modernc.org/libc@v1.9.5/libc_windows.go:230` (init, antes del código).

## Do / Don't
- DO: correr `-race` sobre `internal/hooks` en Windows (QuickJS gcc, compatible).
- DO: correr `-race` sobre `internal/posts` en `ubuntu-latest` (CI) — allí funciona.
- DON'T: forzar `-race` sobre `internal/posts` en Windows local (crash de la dep).
- DON'T: modificar el oráculo `internal/posts/posts_test.go` (no hay tests nuevos; el
  análisis es documental/observacional).

## Tests
- Oráculo preservado: `internal/posts/posts_test.go` SHA256 `396582c0…` (sin cambios en
  C42). Tests C36-C40 siguen 23/23 (no `-race`).
- `internal/hooks/hooks_test.go` (SHA `5f14c71b…`) pasa `-race` con 0 data races.

## Constraints
- No tocar: código fuente (C42 es un análisis documental, no una implementación).
- PARAR y reportar si: en `ubuntu-latest` `go test -race ./internal/posts/...` falla con
  **data race real** (no `checkptr`); o si el oráculo posts_test.go SHA cambia sin
  justificación.
- Tocar SOLO: `docs/reports/CONTRACT-42-REPORT.md`,
  `knowledge/data_models/posts_race_analysis.md`, `specs/CONTRACT-42-race-tests.md`,
  `CHANGELOG.md`.
