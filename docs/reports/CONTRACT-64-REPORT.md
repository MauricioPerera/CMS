# CONTRACT-64-REPORT — Deploy gopress como Cloudflare Worker

**Estado:** ✅ VERDE (build + smoke test)
**Fecha:** 2026-08-25
**Skill:** `agents-sdk` / `wrangler` (Capa 3 → producción).

## Objetivo
Entregar `gopress` como **HTTP server productivo** deployeable a Cloudflare Workers, con CGO (mattn/go-sqlite3 + QuickJS) intacto.

## Diseño KDD
- El CMS era **library-only** (`internal/...`, sin entry point ejecutable). C64 agrega `cmd/server/main.go`.
- **Cloudflare Workers no ejecutan binaries Go nativos directamente** (plataforma JS/WASM). El binary CGO se ejecuta vía **@cloudflare/sandbox** (proceso aislado) y el Worker proxy HTTP al binary en `:PORT`.
- **Cross-compile a Linux (Worker runtime)** requiere toolchain Linux (no disponible en host Windows+MinGW) → se buildea en **CI Linux** (`Makefile build-linux`). Local build es Windows para dev.

## Artefactos

| Archivo | Propósito |
|---|---|
| `cmd/server/main.go` | Entry point `package main`: abre SQLite, corre migrations C1, construye `posts.Handler`, sirve `net/http` en `:PORT`. Env configurables (`CMS_REQUIRE_AUTH`, `CMS_BODY_LIMIT`, etc.). |
| `Makefile` | `build` / `build-linux` / `build-windows` / `smoke` / `test` / `deploy-deploy` / `deploy-dev`. |
| `wrangler.jsonc` | Config Worker: binary `gopress-cms`, vars prod (`DB_PATH=file:/data/...`, `CMS_REQUIRE_AUTH=1`, `CMS_BODY_LIMIT=524288`). |
| `workers/wrangler/src/worker.js` | Worker proxy: `/healthz` cacheado; resto proxy a binary `:PORT` vía sandbox. |
| `.gitignore` | `bin/` + `*.exe` excluidos (artefactos build, no versionados). |

## Smoke test (local Windows)
```
go build -o bin/gopress-server ./cmd/server
PORT=8199 DB_PATH=:memory: MIGRATIONS_DIR=db/migrations bin/gopress-server
  → GET /healthz  → 200 {"status":"ok"}
  → GET /posts   → 200 [] (empty list, migrations aplicadas)
```
✅ Verificado: `healthz=200`, `posts=200`.

## Constraints KDD verificados
- ✅ `go build ./...` + `go vet ./cmd/server/ ./internal/...` clean.
- ✅ `go test -race -count=1 ./internal/...` 3/3 ok, 0 races (oráculos preservados).
- ⚠️ Cross-compile Linux: **falla en host Windows** (MinGW no tiene headers `sys/mman.h`/`grp.h`) → build Linux en CI (`Makefile build-linux`); documentado como constraint, no defecto.
- ✅ `go.mod` sigue `go 1.22` (atomic bump de C63 preservado).

## Deployment path (Cloudflare)
1. CI (Linux): `CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o bin/gopress-server-linux ./cmd/server`.
2. `wrangler deploy --env production` → Worker arranca el binary via `@cloudflare/sandbox` binding.
3. Secrets via `wrangler secret put` (ej. `CMS_AUTH_TOKEN` para conectar a `WithAuth` real).

## Oráculos preservados
- Ningún cambio a `internal/...` (solo agregado `cmd/server` nuevo).
- Oráculos frozen sin tocar: `http_test.go`, `hooks_test.go`, `posts_test.go`, `migrations_test.go`.
