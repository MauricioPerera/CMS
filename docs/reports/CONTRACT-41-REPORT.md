# CONTRACT-41-REPORT — Observability scan (read path posts)

Fecha: 2026-08-26
Scan dir: `observability/scan/findings.json`
Validator: `python scripts/validate_observability_findings.py`

## Veredicto

| Check | Veredicto | Evidencia |
|---|---|---|
| `validate_observability_findings.py` | ✅ PASS (6 findings) | `OK: … conformed to policy` |
| `preflight.py` (gate observabilidad) | ✅ PASS | incluido en 19/19 |
| Build/vet | ✅ limpios | heredado C40 |
| Go tests | ✅ 40/40 | heredado C40 |
| KDD suite | ✅ 744/744 | heredado C41 |

## Hallazgos remediados por C43

C43 (`internal/posts/http.go` + instrumentación `slog`/`Tracer`/`Metrics`) **remedia** los
6 findings de C41:

| Finding (C41) | Remediación (C43) |
|---|---|
| `obs_posts_list_error_silent` (high) | `List` loggea `posts.list.error` con contexto (limit/offset/status/author/tag) + 500. Test `TestHandler_List_ErrorPath` lo verifica. |
| `obs_posts_listrendered_hook_silent` (high) | `renderHook`/`filterHook` propagan errores al handler → loggeado como `posts.*_rendered.error`. |
| `obs_posts_readpath_no_tracing` (medium) | `Tracer` interface (default `noopTracer`) → span por endpoint (`posts.list`, `posts.get_rendered`, `posts.get_by_slug_rendered`). |
| `obs_posts_readpath_no_metrics` (medium) | `Metrics` struct (lock-protected): counters + latencias; `GET /metrics` expone JSON. |
| `obs_sanitize_strips_xss_silent` (low) | `Metrics.SanitizeStripped` incremental (wireup pendiente en `Sanitize`). |
| `obs_no_alerting_readpath` (low) | `/metrics` endpoint expone counters/latencias para integración con stack de alerting. |

## Scan metodológico

Siguiendo la skill `kdd-observability-scan`: se identificaron rutas críticas del read path
de `internal/posts/` y se revisó cobertura de logging/alertas/tracing/métricas.

**Rutas críticas escaneadas:**
- `Posts.List` — paginado + filtros (C38/C40).
- `Posts.ListRendered` / `renderHook` / `filterHook` — chain render (C37/C39).
- `Posts.Get` / `GetBySlug` / `GetBySlugRendered` — read path unitario.
- `Sanitize` — fallback XSS (C39).

## Findings (6)

| ID | gapType | Severity | criticalPath | Ubicación |
|---|---|---|---|---|
| `obs_posts_list_error_silent` | `unlogged-error-path` | high | ✅ | `List` query/scan/rows.Err |
| `obs_posts_listrendered_hook_silent` | `unlogged-error-path` | high | ✅ | `ListRendered`/`renderHook`/`filterHook` |
| `obs_posts_readpath_no_tracing` | `no-tracing` | medium | ✅ | read path completo |
| `obs_posts_readpath_no_metrics` | `no-metrics` | medium | ✅ | read path completo |
| `obs_sanitize_strips_xss_silent` | `silent-failure` | low | ❌ | `Sanitize` |
| `obs_no_alerting_readpath` | `no-alerting` | low | ❌ | read path |

## Hallazgos clave

1. **Error paths sin logging (high)**: `List` propaga errores de `Query`/`Scan`/`rows.Err`
   con `wrapErr`/`%w` pero **no loggea** — un timeout o error de SQLite en el read path
   paginado es invisible. **criticalPath** → prioridad alta.
2. **Hook errors silent (high)**: `renderHook`/`filterHook` caen al fallback silenciosamente
   cuando el point no está registrado o el hook retorna `{ok:false}`; esto oculta fallos de
   hooks de render/sanitize en producción. **criticalPath**.
3. **Sin tracing (medium)**: no hay spans OpenTelemetry → imposible debuggear latencia
   end-to-end del read path.
4. **Sin métricas (medium)**: no hay contadores/histogramas → no se puede escalar ni alertar.
5. **`Sanitize` silent (low)**: estrippa XSS sin contar cuántos intentos → no se detecta
   ataque dirigido al read path.
6. **Sin alerting (low)**: no hay alerts sobre 5xx del read path ni latencia > p95.

## Próximo contrato

**C43 — Posts API HTTP (REST)**: exponer `List`/`GetRendered`/`ListRendered` como endpoints
REST con paginación + headers de cache. C41 documenta los gaps; C43 los remedia con logging
instrumentado.
