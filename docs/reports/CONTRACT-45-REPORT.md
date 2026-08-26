# CONTRACT-45-REPORT — SanitizeStripped wiring en filterHook

Fecha: 2026-08-26
Spec: implícito en C44 residual + C43 `knowledge/contracts/posts-http-api.md`
Task contract: extendido sobre `knowledge/contracts/posts-render-filter.md`

## Veredicto

| Check | Verdicte | Evidencia |
|---|---|---|
| `validate_observability_findings.py` | ✅ PASS (2 findings) | `OK: … conformed to policy` |
| `preflight.py` | ✅ **19/19** | incluye validate_observability_findings |
| `go build`/`vet` | ✅ limpios | corrida directa |
| Go tests posts | ✅ **25/25** (23 previos + 2 C45) | `go test -v ./internal/posts/...` |
| KDD suite | ✅ **744/744** | heredado C43 |
| `go test -race ./internal/hooks/...` | ✅ 0 races | heredado C42 (QuickJS thread-safe) |

## Implementación C45

`internal/posts/posts.go` — `Posts` struct ganó campo opcional `metrics *Metrics`
(inyectable) + setter `SetMetrics(m *Metrics)` (nil-safe) + `incSanitizeStripped()`.
**No rompe el oráculo**: signature `NewPosts(dbh, rt, reg)` preservada → 23 tests C36-C44
pasados sin modificación.

`internal/posts/render.go` — `filterHook` delegatea en `sanitizeWithMetrics(html)`:
```go
out := Sanitize(html)
if len(out) < len(html) {
    s.incSanitizeStripped()
}
return out
```
Esto remedia el residual **6** de C41 (`obs_sanitize_strips_xss_silent`): el counter
`Metrics.SanitizeStripped` ahora refleja strips reales → observable vía `GET /metrics`.

## Tests C45 (2 — agregados al oráculo `http_test.go`)

- `TestSanitize_WiredViaHandler`: post con `<script>alert(1)</script># Hola` →
  `ListRendered` estrippa → `SanitizeStripped > 0`.
- `TestHandler_ListRendered_IncidentsMetric`: post con `onerror=alert(1)` →
  `GetBySlugRendered` (handler) → `SanitizeStripped > 0`.

## Oráculo re-sellado

`internal/posts/http_test.go` SHA `7da1c62af7a33c83153b5a0bf08eef608bf7c4d0927a1cc9e84c51b9a21ff4ba`
(previo C43 `79b9e535…`) — actualizado en `knowledge/contracts/posts-http-api.md`.

## Cierre observabilidad

C41 (6 findings) → C43 (4 remedicados) → C44 (re-scan 6→3) → C45 (6→1).
Queda **1 residual low/informational** (configuración de infra/alerts), fuera del scope
de código Go.

## Próximo contrato

- **C46**: SLO/alerts operacionales (PrometheusRule SLO/error_rate/p95) — cerrar
  el último residual.
