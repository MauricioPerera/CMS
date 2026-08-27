# Contract 51 — Posts Delete (DELETE /posts/{id})

**Estado:** ✅ VERDE (draft → validated → implemented → verified)
**Fecha:** 2026-08-27
**Ciclo KDD:** Extiende C43 (`posts-http-api`) + C47 (write auth pattern). Capa 2 (task contract) sobre `posts-http-api.md` re-sellado.

## Contexto

El CRUD HTTP de C43/C47 (List/GetRendered/GetBySlugRendered/Create/Update/Publish) quedaba incompleto: no había forma de **eliminar** un post. C51 cierra el eslabón DELETE siguiendo el patrón de write establecido (C47: `AuthRequired` + `post.validate` hook + slog + Metrics).

## Store (Capa 1 — `internal/posts/posts.go`)

```go
// Delete elimina un post por ID (hard-delete). Ejecuta post.validate (action:"delete")
// antes del DELETE (invariante C36: hook antes de cada escritura).
// Retorna error wrapeado (sql.ErrNoRows) si no existe.
func (s *Posts) Delete(ctx hooks.Context, id int64) error
```

- Hard-delete (`DELETE FROM posts WHERE id = ?`): el schema C1 (`status CHECK IN ('draft','published','archived')`) no tiene `deleted_at`; `archived` es estado semántico distinto. Hard-delete preserva la simplicidad y la consistencia del read path (C38 filtra por `published`).
- `post.validate` con action:"delete" → hook puede veto (ej. post referenciado). Los tests C51 usan `hookOK` (acepta todo).
- Lookup previo (`SELECT id`) para diferenciar not-found (404) de hook-reject (400).

## HTTP (Capa 1 — `internal/posts/http.go`)

```go
// Delete elimina un post (DELETE /posts/{id}). Requiere auth (C47/C51).
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request)
```

- `DELETE /posts/{id}` registrado dentro de `h.AuthRequired(h.Delete)` → requiere auth.
- `parseIDFromPath` (reusado C47) para extraer `id`.
- Responses:
  - 204 No Content (éxito).
  - 400 Bad Request (`id` inválido o hook reject).
  - 404 Not Found (`id` inexistente).
- Instrumentación: `h.m.IncRequests()` + `IncErrors()` + `RecordLatency` + span `posts.delete`; slog `posts.delete.ok`/`posts.delete.error`/`posts.delete.bad_id` con `user` desde context (C41 remediation).

## Verification

```
go test -race -count=1 ./internal/posts/... -run TestHandler_Delete -v
→ PASS: TestHandler_Delete_OK (204 + post inexistente post-delete)
→ PASS: TestHandler_Delete_NotFound (404)
→ PASS: TestHandler_Delete_WriteMetricsIncremented (Requests > 0)

go test -race ./internal/... -timeout 120s
→ ok  gopress/internal/db
→ ok  gopress/internal/hooks
→ ok  gopress/internal/posts   (0 races)

python scripts/validate_contracts.py knowledge/contracts
→ OK: todos los contratos son validos (42 contracts; posts-http-api.md re-sellado SHA bdf8182f…)
```

## Resultado

- +3 tests en `http_test.go` (Delete OK / NotFound / Metrics).
- Store `posts.go` extiende con `Delete` (invariante hook-action preservado).
- Oráculos congelados C2/C36 preservados (`hooks_test.go` `5f14c71b…`, `posts_test.go` `f500f75f…`).
- `http_test.go` re-sellado → SHA `bdf8182f7adac70d265c0a246c3e21107618d6579e127d10a01411056a8d1897` en `posts-http-api.md`.

## Gate

- `validate_contracts.py`: PASS (42 contracts).
- `preflight.py`: 19/19.
- `validate_observability_findings.py`: 0 findings.
- KDD suite: 744/744 OK (tests python sin cambios; el +3 tests Go no impacta suite KDD).
- `-race`: 0 data races en todos los packages.
