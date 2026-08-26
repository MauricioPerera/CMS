# CONTRACT-39-REPORT — content.filter XSS sanitization

Fecha: 2026-08-26
Spec: `specs/CONTRACT-39-content-filter.md`

## Resumen ejecutivo

| Criterio | Veredicto | Evidencia |
|---|---|---|
| `validate_contracts.py` | ✅ exit 0, 0 errores en 40 archivo(s) | `python scripts/validate_contracts.py knowledge/contracts` |
| `go build ./...` | ✅ exit 0 | corrida directa |
| `go vet ./...` | ✅ limpio | corrida directa |
| `go test ./internal/posts/...` | ✅ **20/20 OK** (7 C36 + 7 C37 + 4 C38 + 2 C39) | `go test -v` |
| `preflight.py` | ✅ **19/19** gates | `python scripts/preflight.py` |
| KDD python suite | ✅ **744/744** OK | `python -m unittest discover -s tests -p test_*.py` |

## Entregado

**Task contract CCDD**: `knowledge/contracts/posts-render-filter.md` —
`target: internal/posts/render.go`, `test_command: "go test ./internal/posts/... -v"`,
SHA256 oráculo `e6e09e40835ab86b41b76a9371e608c0b78617be36f4292ea5ee43fd053f9375`.

**Implementación**: `internal/posts/render.go` — función `Sanitize(html string) string`
con 3 regexps (`scriptRe`, `eventHandlerRe`, `jsURLRe`). `filterHook` (C37) reconfigurado:
en todos los fallback paths aplica `Sanitize` (NO retorna HTML sin sanitizar).

## Tests C39

- `TestContentFilter_XSS`: tabla (script-tag, javascript-url, onerror-quoted, onerror-bare,
  safe-html) — esteriza vectores XSS comunes + preserva HTML seguro.
- `TestContentFilter_ListRenderedChainEndToEnd`: post con content `<script>alert(1)…` →
  render → filter (fallback `Sanitize`) → HTML sin `<script>`.

## Re-sellado acumulativo del oráculo

`internal/posts/posts_test.go` se re-selló 4 veces:

| Contract | SHA256 (inicio) | SHA256 (fin) | Tests añadidos |
|---|---|---|---|
| C36 | `0000…` | `443abba5…` | 7 |
| C37 | `443abba5…` | `6d72f54a…` | 7 (render) |
| C38 | `6d72f54a…` | `a0a0107b…` | 4 (list/search) |
| C39 | `a0a0107b…` | `e6e09e40…` | 2 (sanitización) |

`tests_sha256` actualizado en `posts-crud.md`, `posts-render.md`, `posts-list.md`,
`posts-render-filter.md`. Tests C36/C37/C38 preservados y verdes.

## Hallazgos / decisiones

1. **Regexps simples vs parser DOM**: para el vector XSS común (`<script>`, on*-attrs,
   `javascript:`) basta `regexp` stdlib (sin dependencias). Un parser DOM es over-engineering
   para el MVP y rompería el budget (`deps_allowed: ['std']`).
2. **Idempotencia de `Sanitize`**: verificada (aplicar 2× no cambia el output).
3. **`content.filter` como hardening defensivo**: aunque un hook proveedor lo registre, el
   fallback `Sanitize` garantiza defensa en profundidad (defense-in-depth).
4. **`eventHandlerRe`**: regex `\son\w+\s*=[^>\s]*` cubre attrs con/sin comillas, sin
   incluir `>` (evita truncar tags).

## Próximo contrato

**C40 — Posts list con filtrado por author/tag**: añadir columnas `author_id`/`tags` a la
tabla `posts` y permitir filtrar listados por autor o tag (C1 schema migration + C39 read).
