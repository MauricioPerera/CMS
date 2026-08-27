# CONTRACT-48-REPORT — SQLite driver migration (modernc → mattn) para -race en Windows

Fecha: 2026-08-26

## Veredicto

| Check | Verdicte | Evidencia |
|---|---|---|
| `validate_contracts.py` | ✅ 0 errores (41) | incluye oráculos re-sellados C48 |
| `validate_observability_findings.py` | ✅ PASS (0 findings) | post-C49 |
| `preflight.py` | ✅ **19/19** | incluye validate_observability_findings |
| `go build`/`vet` | ✅ limpios | corrida directa |
| Go tests | ✅ **48/48** (5+12+31) | `go test ./internal/...` OK |
| `go test -race ./internal/...` | ✅ **3/3 ok, 0 races, 0 checkptr crashes** | **C42 cerrado** |
| KDD suite | ✅ **744/744** | heredado |

## Problema (C42 — INCONCLUSIVO en Windows)

`-race` sobre `internal/posts` (y `internal/db`) crasheaba con:
```
panic: checkptr: pointer arithmetic computed bad pointer value
modernc.org/libc@v1.9.5/libc_windows.go:230
  modernc.org/libc.newFile → modernc.org/libc.init()
```

**Root cause**: `golang-migrate/v4/database/sqlite` hardcodea `_ "modernc.org/sqlite"` en su
`init()` (sqlite.go:16). `modernc.org/libc` usa unsafe pointer arithmetic incompatible con
`go test -race` en Windows. El crash se disparaba al importar `internal/db` (que importa
`database/sqlite` → `modernc`) o `_ "modernc.org/sqlite"` directo en los test files.

## Migración C48

1. **`internal/db/init.go`** (touch_only C1 ✅): `database/sqlite` → `database/sqlite3` +
   `sqlite.WithInstance` → `sqlite3.WithInstance` + `migrate.NewWithInstance(..., "sqlite3", ...)`.
2. **`internal/db/migrations_test.go`**: `_ "modernc.org/sqlite"` → `_ "github.com/mattn/go-sqlite3"`,
   `sql.Open("sqlite",...)` → `sql.Open("sqlite3",...)`.
3. **`internal/posts/posts_test.go`**: mismo cambio de import + `sql.Open`.
4. **`go.mod`**: `modernc.org/sqlite v1.10.6` eliminado (go mod tidy); `mattn/go-sqlite3 v1.14.50`.
5. El package `internal/db/sqlite3driver` (wrapper intermedio) resultó **no necesario** — se eliminó.

`go.mod` antes:
```
modernc.org/sqlite v1.10.6
```
Después:
```
github.com/mattn/go-sqlite3 v1.14.50
```

## Oráculos re-sellados (decisión explícita C48 — migración de dependencia)

| Archivo | SHA previo | SHA post-C48 | Contracts afectados |
|---|---|---|---|
| `internal/db/migrations_test.go` | `6590d85d…` | `f1bf4aac…` | `db-migrations` (C1) |
| `internal/posts/posts_test.go` | `396582c0…` | `f500f75f…` | `posts-crud`, `posts-render`, `posts-render-filter`, `posts-list`, `posts-list-filter`, `posts-race-tests` |

> El oráculo `posts_test.go` se comparte entre 6 contracts; todos re-sellados al nuevo
> SHA `f500f75f…` para mantener consistencia. C48 no cambió **comportamiento** (tests
> idénticos), solo el driver de SQLite (pure Go → CGO/C). C48 es un contrato de
> infra/dependencia.

## Veredicto C42

**C41/C42 (hooks -race ✅, posts -race ❌) → C49: VERDE.** Con `mattn/go-sqlite3` (CGO puro C
vía gcc 15.2), `go test -race ./internal/...` corre **0 data races, 0 checkptr crashes**
en Windows amd64. El veredicto "CI linux recomendado" deja de ser necesario — el read
path de posts ahora es `-race`-clean en Windows.

## Próximos contratos
- Back to dev: cualquier contrato funcional nuevo (C47 write API, etc.).
- El ciclo KDD observabilidad (C41→C49) + el gap `-race` Windows (C42→C48) quedan **cerrados**.
