---
type: 'Task Contract'
title: 'Deploy gopress como Cloudflare Worker + production HTTP server'
description: 'Agregar entry point cmd/server y config wrangler para deploy productivo con CGO (go-sqlite3 + QuickJS).'
tags: ['ccdd', 'deploy', 'worker', 'cloudflare', 'server', 'c64']

task: gopress-deploy
intent: "Producir un binary Go ejecutable (package main) que sirva el Handler HTTP de posts sobre net/http stdlib con migraciones C1 aplicadas, configurado via env vars, deployeable a Cloudflare Workers via @cloudflare/sandbox proxy. Preservar CGO (mattn/go-sqlite3 + quickjs-go) y el project constraint go 1.22."
target: cmd/server/main.go
signature: |
  func main()
  func run() error
test_command: "go build ./cmd/server/"
budget:
  cyclomatic_max: 20
  nesting_max: 5
  lines_max: 180
  params_max: 3
tests: "internal/posts/security_hardening_test.go"
tests_sha256: "3a22774462b3f4d70d2278ac383f39eefa34b8000ecf9d14c31b8dbc4bdba2ae"
touch_only: ['cmd/server/main.go', 'workers/wrangler/src/worker.js', 'wrangler.jsonc', 'Makefile', '.gitignore', 'knowledge/contracts/C64-deploy.md', 'docs/reports/CONTRACT-64-REPORT.md', 'CHANGELOG.md']
deps_allowed: ['std', 'github.com/mattn/go-sqlite3', 'github.com/buke/quickjs-go', 'github.com/golang-migrate/migrate/v4']
forbids: ['network', 'subprocess']
---

# Contract: gopress-deploy

## Intent
Entregar `gopress` como **HTTP server productivo** deployeable a Cloudflare Workers, preservando CGO (mattn/go-sqlite3 + QuickJS) y el project constraint `go 1.22`.

## Interface
- `cmd/server/main.go` — entry point `package main`.
- `run(): Open DB → ApplyMigrations → NewPosts → NewHandler → http.ListenAndServe`.
- Env vars: `PORT`, `DB_PATH`, `MIGRATIONS_DIR`, `CMS_REQUIRE_AUTH`, `CMS_REQUIRE_LOGGER`, `CMS_BODY_LIMIT`.

## Invariants
- CGO_ENABLED=1 obligatorio (QuickJS + go-sqlite3).
- `go 1.22` directive preservado (no bump a 1.24 por deps).
- Oráculos frozen preservados (cmd/server es NEW file, no toca internal/).
- Default `DB_PATH=":memory:"` (dev); prod usa `file:/data/gopress.db`.

## Examples
- Dev: `go build -o bin/gopress-server ./cmd/server && PORT=8080 DB_PATH=:memory: ./bin/gopress-server`.
- CI Linux: `CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o bin/gopress-server-linux ./cmd/server`.
- Deploy: `wrangler deploy --env production` (binary arranca via @cloudflare/sandbox).

## Do / Don't
- **Do:** proveer `CMS_REQUIRE_AUTH=1` + `WithAuth` real en prod (C60 fail-fast).
- **Do:** build Linux en CI (host Windows no cross-compila CGO a Linux).
- **Don't:** commitear binarios (`bin/` en .gitignore).
- **Don't:** hardcodear secrets en wrangler.jsonc (usar `wrangler secret`).

## Tests
- `go build ./...` → OK.
- `go vet ./cmd/server/ ./internal/...` → clean.
- Smoke: `GET /healthz 200`, `GET /posts 200`.
- `go test -race ./internal/...` → 3/3 ok, 0 races.

## Constraints
- Go 1.22, CGO. No cross-compile Linux en host Windows (requiere CI Linux).
- **PARAR y reportar si** `go build ./cmd/server/` falla por CGO: confirmar gcc/MinGW presente.
