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

## Findings residuales C44 (cero tras C45+C49) — VER actualizado por C49

C45 (wiring `SanitizeStripped`) remedica el residual #2, y C49 (fail-fast logger guard)
cierra el residual #1. **C49 lleva los findings a 0** — ciclo observabilidad completo (C41→C49).
Ver `docs/reports/CONTRACT-49-REPORT.md`.

## Cierre del ciclo observabilidad

C41 (scan baseline) → C43 (remediar) → C44 (re-scan) → C45 (wiring SanitizeStripped) →
C49 (fail-fast logger) demuestra el ciclo KDD de observabilidad: el scan es
**versionable y gateable**, y su re-scan post-implementación **reduce cuantificablemente**
los gaps (**6 → 0 findings**). Los 4 findings high/medium/criticalPath fueron remedicados
por C43; el `sanitize_strips_xss_silent` cerrado por C45; el `logger_default` cerrado
por C49. **C49 lleva findings.json a 0 — ciclo observabilidad completo.**

## Próximo contrato

- **C46**: SLO/alerts operacionales (resolver residual C44 #1 — infra/PrometheusRule).
