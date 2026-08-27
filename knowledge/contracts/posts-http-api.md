---
type: 'Task Contract'
title: 'Posts HTTP REST API + instrumentación (remedia C41)'
description: 'Endpoints REST stdlib net/http para List/GetRendered/GetBySlugRendered (read, C43) y POST/PUT/PUBLISH (write, C47) con paginación, cache headers, auth opcional y logging/tracing/metrics que remedian findings C41.'
tags: ['ccdd', 'posts', 'http', 'rest', 'observability', 'cms']

task: posts-http-api
intent: "Exponer el read path (C36-C40) y write path (C47) de posts como API REST con net/http stdlib: paginación LIMIT/OFFSET, headers de cache (ETag/Last-Modified), auth opcional inyectable + post.validate hook, y instrumentación (structured logging, tracing span, contadores/metrics) que remede los findings de C41."
target: internal/posts/http.go
signature: |
  func NewHandler(s *Posts, opts ...Option) *Handler
  func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request)
  func (h *Handler) List(w http.ResponseWriter, r *http.Request)                 // GET /posts
  func (h *Handler) GetRendered(w http.ResponseWriter, r *http.Request)           // GET /posts/{id}
  func (h *Handler) GetBySlugRendered(w http.ResponseWriter, r *http.Request)      // GET /posts/s/{slug}
  func (h *Handler) Create(w http.ResponseWriter, r *http.Request)                // POST /posts (auth, C47)
  func (h *Handler) Update(w http.ResponseWriter, r *http.Request)                 // PUT /posts/{id} (auth, C47)
  func (h *Handler) Publish(w http.ResponseWriter, r *http.Request)                // POST /posts/{id}/publish (auth, C47)
  func (h *Handler) Delete(w http.ResponseWriter, r *http.Request)                 // DELETE /posts/{id} (auth, C51)
  func (h *Handler) Patch(w http.ResponseWriter, r *http.Request)                  // PATCH /posts/{id} (auth, C53)
  func (h *Handler) AuthRequired(next http.HandlerFunc) http.HandlerFunc            // middleware auth (C47)
test_command: "go test ./internal/posts/... -v"
budget:
  cyclomatic_max: 16
  nesting_max: 4
  lines_max: 240
  params_max: 3
tests: "internal/posts/http_test.go"
tests_sha256: "32a5c428b70961a8a6213c8666a66096e3af1d9ee90efd6eeb6f7d7be44af881"
touch_only: ['internal/posts/http.go', 'internal/posts/posts.go', 'knowledge/contracts/posts-http-api.md', 'docs/reports/CONTRACT-43-REPORT.md', 'CHANGELOG.md', 'knowledge/data_models/posts_race_analysis.md', 'docs/reports/CONTRACT-41-REPORT.md', 'docs/reports/CONTRACT-47-REPORT.md', 'docs/reports/CONTRACT-51-REPORT.md', 'docs/reports/CONTRACT-53-REPORT.md']
deps_allowed: ['std']
forbids: ['network', 'subprocess', 'llm']
---

# Contract: posts-http-api

## Intent
Exponer el read path de posts (C36-C40) como API REST usando **solo stdlib** (`net/http`,
`encoding/json`, `log/slog`, `net/http/pprof` opcional). Instrumentar con structured
logging (`slog`), tracing span (mockable), y contadores/métricas básicas (mapa in-memory)
que **remedien los 6 findings de C41**:

- `obs_posts_list_error_silent` (high): loggear query/scan/rows.Err de `List`.
- `obs_posts_listrendered_hook_silent` (high): loggear hook errors en `ListRendered`.
- `obs_posts_readpath_no_tracing` (medium): span por endpoint (mockable via `Tracer`).
- `obs_posts_readpath_no_metrics` (medium): contador de requests + histograma latencia.
- `obs_sanitize_strips_xss_silent` (low): counter `sanitize_stripped`.
- `obs_no_alerting_readpath` (low): estructura `Metrics` para exponer vía `/metrics`.

## Interface
```go
type Handler struct { s *Posts; log *slog.Logger; tr Tracer; m *Metrics }
type Tracer interface { Start(name string) Span }; type Span interface { End(); Attr(k,v string) }
type Metrics struct { Requests, Errors, SanitizeStripped int64; Latencies []time.Duration }
func NewHandler(s *Posts, opts ...Option) *Handler
func (h *Handler) List(w http.ResponseWriter, r *http.Request)         // GET /posts
func (h *Handler) GetRendered(w http.ResponseWriter, r *http.Request)  // GET /posts/{id}
func (h *Handler) GetBySlugRendered(w http.ResponseWriter, r *http.Request) // GET /posts/s/{slug}
```

## Routing
- `GET /posts?limit=20&offset=0&status=published&author=1&tag=go&q=hola` → `List`.
- `GET /posts/{id}` → `GetRendered`.
- `GET /posts/s/{slug}` → `GetBySlugRendered`.
- `GET /metrics` → expone `Metrics` (counters + latencias).

## Cache headers
- `ETag` (hash simple del HTML) + `Last-Modified` (post.UpdatedAt).
- `304 Not Modified` si `If-None-Match` / `If-Modified-Since` coincide.

## Pagination metadata (response header)
- `X-Total`, `X-Offset`, `X-Limit`, `X-Has-More` en la respuesta de `List`.

## Observability (remedia C41)
- `slog` estructurado: `log.Info("posts.list.ok", "total", n, "latency_ms", ms)`.
- `Tracer` interface (inyectable; default `noopTracer`) → span por endpoint.
- `Metrics`: counters (requests/errors/sanitize_stripped) + latencias (slice lock-protected).
- `Sanitize` ya existente (C39) → `SanitizeStripped` incremental cuando `len(out) < len(in)`.

## Invariants
- `Handler` es stateless excepto `Metrics` (mutex-protected) → safe para concurrencia.
- Read path no muta posts → solo lecturas de DB.
- ETag derivado del HTML (sha256 truncado) → estable por contenido.
- 304 solo cuando `If-None-Match == ETag` o `If-Modified-Since >= Last-Modified`.

## Examples
- `GET /posts?limit=2&offset=2&tag=go` → 2 items + headers `X-Total`/`X-Has-More`.
- `GET /posts/42` → `RenderedPost` JSON + `ETag`/`Last-Modified`.
- `GET /posts/s/my-slug` segundo request con `If-None-Match` → 304.
- `GET /metrics` → `{"requests":N,"errors":M,"sanitize_stripped":K,"latencies_ms":[...]}`.

## Do / Don't
- DO: loggear **todos** los error paths con contexto estructurado (status, params, latency).
- DO: exponer `/metrics` para que el stack de observabilidad (C41/C42) los consuma.
- DON'T: bloquear el read path con tracing síncrono (usar noopTracer default).
- DON'T: retornar HTML sin ETag en read path renderizado.

## Tests
`internal/posts/http_test.go` (oráculo nuevo, SHA256 `79b9e535…`):
- `TestHandler_List_OK`, `TestHandler_List_Pagination`, `TestHandler_List_ErrorPath`
  (verifica logging del error → `obs_posts_list_error_silent` remediated).
- `TestHandler_GetRendered_NotFound`, `TestHandler_GetRendered_OK`.
- `TestHandler_GetBySlugRendered_ETag_304`.
- `TestMetrics_Increment`, `TestHandler_MetricsEndpoint`.

## Constraints
- Tocar SOLO: `internal/posts/http.go`, `internal/posts/http_test.go` (oráculo nuevo),
  `knowledge/contracts/posts-http-api.md` (tests_sha256), `docs/reports/CONTRACT-43-REPORT.md`,
  `CHANGELOG.md`, `docs/reports/CONTRACT-41-REPORT.md` (marcar findings remediated).
- No tocar: `internal/hooks/*`, `internal/db/*`, oráculos C1-C42 (`posts_test.go`,
  `hooks_test.go`, `migrations_test.go`).
- ABORTAR SI: `List` no loggea errores, o endpoints no aplican cache headers, o `-race` sobre
  `internal/posts` crash (bug de dep, documentado C42 — usar CI linux).
- PARAR y reportar si: `test_command` falla, o `preflight` < 19/19.
- ABORTAR SI: endpoints no aplican cache headers, o `List` no loggea errores (C41 no
  remediated), o `-race` crash sobre `internal/posts` (bug de dep, documentado C42).
- PARAR y reportar si: el `test_command` falla, o `preflight` < 19/19.
