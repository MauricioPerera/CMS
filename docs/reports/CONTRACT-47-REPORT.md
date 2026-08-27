# CONTRACT-47-REPORT — Posts Write API (Create/Update/Publish) + auth

Fecha: 2026-08-26
Spec: `knowledge/contracts/posts-http-api.md` (extendido C43).
Implementación: `internal/posts/http.go` (write path + auth middleware).

## Veredicto

| Check | Verdicte | Evidencia |
|---|---|---|
| `validate_contracts.py` | ✅ 0 errores (41) | incluye `posts-http-api.md` (SHA re-sellado) |
| `validate_observability_findings.py` | ✅ PASS (0 findings) | post-C49 |
| `preflight.py` | ✅ **19/19** | incluye validate_observability_findings |
| `go build`/`vet` | ✅ limpios | corrida directa |
| Go tests posts | ✅ **37/37** (28 previos + 9 C47) | `go test -v -run TestHandler_Create\|TestHandler_Update\|TestHandler_Publish` |
| `go test -race ./internal/...` | ✅ **3/3 ok, 0 races** | C48 (mattn) — posts ahora race-safe en Windows |
| KDD suite | ✅ **744/744** | heredado |

## Implementación C47

`internal/posts/http.go` — endpoints write sobre el store `Posts` existente (C36/C40),
reutilizando el hook `post.validate` (C2) para validación de dominio:

| Endpoint | Handler | Auth | Store |
|---|---|---|---|
| `POST /posts` | `Create` | `AuthRequired` (C47) | `h.s.Create` |
| `PUT /posts/{id}` | `Update` | `AuthRequired` (C47) | `h.s.Update` |
| `POST /posts/{id}/publish` | `Publish` | `AuthRequired` (C47) | `h.s.Publish` |

**Auth (C47):** `AuthFunc` inyectable via `WithAuth(f)` + activador `AuthRequiredEnable(true)`.
Middleware `AuthRequired(next)`: si `authRequired && auth==nil` → 500 (misconfigured); si
`auth` rechaza → 401; si OK, inyecta `user` en contexto (logging de auditoría). Por defecto
auth OFF (tests/local).

**Observabilidad (extendida C41):**
- slog estructurado: `posts.create.ok/error`, `posts.update.*`, `posts.publish.*` con user.
- Tracer spans: `posts.create`, `posts.update`, `posts.publish`.
- Metrics: write requests incrementan `Requests`/`Errors`/latencia (compartidos con read).
- Validation 400 con contexto (slug/title/content vacíos).

## Tests C47 (9 — agregados al oráculo `http_test.go`)
- `TestHandler_Create_OK`, `TestHandler_Create_ValidationFail`, `TestHandler_Create_BadJSON`.
- `TestHandler_Create_AuthRejected` (misconfigured → 500), `TestHandler_WritePath_AuthBlocksWhenEnabled` (rechaza → 401).
- `TestHandler_Update_OK`, `TestHandler_Publish_OK`, `TestHandler_Publish_NotFound`.
- `TestHandler_Create_WriteMetricsIncremented` (write incrementa `Metrics.Requests`).

## Oráculo re-sellado
`internal/posts/http_test.go` SHA `2fca3fc4763e0d6553f78792c845e8f78726830514a7de55e148f13ee6e38d51`
(previo C45 `7da1c62a…` → C45/C49 → ahora C47) — actualizado en
`knowledge/contracts/posts-http-api.md`.

## Integración con C48
C47 + C48 son conjuntos: C47 agrega write handlers; C48 migró el driver SQLite a mattn
(cgO) para que `go test -race ./internal/posts` funcione en Windows. Ambos sellados juntos.

## Próximos contratos
- C50: migrar `internal/hooks` a concurrent `-race` tests con `t.Parallel()` (C48 abrió el camino).
