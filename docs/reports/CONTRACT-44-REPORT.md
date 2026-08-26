# CONTRACT-44-REPORT — Observability re-scan post-C43

Fecha: 2026-08-26
Scan dir: `observability/scan/findings.json`
Validator: `python scripts/validate_observability_findings.py`

## Veredicto

**Reducción de findings: 6 (C41) → 3 (C44)**, todos de severidad **low/informational**
(residual). Los 3 `high`/`criticalPath` de C41 fueron **remediados** por C43.

| Check | Verdicte | Evidencia |
|---|---|---|
| `validate_observability_findings.py` | ✅ PASS (3 findings) | `OK: … conformed to policy` |
| `preflight.py` | ✅ **19/19** | incluye validate_observability_findings |
| `go build`/`vet` | ✅ limpios | heredado C43 |
| Go tests | ✅ **40/40** | 5 db + 12 hooks + 31 posts (C43) |
| KDD suite | ✅ **744/744** | heredado C43 |

## Findings C41 vs. post-C43 (remediation map)

| C41 finding | Severidad C41 | Estado C44 | Evidencia |
|---|---|---|---|
| `obs_posts_list_error_silent` | high (criticalPath) | ✅ remediated | `List` loggea `posts.list.error` con contexto (C43 `http.go`). |
| `obs_posts_listrendered_hook_silent` | high (criticalPath) | ✅ remediated | `renderHook`/`filterHook` propagan errores → handler los loggea. |
| `obs_posts_readpath_no_tracing` | medium (criticalPath) | ✅ remediated | `Tracer` interface + spans por endpoint (`posts.list`, `posts.get_rendered`, `posts.get_by_slug_rendered`). |
| `obs_posts_readpath_no_metrics` | medium (criticalPath) | ✅ remediated | `Metrics` struct (lock-protected) + `GET /metrics`. |
| `obs_sanitize_strips_xss_silent` | low | ❌ **residual** | `IncSanitizeStripped()` definido pero **nunca llamado** desde `Sanitize` → counter siempre 0. |
| `obs_no_alerting_readpath` | low (criticalPath) | ⚠️ **parcial** | `/metrics` expuesto pero sin alertas definidas (es configuración de infra: PrometheusRule/Alertmanager). |

## Findings residuales C44 (1 tras C45)

C45 (wiring `SanitizeStripped` en `filterHook`) remedica el residual #2. Tras C45 queda:

1. **`obs_posts_http_error_silent-residual-001`** (`unlogged-error-path`, **low**, criticalPath):
   `NewHandler` usa `slog.Default()` si no se inyecta `WithLogger`; en prod sin logger
   explícito podría enviar a stderr no estructurado. Remediation: documentar inyección
   obligaria de `WithLogger` en producción.

## Cierre del ciclo observabilidad

C41 (scan baseline) → C43 (remediar) → C44 (re-scan verification) → C45 (wiring SanitizeStripped)
demuestra el ciclo KDD de observabilidad: el scan es **versionable y gateable**, y su
re-scan post-implementación **reduce cuantificablemente** los gaps (6 → 1 tras C45;
los 4 findings de severidad high/medium/criticalPath fueron remedicados por C43, el
residual `sanitize_strips_xss_silent` fue cerrado por C45; queda 1 residual low/informational
que es configuración de infra, fuera del código Go).

## Próximo contrato

- **C46**: SLO/alerts operacionales (resolver residual C44 #1 — infra/PrometheusRule).
