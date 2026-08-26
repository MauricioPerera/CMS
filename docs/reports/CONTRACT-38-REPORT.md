# CONTRACT-38 — Posts list/search — REPORT

Fecha: 2026-08-26
Spec: `specs/CONTRACT-38-posts-list.md`

## Resumen ejecutivo

| Criterio | Veredicto | Evidencia |
|---|---|---|
| Validador de contratos | ✅ exit 0, 0 errores en 39 archivo(s) | `python scripts/validate_contracts.py knowledge/contracts` |
| Validador de specs | ✅ exit 0, 0 errores en 39 archivo(s) | `python scripts/validate_specs.py` |
| `test_command` posts-list | ✅ `go test ./internal/posts/... -v` 18/18 (7 C36 + 7 C37 + 4 C38) | `validate_test_commands.py` → PASS |
| OKF | ✅ nodo `posts_list.md` válido, enlazado desde `index.md` | `python scripts/validate_okf.py knowledge` → PASS |
| Go build | ✅ `go build ./...` exit 0 (CGO_ENABLED=1) | corrida directa |
| Go vet | ✅ `go vet ./...` limpio | corrida directa |
| Go tests | ✅ 18/18 posts + 17 db/hooks OK | `go test ./internal/...` |
| Preflight | ✅ **19/19** gates | `python scripts/preflight.py` |
| KDD python suite | ✅ **744/744** OK | `python -m unittest discover -s tests -p test_*.py` |

## Entregado

**Task contract CCDD**: `knowledge/contracts/posts-list.md` — `target: internal/posts/posts.go`,
`test_command: "go test ./internal/posts/... -v"`, SHA256 oráculo `a0a0107b…`.

**Implementación**: `internal/posts/posts.go` — `ListInput`, `ListResult`,
`RenderedListResult`, `List(ListInput)`, `ListRendered(ctx, ListInput)`. Read path público
filtra a `published` (Status=""`); search por slug/title (LIKE); clamping de `Limit`.
`ListRendered` aplica el chain `post.render`→`content.filter` (C37) sobre cada item.

**Oráculo congelado**: `internal/posts/posts_test.go` — 18 tests (7 C36 + 7 C37 + 4 C38).
Re-sellado de C36/C37 (mismo oráculo compartido).

## Hallazgos / decisiones

1. **Re-sellado acumulativo del oráculo**: `internal/posts/posts_test.go` se re-selló 3 veces
   (C36 → C37 → C38) porque cada contrato evoluciona el mismo package. El SHA final
   (`a0a0107b…`) refleja 18 tests. Documentado en `docs/reports/CONTRACT-36-REPORT.md`
   (re-sellado C37) y `CONTRACT-37-REPORT.md` (re-sellado C38).
2. **`modernc.org/sqlite` y `[]`any`: el driver no soporta pasar un slice `[]interface{}`
   como argumento de `QueryRow` — requiere spread `args...`. Fix en `List` count query.
3. **Read path público**: `Status == ""` → `WHERE status='published'`; otro valor → filtro
   exacto (admin). Test `TestList_PublishedOnly` verifica que drafts no aparecen.
4. **`HasMore`**: `(offset + len(items)) < Total` — consistente con paginación real.

## Próximo contrato

**C39 — Posts con hook `content.filter` end-to-end**: testear el chain completo
`post.render`→`content.filter` sobre listados paginados (sanitización de XSS en `content`
post-render), con un hook `content.filter` que estrileña HTML.
