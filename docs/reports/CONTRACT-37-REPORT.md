# CONTRACT-37 — Posts render (Markdown→HTML + hooks) — REPORT

Fecha: 2026-08-26
Spec: `specs/CONTRACT-37-posts-render.md`

## Resumen ejecutivo

| Criterio | Verdicto | Evidencia |
|---|---|---|
| Validador de contratos | ✅ exit 0, 0 errores en 38 archivo(s) | `python scripts/validate_contracts.py knowledge/contracts` |
| Validador de specs | ✅ exit 0, 0 errores en 38 archivo(s) | `python scripts/validate_specs.py` |
| `test_command` posts-render | ✅ `go test ./internal/posts/... -v` 14/14 (7 C36 + 7 C37) | `validate_test_commands.py` → PASS |
| OKF | ✅ nodo `posts_render.md` válido, enlazado desde `index.md` | `python scripts/validate_okf.py knowledge` → PASS |
| Go build | ✅ `go build ./...` exit 0 (CGO_ENABLED=1) | corrida directa |
| Go vet | ✅ `go vet ./...` limpio | corrida directa |
| Go tests | ✅ 14/14 PASS (db 5/5, hooks 12/12, posts 14/14) | `go test ./internal/...` |
| Preflight | ✅ **19/19** gates | `python scripts/preflight.py` |
| KDD python suite | ✅ **744/744** OK | `python -m unittest discover -s tests -p test_*.py` |

## Entregado

**Task contract CCDD**: `knowledge/contracts/posts-render.md` — `target: internal/posts/render.go`,
`test_command: "go test ./internal/posts/... -v"`, SHA256 oráculo `6d72f54a…` (re-sellado C36).

**Implementación**: `internal/posts/render.go` — `RenderedPost`, `GetRendered`,
`GetBySlugRendered`, `renderMarkdown` (sintaxis mínima: headers, bold, italic, code, listas).
Cadena de hooks: `post.render` → `content.filter` (fallback Go cuando no hay hooks).

**Oráculo congelado** (14 tests en `internal/posts/posts_test.go`, PM-frozen): 7 C36 preservados
+ 7 C37. Re-sellado documentado en `docs/reports/CONTRACT-36-REPORT.md`.

## Hallazgos / decisiones

1. **Backward-compat preservada**: `Get`/`GetBySlug` (C36) retornan `Content` raw sin ejecutar
   hooks de render. `GetRendered`/`GetBySlugRendered` son métodos nuevos que añaden el read
   path renderizado. Test `TestGet_StillReturnsRawContent` garatona esto.
2. **Fallback `renderMarkdown`**: sintaxis mínima en Go (stdlib `regexp`/`strings`), sin JS ni
   dependencias. No cuelga (no usa QuickJS).
3. **Chain de hooks**: `post.render` (Markdown→HTML) → `content.filter` (sanitización). El
   payload de `post.render` incluye `action:"render"` + `post` (id/slug/title/content/status);
   `content.filter` recibe `{ html }`. Un hook rechazado propaga error con `%w`.
4. **Optionalidad de hooks**: si `reg.Call(point, ...)` retorna error (point no registrado),
   se cae al fallback (no se propaga el error) — el read path nunca falla por falta de hooks.

## Próximo contrato

**C38 — Posts list/search**: listado paginado de posts (`LIMIT/OFFSET`) con search por
`slug`/`title`, filtrando por `status` (solo `published` en read path público).
