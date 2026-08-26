# CONTRACT-36 — Posts CRUD — REPORT

Fecha: 2026-08-26
Spec: `specs/CONTRACT-36-posts-crud.md`

## Resumen ejecutivo

| Criterio | Verdicto | Evidencia |
|---|---|---|
| Validador de contratos | ✅ exit 0, 0 errores en 37 archivo(s) | `python scripts/validate_contracts.py knowledge/contracts` |
| Validador de specs | ✅ exit 0, 0 errores en 37 archivo(s) | `python scripts/validate_specs.py` |
| `test_command` posts-crud | ✅ `go test ./internal/posts/... -v` 7/7 | `validate_test_commands.py` → PASS |
| OKF | ✅ nuevo nodo `posts_crud.md` válido, enlazado desde `index.md` | `python scripts/validate_okf.py knowledge` → PASS |
| Go build | ✅ `go build ./...` exit 0 (CGO_ENABLED=1) | corrida directa |
| Go vet | ✅ `go vet ./...` limpio | corrida directa |
| Go tests (race) | ✅ 7/7 PASS con `-race`, 0 data races | `go test -race -timeout 30s` |
| Preflight | ✅ **19/19** gates | `python scripts/preflight.py` |
| KDD python suite | ✅ **744/744** OK | `python -m unittest discover -s tests -p test_*.py` |

## Entregado

**Task contract CCDD**: `knowledge/contracts/posts-crud.md` — `target: internal/posts/posts.go`,
`test_command: "go test ./internal/posts/... -v"`, SHA256 oráculo
`6d72f54a4688ade36feeb61bac2675d421043f35517f43157d218f3fab9b3197` (re-sellado por C37;
ver sección "Re-sellado del oráculo (C37)" más abajo).

**Implementación**: `internal/posts/posts.go` — `Posts` store con `Create`, `Update`,
`Publish`, `Get`, `GetBySlug` sobre la tabla `posts` (C1). Antes de cada escritura ejecuta
el hook `post.validate` (C2) con payload `{ action, post }`; un `{ ok:false, error }`
aborta la operación sin escribir.

**Oráculo congelado**: `internal/posts/posts_test.go` (7 tests, PM-frozen).

## Hallazgos / decisiones

1. **Driver `file` de golang-migrate**: `internal/db/init.go` importaba `source` pero no
   `_ "github.com/golang-migrate/migrate/v4/source/file"` → `source.Open("file://...")` fallaba
   con `unknown driver 'file'`. Fix: agregar el import en `init.go` (pertenece al package db,
   no al test). NO toca el oráculo C1.
2. **Path de migraciones en test**: `filepath.Clean("../../db/migrations")` desde `internal/posts/`
   llega a `db/migrations`; `filepath.ToSlash` para el `file://` URL (Windows backslashes rompen
   el parser URL de golang-migrate).
3. **Driver `sqlite`**: el test importa `_ "modernc.org/sqlite"` para registrar el driver
   `database/sql` → `sql.Open("sqlite", ...)`.
4. **Thread-safety**: heredado del C2 — cada `Exec` de hooks crea Runtime+Context efímero
   (QuickJS usa TLS), 0 data races bajo concurrencia.

## Re-sellado del oráculo (C37)

C37 (`posts-render`) evoluciona el read path de `internal/posts/`, lo que requiere añadir
tests nuevos al oráculo `internal/posts/posts_test.go`. El `audit_seals` detecta el cambio
de SHA y lo flagtea como re-sellado:

- SHA C36 original: `443abba5cd29a4760b33e8e8446308d0d8c0ce92b95b5dcd15542a0afa3aae4c`.
- SHA post-C37 (re-sellado): `6d72f54a4688ade36feeb61bac2675d421043f35517f43157d218f3fab9b3197`.
- Los 7 tests C36 (`TestCreate_*`, `TestGet_*`, `TestUpdate_*`, `TestPublish_*`) **siguen
  verdes** (backward-compat preservada: `Get`/`GetBySlug` retornan `Content` raw).
- Añadidos 7 tests C37 (`TestRenderMarkdown_*`, `TestGetRendered_*`,
  `TestContentFilterApplied`, `TestGet_StillReturnsRawContent`).
- `tests_sha256` en `knowledge/contracts/posts-crud.md` actualizado al nuevo hash.
- Justificación del re-sellado: C37 documenta el read path en el mismo package
  `internal/posts` que C36; la alternativa (nuevo package) hubiera duplicado el store y
  roto la coherencia de `Posts`. El re-sellado es visible en diff (política KDD: el cambio
  de oráculo es explícito y documentado, no silencioso).

## Próximo contrato

**C37 — Posts con hook post.render**: integrar `post.render` (Markdown→HTML) en el read path
`Get`/`GetBySlug`, con sanitización vía `content.filter`.
