# Contrato 43 — Posts HTTP REST API + instrumentación

Prerrequisitos: C1 (schema), C2 (hooks), C36-C40 (posts CRUD/render/list/filter), C41
(observability findings), C42 (race verdict C40).

> Capa: contrato de ejecución. Task contract CCDD: `knowledge/contracts/posts-http-api.md`.

## T1-HTTP-HANDLER — stdlib net/http

OBJETIVO: `internal/posts/http.go` — `Handler` con `List`/`GetRendered`/`GetBySlugRendered`
sobre `net/http` stdlib (sólo `encoding/json`, `log/slog`, `net/http`).

- Routing manual (switch en `ServeHTTP` sobre `r.Method` + `r.PathValue`).
- `List`: query params `limit/offset/status/author/tag/q` → `ListInput`; headers de
  paginación `X-Total/X-Offset/X-Limit/X-Has-More`; JSON `[]RenderedPost`.
- `GetRendered`: `:id` (int) → `GetRendered`.
- `GetBySlugRendered`: `/s/:slug` → `GetBySlugRendered`.

## T1-CACHE-HEADERS

- `ETag` = `W/"<sha256[:8] del HTML>"`, `Last-Modified` = post.UpdatedAt.UTC.Format.
- `304 Not Modified` si `If-None-Match` matchea ETag o `If-Modified-Since` >= Last-Modified.
- `Cache-Control: no-cache` (read path dinámico; el ETag es el cache validator).

## T1-OBSERVABILITY (remedia C41)

- **Structured logging (`slog`)**: wrapper `logErr`/`logInfo` que incluye `endpoint`,
  `latency_ms`, `error`. Remedia `obs_posts_list_error_silent` + `obs_posts_listrendered_hook_silent`.
- **Tracer interface** (inyectable, default noop): `Start(name)` → `Span{End, Attr}`.
  Remedia `obs_posts_readpath_no_tracing`.
- **Metrics struct** (lock-protected): `Requests`, `Errors`, `SanitizeStripped` (int64) +
  `Latencies []time.Duration`. Endpoint `GET /metrics` expone JSON.
  Remedia `obs_posts_readpath_no_metrics` + `obs_no_alerting_readpath`.
- **Sanitize tracking**: `SanitizeStripped` incrementa cuando `len(out) < len(in)`.
  Remedia `obs_sanitize_strips_xss_silent`.

## T1-TESTS

`internal/posts/http_test.go` (oráculo):
- `TestHandler_List_OK`, `TestHandler_List_Pagination`, `TestHandler_List_ErrorPath`.
- `TestHandler_GetRendered_NotFound`.
- `TestHandler_List_ETag_304`.
- `TestMetrics_Increment`.

## Criterios de aceptación

- [ ] `python scripts/validate_contracts.py knowledge/contracts` exit 0 (42 archivo(s)).
- [ ] `go build ./...` exit 0.
- [ ] `go vet ./...` limpio.
- [ ] `go test ./internal/... ` → 5 db + 12 hooks + 23 posts + N http OK.
- [ ] `python scripts/preflight.py` → 19/19 (incluye validate_observability_findings).
- [ ] `python -m unittest discover -s tests -p "test_*.py"` → 744/744.

## Restricciones

### Tocar SOLO
- `internal/posts/http.go` (nuevo), `internal/posts/http_test.go` (nuevo),
  `knowledge/contracts/posts-http-api.md` (tests_sha256),
  `docs/reports/CONTRACT-41-REPORT.md` (marcar findings remediated),
  `docs/reports/CONTRACT-43-REPORT.md`, `CHANGELOG.md`.

- No tocar: `internal/hooks/*`, `internal/db/*`, `internal/posts/posts.go`/`render.go`
  (sólo leer), oráculos C1-C42 salvo re-sellado.
- ABORTAR SI: endpoints no aplican cache headers, `List` no loggea errores (C41 no
  remediated), o `-race` crash sobre `internal/posts` (bug de dep, documentado C42).
