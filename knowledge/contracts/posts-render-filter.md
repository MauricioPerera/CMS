---
type: 'Task Contract'
title: 'content.filter XSS sanitization (fallback defensivo)'
description: 'Sanitize: fallback Go de content.filter que elimina script tags, event handlers on* y javascript: URLs. Garantiza que el read path nunca sirva HTML ejecutable aunque post.render produzca markup inseguro.'
tags: ['ccdd', 'posts', 'render', 'xss', 'sanitization', 'content-filter', 'cms']

task: content-filter-sanitize
intent: "Proveer un fallback defensivo de `content.filter` en Go (sin JS) que estrile Xe los vectores XSS clásicos, aplicándose tanto cuando no hay hook registrado como como hardening tras un hook que no mutate."
target: internal/posts/render.go
signature: |
  func Sanitize(html string) string
test_command: "go test ./internal/posts/... -v"
budget:
  cyclomatic_max: 10
  nesting_max: 3
  lines_max: 40
  params_max: 1
tests: "internal/posts/posts_test.go"
tests_sha256: "e6e09e40835ab86b41b76a9371e608c0b78617be36f4292ea5ee43fd053f9375"
touch_only: ['internal/posts/render.go']
deps_allowed: ['std']
forbids: ['network', 'subprocess', 'llm', 'unsafe']
---

# Contract: content-filter-sanitize

## Intent
`Sanitize` es el **fallback defensivo** de `content.filter` (C37): elimina vectores XSS
clásicos sobre el HTML producido por `post.render` (hook JS o fallback Markdown). Se aplica
(a) cuando no hay registry/runtime, (b) cuando `reg.Call` falla (point no registrado), y
(c) cuando un hook registrado retorna `{ ok:false }` o HTML vacío. **Nunca retorna HTML
sin sanitizar.**

## Interface
```go
// Sanitize estripea vectores XSS sin re-ordenar HTML.
func Sanitize(html string) string
```

## Reglas de sanitización (MUST)
- **`scriptRe`**: elimina `<script>...</script>` (case-insensitive, dotall).
- **`eventHandlerRe`**: elimina atributos `on*=` (onclick, onerror, onload, ...).
- **`jsURLRe`**: neutraliza `javascript:` → `unsafe:` en href/src.
- NO usar dependencias externas (stdlib `regexp` únicamente).

## Examples
- `Sanitize("<script>alert(1)</script>")` → `""`.
- `Sanitize("<a href='javascript:alert(1)'>x</a>")` → `"<a href='unsafe:alert(1)'>x</a>"`.
- `Sanitize("<img src=x onerror=alert(1)>")` → `"<img src=x>"`.
- `Sanitize("<p>safe</p>")` → `"<p>safe</p>"` (sin cambios).

## Invariants
- `Sanitize` es **idempotente**: aplicar dos veces no cambia el resultado.
- `Sanitize` es **pura**: sin side-effects, solo estrippa markup.
- El read path (`GetRendered`/`GetBySlugRendered`/`ListRendered`) **nunca** retorna HTML con
  `<script>` activo tras `content.filter`, aunque el hook lo produzca.
- Si `content.filter` retorna `{ ok:false }` → se propaga error (NO sanitiza).

## Do / Don't
- DO: aplicar `Sanitize` en todos los fallback paths de `content.filter` (nil registry,
  point no registrado, hook que no muta).
- DO: testear subcases (script tags, on*-attrs, javascript: URLs, safe HTML).
- DON'T: usar un parser DOM (no hay dependencias; `regexp` es suficiente para el vector
  común).
- DON'T: escapar todo el body (roto UX; sólo estripea vectores XSS).

## Tests
- `TestContentFilter_XSS`: tabla con script-tag, javascript-url, onerror attrs (quoted +
  bare), safe-html.
- `TestContentFilter_ListRenderedChainEndToEnd`: post con content inseguro → render →
  filter (fallback) → HTML sin `<script>`.

## Constraints
### Tocar SOLO
- `internal/posts/render.go` (añadir `Sanitize` + regexps), `internal/posts/posts_test.go`
  (tests C39, re-sellado), contratos que referencian el oráculo compartido
  (`posts-crud.md`, `posts-render.md`, `posts-list.md`, `posts-render-filter.md`),
  `docs/reports/CONTRACT-39-REPORT.md`.

- No tocar: `internal/hooks/*`, `internal/db/*`, `db/migrations/*`.
- ABORTAR SI: el read path retorna HTML con `<script>` activo tras `content.filter`, o
  `Sanitize` no es idempotente, o el chain end-to-end permite markup inseguro.
- PARAR y reportar si: `TestContentFilter_XSS` falla algún subcase, o
  `TestContentFilter_ListRenderedChainEndToEnd` detecta `<script>` en el HTML resultante.
