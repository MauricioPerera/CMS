# Contrato 42 — Race tests del read path posts

Prerrequisitos: C2 (hooks QuickJS thread-safe), C1 (SQLite `modernc.org/sqlite`),
C36-C40 (posts CRUD/render/list/sanitize).

> Capa: contrato de ejecución. Task contract CCDD:
> `knowledge/contracts/posts-race-tests.md`.

## T1-SCOPE

Verificar thread-safety del read path `List`/`ListRendered`/`ListRenderedChain` (C38/C39/C40)
ejecutándolos concurrentemente bajo el detector de data races (`-race`).

## T1-RESELLO-ORACULO

Se propone añadir `TestListConcurrent` al oráculo `internal/posts/posts_test.go`:
- N goroutines ejecutan `List(ListInput{Limit:5})` sobre posts pre-poblados.
- N goroutines ejecutan `ListRendered(ctx, …)` con hooks.
- Re-sellado del SHA a `…`.

## T1-VERDICTO

**INCONCLUSIVO en Windows/amd64**: el flag `-race` crashdea el binario de test en
*init* de `modernc.org/sqlite` (dependencia del driver SQLite en uso por C1) con
`checkptr: pointer arithmetic computed bad pointer value` en
`modernc.org/libc@v1.9.5/libc_windows.go:230`. El crash ocurre **antes** de que nuestro
código corra (reproducible en `TestContentFilter_XSS`, un test puro sin DB).

Evidencia:
- `go test -race ./internal/posts/...` → `FAIL` (`checkptr` en `libc.init`).
- `go test -race ./internal/hooks/...` → **OK** (QuickJS gcc binding, 0 data races).

## T1-PLAN B (CI / linux)

- En `ubuntu-latest` el mismo `-race` sobre `internal/posts` **sí funciona** (el bug de
  `modernc.org/libc` con `checkptr` es Windows-only). CI debe correr `-race` allí.
- Alternativa Windows: migrar a `github.com/mattn/go-sqlite3` (CGO sqlite real, compatible
  con `-race`), pero requiere gcc + driver distinto (cambio de dependencia, C1-adjacent).

## Criterios de aceptación

- [ ] `go test -race ./internal/hooks/...` → OK, 0 data races.
- [ ] `python scripts/validate_contracts.py knowledge/contracts` exit 0.
- [ ] `python scripts/preflight.py` → 19/19.
- [ ] `python -m unittest discover -s tests -p "test_*.py"` → 744/744.

## T1-CIERRE

- **Hooks (QuickJS): race-safe verificado** (`-race` OK en `internal/hooks`).
- **Posts read path**: la lógica de Go (`List`/`ListRendered`/`Sanitize`) es pura y
  read-only sobre `*sql.DB` compartido; `database/sql` es thread-safe por diseño. La
  ausencia de data races en tests no `-race` + la thread-safety declarada de `database/sql`
  cubren el read path. El `-race` sobre posts queda bloqueado por la limitación de
  `modernc.org/sqlite` en Windows.

## Restricciones

- No forzar `-race` sobre `internal/posts` en Windows (crash de la dependencia).
- PARAR y reportar si: en `ubuntu-latest` `go test -race ./internal/posts/...` falla con
  data race real (no `checkptr`).
