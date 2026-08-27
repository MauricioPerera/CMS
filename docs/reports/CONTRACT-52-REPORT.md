# Contract 52 — Delete auth-reject + bad-ID paths (cobertura write path)

**Estado:** ✅ VERDE (draft → validated → implemented → verified)
**Fecha:** 2026-08-27
**Ciclo KDD:** Extiende C51 (Delete) sobre `posts-http-api.md` (re-sellado SHA). Capa 3 (tests) → Nivel 1 (test_command).

## Contexto

C51 implementó `DELETE /posts/{id}` con auth + store `Delete`, pero sólo probó el **happy-path** (`Delete_OK`). C52 cierra la cobertura de auth en el write path Delete: caso en que el middleware `AuthRequired` **bloquea** la petición (401) antes de tocar el store, y caso de `id` no numérico (400).

## Tests (Capa 3 — `internal/posts/http_test.go`)

- `TestHandler_Delete_AuthRejected`: handler con `WithAuth(rejectAll)` + `AuthRequiredEnable(true)` → `AuthRequired(Delete)` retorna 401 y el post **no** se elimina (verificado via `s.Get` post-condición).
- `TestHandler_Delete_BadID`: path `/posts/not-a-number` → `parseIDFromPath` falla → 400 (`posts.delete.bad_id` logueado).
- Helper reutilizable `setupAuthHandlerWithReject` (extract del patrón `TestHandler_WritePath_AuthBlocksWhenEnabled`).

## Verification

```
go test -race -count=1 ./internal/posts/... -run TestHandler_Delete -v
→ PASS: TestHandler_Delete_OK (204)
→ PASS: TestHandler_Delete_NotFound (404)
→ PASS: TestHandler_Delete_WriteMetricsIncremented
→ PASS: TestHandler_Delete_AuthRejected (401)
→ PASS: TestHandler_Delete_BadID (400)

python scripts/validate_contracts.py knowledge/contracts
→ OK: todos los contratos son validos (42 contracts; posts-http-api.md SHA fb49259c…)
```

## Resultado

- +2 tests en `http_test.go` (5 total Delete tests).
- `posts-http-api.md` re-sellado: `tests_sha256` = `fb49259cc69732773ee83bd29a51099be6c6295bdee5f260c5768f23012b122e`.
- Auth `AuthRequired` verificado bloqueando Delete antes del handler (fail-fast en middleware, no en store).
- Oráculos congelados C2/C36 preservados.

## Gate

- `validate_contracts.py`: PASS.
- `preflight.py`: 19/19.
- `validate_observability_findings.py`: 0 findings.
- `-race`: 0 data races.
- KDD suite: 744/744 OK.
