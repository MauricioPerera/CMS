---
type: 'Task Contract'
title: 'DB + migraciones baseline'
description: 'Wrapper de golang-migrate sobre SQLite (modernc.org/sqlite, puro Go) + schema baseline (posts, users, sessions, options) con migraciones versionadas y tests de verificación.'
tags: ['ccdd', 'db', 'migraciones', 'sqlite', 'foundation']

task: db-migrations
intent: "Inicializar SQLite con migraciones versionadas (golang-migrate) y el schema baseline."
target: internal/db/init.go
signature: "func ApplyMigrations(dbh *sql.DB, sourceURL string) error"
test_command: "go test ./internal/db/... -run TestMigrations"
budget:
  cyclomatic_max: 10
  nesting_max: 4
  lines_max: 80
  params_max: 3
tests: "internal/db/migrations_test.go"
tests_sha256: "f1bf4aace6aa9b3b4a692386cfa05e4ad142485864c011723352c48af880a26a"
touch_only: ['internal/db/init.go', 'db/migrations/001_init.up.sql', 'db/migrations/001_init.down.sql', 'db/migrations/002_add_post_authors.up.sql', 'db/migrations/002_add_post_authors.down.sql', 'db/migrations/003_soft_delete.up.sql', 'db/migrations/003_soft_delete.down.sql']
deps_allowed: ['github.com/golang-migrate/migrate/v4', 'modernc.org/sqlite']
forbids: ['network', 'subprocess', 'llm']
---

# Contract: db-migrations

## Intent
Fundación del CMS: inicializar SQLite con migraciones versionadas usando
`golang-migrate/migrate` sobre el driver `modernc.org/sqlite` (puro Go, CGO_ENABLED=1
provisto por QuickJS). El schema baseline cubre posts, users, sessions y options. Las
tablas `taxonomy`/`terms` se dejan para el contrato de custom post types (C5).

## Interface
```go
func ApplyMigrations(dbh *sql.DB, sourceURL string) error
func Rollback(dbh *sql.DB, sourceURL string) error
```

## Invariants
- `ApplyMigrations` aplica SOLO migraciones hacia adelante (Up) hasta `version latest`.
- `Rollback` revierte hasta `version 0` (baja todo en orden inverso).
- Nunca lanza ante `sourceURL` inexistente sin devolver un error wrappeado.
- El schema baseline no cambia de vuelta: las tabs/posts/users/sessions/options tienen
  los campos, constraints y CHECK declarados en el data model.

## Examples
- `ApplyMigrations(db, "file://db/migrations")` sobre memoria → tablas posts, users,
  sessions, options creadas; posts.slug UNIQUE; users.email UNIQUE; posts.status con CHECK.
- `Rollback(db, "file://db/migrations")` → las 4 tablas borradas.
- Migrar sobre filesystem `file://db/migrations` → idempotente (segunda llamada no duplica).

## Do / Don't
- DO: usar `modernc.org/sqlite` (puro Go) para el driver SQL; `file://` para el source.
- DO: envolver errores con `%w` para trazabilidad.
- DON'T: hardcodear paths de migración (viene por `sourceURL`).
- DON'T: tocar `internal/db/migrations_test.go` (oráculo congelado), `go.mod`, nodos OKF existentes.

## Tests
(Los tests están en `internal/db/migrations_test.go`, oráculo congelado con SHA256
`6590d85d9e4dffee3147b21719016e964e11979d52d542fbc49ae257022efdec`. El PM lo autorizó
antes de delegar; el implementador no lo toca ni modifica.)

## Constraints
- PARAR y reportar si... `modernc.org/sqlite` no compila en Windows (tu entorno local)
  o en `ubuntu-latest` (CI), o si `golang-migrate` no aplica migraciones sobre SQLite
  en-memoria de forma reproducible.
