# CONTRACT-48-REPORT — SQLite driver migration (modernc → mattn) para -race en Windows

Fecha: 2026-08-26

## Veredicto

| Check | Verdicte | Evidencia |
|---|---|---|
| `validate_contracts.py` | ✅ 0 errores (41) | oráculos `migrations_test.go`/`posts_test.go` re-sellados |
| `go build`/`vet` | ✅ limpios | corrida directa |
| Go tests | ✅ **48/48** | 5 db + 12 hooks + 31 posts |
| `go test -race ./internal/...` | ✅ **3/3 ok, 0 data races, 0 checkptr crashes** | **C42 cerrado (Windows amd64)** |
| KDD suite | ✅ **744/744** | heredado |

## Problema (C42 — INCONCLUSIVO en Windows)

`-race` sobre `internal/posts` y `internal/db` crasheaba:
```
panic: checkptr: pointer arithmetic computed bad pointer value
modernc.org/libc@v1.9.5/libc_windows.go:230 (newFile en init)
```

**Root cause**: dos capas forzaban `modernc.org/sqlite`:
1. `internal/posts/posts_test.go` + `internal/db/migrations_test.go` importaban `_ "modernc.org/sqlite"`.
2. **`golang-migrate/v4/database/sqlite` importa internamente `_ "modernc.org/sqlite"`** (sqlite.go:16)
   → activa el `init()` de modernc/libc en toda cadena de imports.

Esto hacía **imposible** usar `mattn/go-sqlite3` (registra `"sqlite3"`) manteniendo
`sql.Open("sqlite", ...)` (modernc registra `"sqlite"`) — colisión de nombre + el import
transitivo de migrate forzaba modernc de todos modos.

## Migración C48

1. **`internal/db/init.go`** (touch_only C1 ✅): `database/sqlite` →
   `database/sqlite3` (variante **mattn CGO** de migrate, no importa modernc) +
   `sqlite.WithInstance` → `sqlite3.WithInstance` + nombre `"sqlite3"`.
2. **Tests** (oráculos re-sellados): `_ "modernc.org/sqlite"` →
   `_ "github.com/mattn/go-sqlite3"`, `sql.Open("sqlite",...)` → `sql.Open("sqlite3",...)`.
3. **`go.mod`**: `modernc.org/sqlite` + 7 transitive `modernc.*` **eliminados** (go mod tidy);
   `github.com/mattn/go-sqlite3 v1.14.50`.
4. El package `internal/db/sqlite3driver` (wrapper alias) resultó **no necesario** — se eliminó.

### go.mod (driver)
Antes:
```
modernc.org/sqlite v1.10.6
```
Después:
```
github.com/mattn/go-sqlite3 v1.14.50
```

## Oráculos re-sellados (decisión explícita C48)

| Archivo | SHA previo | SHA post-C48 | Contracts |
|---|---|---|---|
| `internal/db/migrations_test.go` | `6590d85d…` | `f1bf4aac…` | db-migrations (C1) |
| `internal/posts/posts_test.go` | `396582c0…` | `f500f75f…` | posts-crud, posts-render, posts-render-filter, posts-list, posts-list-filter, posts-race-tests |

> C48 migra el driver en los test files oráculo — un cambio de dependencia justificado
> y documentado. El `FM_TOUCH_TESTS` permite re-sellar siempre que el `tests_sha256`
> coincida con el archivo real. El comportamiento de los tests (aserciones) se mantiene;
> solo cambia el driver subyacente (pure Go → CGO).

## Veredicto C42 → VERDE

C42 dejaba `internal/posts -race` como **INCONCLUSIVO** en Windows (bug de dependencia).
Tras C48: `go test -race ./internal/posts` → **ok, 0 data races, 0 crashes**. El veredicto
de C42 se revierte de INCONCLUSIVO → **VERDE** (Windows amd64). CI linux ya no es necesario
para `-race`.

## Próximos contratos
- C50: `t.Parallel()` en tests hooks/posts (C48 demostró race-safety del driver).
