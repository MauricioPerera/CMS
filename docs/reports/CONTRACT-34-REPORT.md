# CONTRACT-34 — DB + migraciones baseline — REPORT

Fecha: 2026-08-25
Spec: `specs/CONTRACT-34-db-migrations.md`

## Resumen ejecutivo

| Criterio | Veredicto | Evidencia |
|---|---|---|
| Validador de contratos | ✅ exit 0, 0 errores / 0 warnings | `python scripts/validate_contracts.py knowledge/contracts` |
| `test_command` del contrato | ✅ `go test ./internal/db/...` verde (5 tests) | corrida directa + `validate_test_commands.py` |
| Schema baseline | ✅ 4 tablas creadas (posts, users, sessions, options), constraints verificados | `TestMigrations_CreateTables`, `TestMigrations_PostsSchema`, `TestMigrations_UserEmailUnique`, `TestMigrations_PostStatusCheck` |
| Rollback | ✅ `001_init.down.sql` dropea las 4 tablas en orden inverso | `TestMigrations_RollbackDropsTables` |
| OKF | ✅ nodo `cms_schema.md` OKF-válido, enlazado desde `index.md` | `python scripts/validate_okf.py knowledge` → PASS |
| Go build | ✅ `go build ./...` exit 0 (CGO_ENABLED=1 implícito por modernc.org/sqlite) | corrida directa |
| Suite KDD (Python) | ✅ 744 tests verdes 2× | `python -m unittest discover -s tests` |
| Go build completo | ✅ compila sin errores de tipos ni imports | `go build ./...` |

## Entregado

**T1-DB-SCHEMA** (`db/migrations/001_init.up.sql` + `.down.sql`):
- `posts`: id, slug (UNIQUE), title, content, status (CHECK), timestamps.
- `users`: id, email (UNIQUE), password_hash, display_name, timestamps.
- `sessions`: id (TEXT PK), user_id (FK → users), expires_at, created_at.
- `options`: key (PK), value.
- Convención golang-migrate (`<NNN>_<name>.up.sql` / `.down.sql`), sin directives
  `-- +goose` (que el parser `iofs.PartialDriver` de golang-migrate interpreta y
  silencia — hallazgo documentado aquí).

**T1-APP-INIT** (`internal/db/init.go`):
- `ApplyMigrations(dbh, sourceURL)`, `Rollback(dbh, sourceURL)`, `Version(dbh, sourceURL)`.
- Driver `modernc.org/sqlite` (puro Go); source `file://` registado vía `source.Open`.
- Path de migraciones resuelto vía `runtime.Caller` en tests (independiente del cwd).

**Oráculo** (`internal/db/migrations_test.go`): 5 tests, SHA256
`6590d85d9e4dffee3147b21719016e964e11979d52d542fbc49ae257022efdec`.

**Data model OKF** (`knowledge/data_models/cms_schema.md`): schema baseline documentado
como nodo OKF `Data Model`, enlazado desde `knowledge/index.md`.

**Task contract** (`knowledge/contracts/db-migrations.md`): frontmatter OKF+CCDD con
`test_command` en verde, `tests_sha256` sellado, `touch_only` cubriendo solo
`internal/db/init.go`.

## Verificación final del PM

- `python scripts/validate_contracts.py knowledge/contracts` → exit 0 (34 contratos).
- `python scripts/validate_okf.py knowledge` → PASS (nuevo nodo enlazado).
- `python scripts/validate_test_commands.py knowledge/contracts .` → PASS en db-migrations.
- `go build ./...` → exit 0, CGO_ENABLED=1.
- `go test ./internal/db/... -v` → 5/5 OK.
- `python -m unittest discover -s tests -p "test_*.py"` → 744/744 OK (suite KDD intocada).

## Pendientes / ítems de seguimiento

Ninguno para C1. Próximo contrato: **C2 — Hook system** (`internal/hooks/`). El
`go.mod` ya tiene las deps para C1; C2 agregará `github.com/yosinon/xgo` (QuickJS).
