# Contrato 39 — content.filter XSS sanitization

Prerrequisitos: C1 (schema), C2 (hooks QuickJS), C36 (posts CRUD), C37 (render chain),
C38 (list/search).

> Capa: contrato de ejecución. Task contract CCDD: `knowledge/contracts/posts-render-filter.md`.

## T1-SANITIZE — fallback defensivo de content.filter

OBJETIVO: `internal/posts/render.go` — función `Sanitize(html string) string` que estriple
vectores XSS (script tags, on*-attrs, javascript: URLs) como fallback defensivo de
`content.filter`. Se aplica cuando no hay hook registrado o cuando el hook no muta el HTML.

- `Sanitize`: regex `scriptRe` (dotall, case-insensitive), `eventHandlerRe` (on*-attrs),
  `jsURLRe` (javascript: → unsafe:).
- `filterHook` (C37) modificado: fallback → `Sanitize` (no retorna HTML sin sanitizar).
- Backward-compat: `Get`/`GetBySlug` preservan Content raw; render sólo en read path.

## T1-RESELLO-ORACULO

C39 re-sella `internal/posts/posts_test.go` (misma orquestación compartida desde C36).
2 tests C39 añadidos (`TestContentFilter_XSS`, `TestContentFilter_ListRenderedChainEndToEnd`);
20/20 preservados. SHA → `e6e09e40…`.

## T1-TESTS — oráculo congelado (re-sellado)

Verifica:
- `Sanitize("<script>")` elimina script; `javascript:` neutralizado; on*-attrs borrados.
- `ListRendered` con content inseguro → HTML sin `<script>` (chain fallback).
- Idempotencia de `Sanitize`.

## Criterios de aceptación

- [ ] `python scripts/validate_contracts.py knowledge/contracts` exit 0.
- [ ] `go build ./...` exit 0.
- [ ] `go vet ./...` limpio.
- [ ] `go test ./internal/posts/... -v` → 20/20 OK.
- [ ] `python scripts/preflight.py` → 19/19.
- [ ] `python -m unittest discover -s tests -p "test_*.py"` → 744/744.

## Restricciones

### Tocar SOLO

- `internal/posts/render.go`, `internal/posts/posts_test.go` (re-sellado),
  `knowledge/contracts/{posts-crud,posts-render,posts-list,posts-render-filter}.md`
  (tests_sha256), `docs/reports/CONTRACT-39-REPORT.md`, `CHANGELOG.md`.

- No tocar: `internal/hooks/*`, `internal/db/*`, `db/migrations/*`, specs/contratos C1-C38
  salvo re-sellado documentado.
- ABORTAR SI: el read path retorna HTML con `<script>` activo tras `content.filter`.
