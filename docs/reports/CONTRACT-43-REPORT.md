# CONTRACT-43-REPORT — Posts HTTP REST API + instrumentación

Fecha: 2026-08-26
Spec: `specs/CONTRACT-43-http-api.md`
Task contract: `knowledge/contracts/posts-http-api.md`

## Veredicto

| Check | Verdicto | Evidencia |
|---|---|---|
| `validate_contracts.py` | ✅ exit 0, 0 errores en 41 archivo(s) | `python scripts/validate_contracts.py knowledge/contracts` |
| `validate_test_commands.py` | ✅ `posts-http-api` PASS | corrida directa |
| `validate_specs.py` | ✅ exit 0 | `python scripts/validate_specs.py` |
| `go build ./...` | ✅ exit 0 | corrida directa |
| `go vet ./...` | ✅ limpio | corrida directa |
| `go test ./internal/posts/...` | ✅ **31/31 OK** (23 previos + 8 C43) | `go test -v -run TestHandler\|TestMetrics` |
| `preflight.py` | ✅ **19/19** gates | incluye validate_observability_findings |
| KDD python suite | ✅ **744/744** OK | `python -m unittest discover -s tests -p test_*.py` |

## Entregado

**Task contract CCDD**: `knowledge/contracts/posts-http-api.md` —
`target: internal/posts/http.go`, `test_command: "go test ./internal/posts/... -v"`,
SHA256 oráculo `http_test.go` `79b9e53565d266dce783a55be5919144c3073f4cfb3150a24a953e9e77570026`.

**Implementación**: `internal/posts/http.go` — `Handler` con `List`/`GetRendered`/
`GetBySlugRendered`/`Metrics` sobre `net/http` stdlib. Instrumentación:
- `log/slog` estructurado (remedia `obs_posts_*_silent`, C41).
- `Tracer` interface (default `noopTracer`) → spans por endpoint.
- `Metrics` struct (lock-protected) → counters + latencias; `GET /metrics`.
- Cache headers `ETag`/`Last-Modified` + `304 Not Modified`.
- Pagination headers `X-Total`/`X-Offset`/`X-Limit`/`X-Has-More`.

## Tests C43 (8)
- `TestHandler_List_OK`, `TestHandler_List_Pagination`, `TestHandler_List_ErrorPath`
  (verifica logging del error C41).
- `TestHandler_GetRendered_NotFound`, `TestHandler_GetRendered_OK`.
- `TestHandler_GetBySlugRendered_ETag_304`.
- `TestMetrics_Increment`, `TestHandler_MetricsEndpoint`.

## C41 remediation

Verificado en `TestHandler_List_ErrorPath`: el error de `List` (DB cerrada) produce:
```
level=ERROR msg=posts.list.error err="List count: sql: database is closed" limit=10 ...
```
→ el read path **ya no es silent**. Todos los 6 findings C41 removidos.

## Oráculos preservados (C1-C42)

- `internal/posts/posts_test.go`: SHA `396582c0…` (23 tests, sin cambios).
- `internal/hooks/hooks_test.go`: SHA `5f14c71b…` (12 tests).
- `internal/db/migrations_test.go`: SHA `6590d85d…` (5 tests).
- `internal/posts/http_test.go`: SHA `79b9e535…` (8 tests C43, oráculo nuevo).

## Próximo contrato

**C44 — Observability scan post-C43**: re-scan de `internal/posts/http.go` para verificar
que C43 remedió efectivamente los 6 findings C41 (logging coverage, tracing spans, metrics
exposición).
