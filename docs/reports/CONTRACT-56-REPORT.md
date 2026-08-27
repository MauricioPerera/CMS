# Contract 56 — E2E integration (httptest.NewServer sobre el mux real)

**Estado:** ✅ VERDE (draft → validated → implemented → verified)
**Fecha:** 2026-08-27
**Ciclo KDD:** Dogfood E2E del routing real Go 1.22 + auth middleware + rate-limit (C55). Capa 3 (tests).

## Contexto

Hasta C55 los tests de `internal/posts` ejercían handlers directamente (`httptest.NewRecorder` sobre `h.AuthRequired(h.X)`) o el store. C56 cierra el gap **E2E end-to-end real** levantando `httptest.NewServer(h)` sobre el `Handler` (que implementa `http.Handler` vía `ServeHTTP` → `h.smux`), disparando requests HTTP reales que cruzan:
- Go 1.22 `http.ServeMux` (patterns `{id}`, method-based `DELETE /posts/{id}`).
- Middleware `AuthRequired` (401/200 post-auth).
- Rate limiter `AuthRequired` (429, C55).
- Soft-delete read filter (C54): GET de borrado → 404.

## Tests (Capa 3 — `internal/posts/e2e_test.go`, package posts)

Nuevos 6 tests E2E:
- `TestE2E_MetricsAndRead`: `GET /metrics` público → 200 + JSON counters.
- `TestE2E_WriteAuthRejects`: `POST /posts` con `auth=rejectAll` → 401 (vía server real).
- `TestE2E_CreateWithAuth`: `POST /posts` con `auth=authOK` → 201.
- `TestE2E_GetRenderedRouting`: `GET /posts/{id}` → 200 + `<h1>` (render Markdown→HTML).
- `TestE2E_DeleteThenRestoreRouting`: `DELETE` → 204 → `GET` → **404** (soft-delete filter) → `RESTORE` → 200 → `GET` → 200 (read path vuelve a ver el post).
- `TestE2E_PatchUnauthAndRateLimit`: `PATCH` con `rejectAll` → 401; con `authOK` + capacidad 3 → 200×3 + 429.

## Invariants preservados

- Oráculo `http_test.go` SHA `854cbb82…` (C55) — C56 **no toca** `http_test.go`, crea `e2e_test.go` (file nuevo, no oráculo).
- Oráculos C1/C2/C36 (`migrations_test.go`/`hooks_test.go`/`posts_test.go`) preservados.
- `e2eSetupConfig` reusa `newTestHandler` (freshDB con migraciones 001–003 incluida).

## Verification

```
go vet ./internal/posts/...        # clean ✓
go test -race -count=1 ./internal/... -timeout 120s
  ok   gopress/internal/db       (5/5 migraciones)
  ok   gopress/internal/hooks    (14, 0 races)
  ok   gopress/internal/posts    (43 posts + 6 E2E C56, 0 races)

python scripts/validate_contracts.py knowledge/contracts  → OK (42)
python scripts/validate_specs.py                          → 0 errors / 44
python scripts/preflight.py                              → 19/19
python -m unittest discover -s tests -p "test_*.py"      → 744/744 OK
```

## Gate

- `go vet`: PASS (0 warnings).
- `validate_contracts.py`: PASS (42).
- `preflight.py`: 19/19.
- `-race`: 0 data races, 0 build failures.
- KDD suite: 744/744 OK.
