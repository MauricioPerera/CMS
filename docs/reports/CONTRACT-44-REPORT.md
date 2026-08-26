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

## Findings residuales C44 (3)

1. **`obs_posts_http_error_silent-residual-001`** (`unlogged-error-path`, **low**, criticalPath):
   `NewHandler` usa `slog.Default()` si no se inyecta `WithLogger`; en prod sin logger explícito
   podría enviar a stderr no estructurado. Remediation: documentar inyección obligatoria.

2. **`obs_sanitize_strips_xss_silent-residual-002`** (`silent-failure`, **low**):
   `Sanitize` (render.go:142) no incrementa `Metrics.SanitizeStripped` → no se observa
   intentos de inyección XSS estrippeados. Remediation: pasar `*Metrics` a `filterHook` o
   hacer `Sanitize` recibirlo.

3. **`obs_no_alerting_readpath-residual-003`** (`no-alerting`, **informational**):
   Sin reglas de alerta (SLO/error_rate/p95) en el repo; es configuración de infra, fuera
   del código Go. Remediation: nuevo nodo ops o `knowledge/data_models/` con SLO/alerts.

## Cierre del ciclo observabilidad

C41 (scan baseline) → C43 (remediar) → C44 (re-scan verification) demuestra el ciclo
KDD de observabilidad: el scan es **versionable y gateable**, y su re-scan post-
implementación **reduce cuantificablemente** los gaps (6 → 3, severidad alta → baja).

## Próximos contratos

- **C45**: wiring de `SanitizeStripped` en `filterHook` (resolver residual #2).
- **C46**: SLO/alerts operacionales (resolver residual #3 — fuera de código Go).
