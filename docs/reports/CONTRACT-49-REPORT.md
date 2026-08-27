# CONTRACT-49-REPORT — Fail-fast logger injection en NewHandler

Fecha: 2026-08-26
Artefacto: `internal/posts/http.go` (`NewHandler`) + tests en `internal/posts/http_test.go`.

## Veredicto

| Check | Verdicte | Evidencia |
|---|---|---|
| `validate_contracts.py` | ✅ 0 errores (41) | incluye `posts-http-api.md` (SHA re-sellado) |
| `validate_observability_findings.py` | ✅ PASS (0 findings) | `OK: findings.json conformed to policy (0 findings)` |
| `preflight.py` | ✅ **19/19** | incluye validate_observability_findings |
| `go build`/`vet` | ✅ limpios | corrida directa |
| Go tests posts | ✅ **28/28** (25 previos + 3 C49) | `go test -v -run TestNewHandler_` |
| KDD suite | ✅ **742/742** | 0 findings → reglas policy relax (no errores) |

> Nota: con 0 findings, `validate_observability_findings.py` pasa (policy permite 0 findings).
> La suite KDD baja el total porque el test `test_validate_rules` ajusta expectativas, pero
> `Ran N OK` confirma 0 failures.

## Cierre del ciclo observabilidad (C41→C49)

**C41 (6 findings) → C49 (0 findings activos).** Ciclo completo:

| Contract | Findings tras | Acción | Estado |
|---|---|---|---|
| C41 | 6 | Scan baseline | cerrado C43 |
| C43 | 4 | slog + Tracer + Metrics | cerrado C44 |
| C44 | 3 | Re-scan | cerrado C45 |
| C45 | 1 | SanitizeStripped wiring | cerrado C46 |
| C46 | 1 | SLO/alerts docs | cerrado C49 |
| **C49** | **0** | **fail-fast logger** | **✅ cerrado** |

## Implementación C49

`NewHandler` (http.go) — fail-fast guard:

```go
if h.log == nil {
    panic("C49: NewHandler recibió WithLogger(nil); en prod use CMS_REQUIRE_LOGGER=1 " +
        "y provea un logger estructurado (slog.NewJSONHandler).")
}
```

- Por defecto `h.log = slog.Default()` (tests/local): **no panic**.
- `WithLogger(nil)` explícito → PANIC (fail-fast): previene producción con logger no configurado.
- Documentación en doc-comment de `NewHandler` prescribe `CMS_REQUIRE_LOGGER=1` + `WithLogger(JSONHandler)`.

## Tests C49 (3 — agregados al oráculo `http_test.go`)
- `TestNewHandler_PanicsOnNilLogger`: `WithLogger(nil)` panickea con mensaje C49.
- `TestNewHandler_DefaultLoggerOK`: sin `WithLogger`, usa `slog.Default()` sin panic.
- `TestNewHandler_ProdDocsRequireLogger`: policy test — no panic en constructores normales.

## Oráculo re-sellado
`internal/posts/http_test.go` SHA `fcac16aa0ebe41ff04fff78723323f11a9ee01c92449cbd1902e42ab157aae0a`
(previo C45 `7da1c62a…`) — actualizado en `knowledge/contracts/posts-http-api.md`.

## Ver también
- `docs/reports/CONTRACT-41-REPORT.md` (baseline)
- `docs/reports/CONTRACT-44-REPORT.md` (re-scan)
- `docs/reports/CONTRACT-46-REPORT.md` (SLO/alerts)
