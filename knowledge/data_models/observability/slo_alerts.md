---
type: 'Data Model'
title: 'Observability SLO — CMS posts read path'
description: 'SLOs y alertas operacionales (PrometheusRule) sobre el read path posts, derivadas de /metrics expuesto por C43. Cierra obs_no_alerting_readpath.'
tags: ['observability', 'slo', 'alerts', 'prometheus', 'posts', 'metrics']
---

# Observability SLO — CMS posts read path
<!-- knowledge/data_models/observability/slo_alerts.md -->

Nodo KDD (knowledge/data_models) que documenta las SLO/alertas operacionales para el read
path de posts. Este nodo **cierra el residual observacional `obs_no_alerting_readpath`**
identificado en C41/C44 (configuración de infra/Prometheus, fuera del código Go).

## SLO (puntos de datos — derivados de `/metrics` expuesto por C43)

| SLI | Métrica | Threshold | Severidad |
|---|---|---|---|
| HTTP error rate | `posts_requests_total{status="5xx"}` / `posts_requests_total` | > 5% (5m) | P1 |
| P95 latency | histograma `posts_request_duration_ms_bucket` | > 500ms (5m) | P2 |
| Render fallback ratio | `posts_sanitize_stripped` (estímnes de XSS) | spike > 10x baseline (1h) | P3 |
| DB error rate | `posts_errors_total{type="db"}` / requests | > 3% (5m) | P1 |

## PrometheusRule (ejemplo)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: gopress-posts-readpath
  labels:
    severity: warning
spec:
  groups:
  - name: posts_readpath
    rules:
    - alert: PostsHighErrorRate
      expr: |
        sum(rate(posts_requests_total{status=~"5.."}[5m]))
        /
        sum(rate(posts_requests_total[5m])) > 0.05
      for: 2m
      labels: { severity: p1 }
      annotations:
        summary: "CMS posts read path error rate > 5%"
        description: "{{ $value | printf \"%.2f\" }}% errors en /posts en 5m."

    - alert: PostsHighLatencyP95
      expr: |
        histogram_quantile(0.95,
          sum(rate(posts_request_duration_ms_bucket[5m])) by (le)) > 500
      for: 3m
      labels: { severity: p2 }
      annotations:
        summary: "CMS posts P95 latency > 500ms"
        description: "p95 = {{ $value }}ms en 5m."

    - alert: PostsXSSSpike
      expr: |
        rate(posts_sanitize_stripped[1h])
        / on() group_left
        rate(posts_requests_total[1h]) > 0.10
      for: 10m
      labels: { severity: p3 }
      annotations:
        summary: "Spike in XSS strips / sanitize attempts"
```

## Relación con findings.json

Este nodo **remeda** el residual `obs_no_alerting_readpath-residual-003` del
`findings.json` C44/C45: el gap no era de código sino de configuración de infra, y aquí
se documenta la regla de alerta concreta. Tras aplicar este PrometheusRule en el
cluster, el finding puede marcarse como **remediado** y el ciclo observabilidad
C41→C43→C44→C45→C46 cierra completo.

## Ver también
- `docs/reports/CONTRACT-44-REPORT.md` (cierre parcial)
- `docs/reports/CONTRACT-45-REPORT.md` (wiring SanitizeStripped)
- `docs/reports/CONTRACT-46-REPORT.md` (este nodo — cierre completo)
