# Contrato 34 — DB + migraciones baseline (GoPress)

Prerrequisitos: plantilla KDD en verde (19/19 gates, 744 tests). Proyecto nuevo definido
en `DEFINITION.md` (CMS monolítico Go + SQLite + plugins JS, binario único). Este
contrato pone la base de datos baseline sobre la cual C2-C6 dependen.

> Capa: contrato de ejecución (nivel proyecto). La tarea de código (T1-DB-SCHEMA,
> T1-APP-INIT) lleva task contracts CCDD en `knowledge/contracts/db-migrations.md`.
> El oráculo (T1-TESTS) es autorado por el PM y congelado ANTES de delegar.

## T1-DB-SCHEMA — migraciones versionadas + schema baseline

OBJETIVO: `db/migrations/001_init.{up,down}.sql` con el schema baseline (posts, users,
sessions, options) validado por `knowledge/data_models/cms_schema.md`. Motor
`golang-migrate/migrate/v4`, driver `modernc.org/sqlite` (puro Go). Convención de
filenames `<NNN>_<name>.up.sql` / `.down.sql` (sin directives `-- +goose` — el parser
`iofs.PartialDriver` las interpreta y silencia el SQL).

## T1-APP-INIT — wrapper de migraciones en Go

OBJETIVO: `internal/db/init.go` con `ApplyMigrations`, `Rollback`, `Version` sobre una
instancia `*sql.DB` ya abierta.

## T1-TESTS — oráculo congelado

OBJETIVO: `internal/db/migrations_test.go` (5 tests, SHA256 sellado en el task contract).
Autorados por el PM. Verifican: tablas creadas, UNIQUE constraints, CHECK en status,
rollback dropea todo.

## Criterios de aceptación

- [x] `python scripts/validate_contracts.py knowledge/contracts` exit 0 (34 contratos, 0 errores).
- [x] `python scripts/validate_okf.py knowledge` exit 0 (cms_schema.md enlazado desde index.md).
- [x] `python scripts/validate_test_commands.py knowledge/contracts .` → PASS en db-migrations.
- [x] `go build ./...` exit 0 (CGO_ENABLED=1).
- [x] `go test ./internal/db/... -v` → 5/5 OK.
- [ ] `python -m unittest discover -s tests -p "test_*.py"` verde 2× (suite KDD intocada).
- [ ] CI doble corrida (ubuntu + windows) verde.

## Restricciones

- T1-DB-SCHEMA toca SOLO: `db/migrations/001_init.{up,down}.sql` + `knowledge/data_models/cms_schema.md` (data model OKF).
- T1-APP-INIT toca SOLO: `internal/db/init.go`, `go.mod`, `go.sum`.
- T1-TESTS NO se toca (oráculo congelado, `FM_TOUCH_TESTS`).
- `go.mod` compartido entre T1-DB-SCHEMA y T1-APP-INIT → lock de merge en CI.
- Sin red, sin LLM. CGO_ENABLED=1 requerido (QuickJS desde C2, modernc.org/sqlite es puro Go pero el build global usa CGo).
- NO commitear (el PM commitea tras verificar). Evidencia local en `.agents/logs/db-migrations-REPORT.md`.
- ABORTAR SI: `modernc.org/sqlite` no compila en Windows/CI; o `golang-migrate` no aplica migraciones sobre SQLite en-memoria de forma reproducible.
