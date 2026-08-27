# Contract 55 — Rate limiting en write endpoints

**Estado:** ✅ VERDE (draft → validated → implemented → verified)
**Fecha:** 2026-08-27
**Ciclo KDD:** Extiende C47 (write path auth) + C51/C53/C54 (Delete/Restore/Patch). Capa 3 (HTTP tests).

## Contexto

Los write endpoints (POST/PUT/PATCH/DELETE) son endpoints mutantes — sin rate limiting
una fuente (user/IP) puede agotar SQLite (lock contention) o saturar el write path.
C41 (observabilidad) ya instrumenta reads; C55 cierra el gap de protección en writes.

## Diseño

**Token bucket por-key** (`internal/posts/ratelimit.go`), thread-safe (`sync.Mutex`):
- `TokenBucketRateLimiter(capacity, refillPerSec)`.
- Inicializa al **máximo de burst** (capacidad) en primera visita — el bucket arranca lleno.
- Refill proporcional a `elapsed` (token bucket clásico); cap a `capacity`.
- Si no hay token → 429 + `Retry-After` (tiempo hasta el siguiente token).

**Key de rate-limit** (auth primero, luego IP):
1. Usuario autenticado (`userKey` context, C47) → `user:<u>`.
2. `X-Forwarded-For` (primer elemento) → `ip:<ip>`.
3. `RemoteAddr` host → `ip:<ip>`.
4. Fallback → `ip:<unknown>`.

**Aplicación:** en el middleware `AuthRequired` (C47), que ya envuelve todos los writes
(`POST /posts`, `PUT /posts/{id}`, `POST /posts/{id}/publish`, `DELETE /posts/{id}`,
`POST /posts/{id}/restore`, `PATCH /posts/{id}`). Aplica SOLO a write methods
(`isWriteMethod`) — **reads (GET/HEAD) no se throttlean** (no impacta C41 read path).

**Integración opción:** `WithRateLimiter(rl RateLimiter)` — inyectable, nil-safe (default
sin rate limit = tests existentes no se rompen). La interfaz `RateLimiter` permite mockeos.

## Store/Handler touch points

- `ratelimit.go` (nuevo): `TokenBucketRateLimiter`, `RateLimiter`, `RateLimitResult`,
  `Allow`, `rateLimitKey`, `isWriteMethod`.
- `http.go`: `Handler.rl` field + `WithRateLimiter` option; lógica 429 en `AuthRequired`
  (post-auth, keyeado por usuario; pre-auth también si auth OFF pero rate-limit ON).
- **Invariante C55:** el rate limit no interfiere con `TestHandler_Delete_OK`/etc. — esos
  tests no inyectan `rl` (nil) → behavior unchanged.

## Tests (Capa 3)

`internal/posts/http_test.go` (+3 C55):
- `TestHandler_RateLimit_AllowsUnderLimit`: capacidad 2 → 2× PATCH 200, 3ra 429.
- `TestHandler_RateLimit_RejectsOverLimit`: capacidad 1 → 1× 200, 2da 429 + `Retry-After`.
- `TestRateLimiter_DirectTokens`: unit del limiter (1er Allow OK, 2do rechazado).

## Verification

```
go build ./...                 # clean
go vet ./...                   # clean
go test -race -count=1 ./internal/... -timeout 120s
  ok   gopress/internal/db      (5/5 migraciones)
  ok   gopress/internal/hooks    (14 tests, 0 races)
  ok   gopress/internal/posts    (43 tests + 3 C55, 0 races)

python scripts/validate_contracts.py knowledge/contracts  → OK (42)
python scripts/validate_specs.py                          → 0 errors / 44
python scripts/preflight.py                              → 19/19
python scripts/validate_observability_findings.py        → 0 findings
python -m unittest discover -s tests -p "test_*.py"      → 744/744 OK
```

## Oráculos preservados

- `posts_test.go` SHA `f500f75f…` ✅ (invariante C2).
- `hooks_test.go` SHA `5f14c71b…` ✅ (invariante C2).
- `migrations_test.go` SHA `f1bf4aac…` ✅ (invariante C1).
- `posts-http-api.md` re-sellado: SHA `854cbb82…`.

## Gate

- `validate_contracts.py`: PASS (42).
- `preflight.py`: 19/19.
- `-race`: 0 data races.
- KDD suite: 744/744 OK.
