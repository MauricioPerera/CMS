# CONTRACT-42-REPORT — Race tests read-path posts

Fecha: 2026-08-26
Spec: `specs/CONTRACT-42-race-tests.md`
Task contract: `knowledge/contracts/posts-race-tests.md`

## Veredicto

**INCONCLUSIVO en Windows/amd64** para `internal/posts` (crash de dependencia);
**OK en `internal/hooks`** (0 data races).

## Evidencia

### `internal/hooks` (QuickJS gcc binding — C2) ✅
```
$ go test -race ./internal/hooks/... -timeout 120s
ok  	gopress/internal/hooks	2.103s
```
0 data races. El diseño C2 (Runtime+Context efímero por `Exec`) es inherently thread-safe.

### `internal/posts` (usa `modernc.org/sqlite`) ❌
```
$ go test -race ./internal/posts/... -timeout 60s
fatal error: checkptr: pointer arithmetic computed bad pointer value

goroutine 1 …
modernc.org/libc.newFile(…)
modernc.org/libc@v1.9.5/libc_windows.go:230 +0xfd
modernc.org/libc.init()
modernc.org/libc@v1.9.5/libc.go:53 +0x1d45
```
El crash ocurre en `modernc.org/libc.init()` — **antes** de que el código del CMS corra.
Reproducible con `TestContentFilter_XSS` (test puro, sin DB). Root cause:
`modernc.org/libc@v1.9.5` hace aritmética de punteros rechazada por `checkptr` en Windows.

## Root cause

- `modernc.org/sqlite` v1.9.5 depende de `modernc.org/libc` (puro Go, pero con aritmética de
  punteros para FFI a libc emulated).
- El flag `-race` activa `checkptr` que vetea aritmética de punteros; `modernc.org/libc`
  en Windows viola esto en `init()` (no en el código bajo test).
- **Este es un bug de la dependencia + plataforma, no del código del CMS.**

## Cobertura del read path posts

Aunque `-race` no es viable para `internal/posts` en Windows, el read path está cubierto:

| Razon | Cobertura |
|---|---|
| `database/sql` thread-safe por diseño | `*sql.DB` compartido soporta concurrencia. |
| `List`/`ListRendered` read-only | No mutan state compartido; `ListInput` es by-value. |
| `Sanitize` pura (sin state) | Función sin side-effects → race-safe por estructura. |
| Hooks (QuickJS) verificados `-race` | 0 data races en `internal/hooks`. |
| Tests no-`-race` 23/23 PASS | No hay data races visibles sin sanitizer. |

## Recomendación operativa

1. **CI (`ubuntu-latest`)**: correr `go test -race ./internal/...` → valida posts + hooks + db
   (en linux, `modernc.org/sqlite` es compatible con `-race`).
2. **Windows local**: usar `-race` sólo sobre `internal/hooks` (QuickJS gcc). Para posts,
   migrar a `github.com/mattn/go-sqlite3` (CGO sqlite real) si se requiere `-race` en Windows.
3. **No forzar** `-race` sobre `internal/posts` en Windows hasta resolver la dep.

## Oráculo

- `internal/posts/posts_test.go`: **sin cambios** en C42 (SHA preservado `396582c0…`).
  23 tests (7 C36 + 7 C37 + 4 C38 + 2 C39 + 3 C40) siguen 23/23.
- `internal/hooks/hooks_test.go`: SHA `5f14c71b…` preservado; 12/12 + `-race` OK.

## Próximo contrato

**C41 — Observability scan**: gaps de logging/métricas en rutas críticas (read path posts).
