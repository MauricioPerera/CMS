# Contrato 01 — DB + migraciones baseline

Prerrequisitos: plantilla KDD en verde (19/19 gates, 744 tests). Este contrato pone la
base de datos baseline sobre la cual todos los contratos siguientes (hook system, posts,
auth, admin) dependen. Spec validado por `knowledge/contracts/validate-specs.md` y
`scripts/validate_specs.py`.

> Capa: contrato de ejecución (nivel proyecto). La tarea de código lleva su task contract
> CCDD en `knowledge/contracts/db-migrations.md`.

## DB-MIGRATE (T1) — migraciones estructuradas con golang-migrate sobre SQLite

OBJETIVO: `db/migrations/` con `001_init.up.sql` + `001_init.down.sql` (convención
golang-migrate), schema baseline verificable con `go test`. Driver `govlna/sqlite` (puro
Go; CGO_ENABLED=1 provisto por QuickJS). Primer `go.mod` del proyecto raíz.

### Schema baseline (C1)

- `posts`: `id` (INTEGER PK autoincrement), `slug` (TEXT UNIQUE NOT NULL), `title`
  (TEXT NOT NULL), `content` (TEXT NOT NULL), `status` (TEXT NOT NULL, CHECK en
  `draft`/`published`/`archived`), `created_at`/`updated_at` (TIMESTAMP NOT NULL DEFAULT
  now).
- `users`: `id` (INTEGER PK autoincrement), `email` (TEXT UNIQUE NOT NULL),
  `password_hash` (TEXT NOT NULL), `display_name` (TEXT), `created_at`/`updated_at`
  (TIMESTAMP NOT NULL DEFAULT now).
- `sessions`: `id` (TEXT PK, uuid), `user_id` (INTEGER NOT NULL REFERENCES users),
  `expires_at` (TIMESTAMP NOT NULL), `created_at` (TIMESTAMP NOT NULL DEFAULT now).
- `options`: `key` (TEXT PK), `value` (TEXT NOT NULL).

> `taxonomy`/`terms` y custom post types se dejan para C5/C6 (no van en el baseline).

### Convenciones

- Filenames: `<NNN>_<name>.up.sql` / `<NNN>_<name>.down.sql` (golang-migrate default).
- Versionamiento secuencial desde `001`.
- `go.mod` en raíz del repo (`module gopress`); `go.sum` generado.

### Tareas y perímetros (disjuntos)

- **T1-DB-SCHEMA** — `db/migrations/001_init.{up,down}.sql`: touch_only `['db/migrations/*.sql']`.
- **T1-APP-INIT** — `internal/db/init.go` (wrapper golang-migrate sobre sqlite):
  touch_only `['internal/db/init.go', 'go.mod']`.
- **T1-TESTS** — `tests_go/db_test.go` (oráculo congelado del PM):
  touch_only `['tests_go/db_test.go']`.

> Los tres son secuenciales (T1-APP-INIT importa el schema de T1-DB-SCHEMA; T1-TESTS
> asserta sobre T1-APP-INIT). El oráculo (T1-TESTS) es autorado por el PM y congelado; el
> implementador solo escribe T1-DB-SCHEMA + T1-APP-INIT.

## Criterios de aceptación

- [ ] `go build -o /dev/null ./...` exit 0 (el proyecto compila con CGO_ENABLED=1).
- [ ] `go test ./internal/db/... -run TestMigrations` verde (apply 001 → verifica 4 tablas +
  PK/UNIQUE/CHECKS; rollback → verifica tablas borradas) → **diff sobre `tests_go/` vacío
  al cierre** (el oráculo no se toca).
- [ ] `python scripts/validate_contracts.py knowledge/contracts` exit 0 (incluye
  `db-migrations.md`).
- [ ] `python scripts/validate_okf.py knowledge` exit 0 (schema baseline documentado como
  data model OKF, enlazado desde `index.md`).
- [ ] `python -m unittest discover -s tests -p "test_*.py"` verde (tooling KDD intocado).
- [ ] `python scripts/assemble_context.py ccdd/context.json "db migrations baseline" -v`
  → exit 0, stdout byte-identical entre 2 invocaciones (determinismo).
- [ ] Suite completa 2× verde (CI doble corrida).

## Restricciones

- Touch SOLO los archivos declarados en cada sub-tarea; `go.mod`/`go.sum` son compartidos
  (cualquier sub-tarea los modifica → lock de merge en CI).
- `govlna/sqlite`, `golang-migrate/migrate` las únicas deps nuevas (se agregan a `go.mod`).
- Sin red, sin LLM en el código Go.
- El schema baseline no cambia de vuelta: si las tablas migran, el contrato C5/C6 lo
  documenta y re-migrea; C1 no se reabre.
- NO commitear (el PM commitea tras verificar). Los entregables del agente van en
  `.agents/logs/db-migrations-REPORT.md` (gitignorado).
- ABORTAR SI: `go build` requiere dependencias no-pure-Go que rompan cross-compile sobre
  Windows (tu entorno local) o sobre `ubuntu-latest` en CI; o si golang-migrate no aplica
  migraciones sobre SQLite en-memoria reproducible.
