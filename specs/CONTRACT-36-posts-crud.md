# Contrato 36 — Posts CRUD

Prerrequisitos: C1 (db-migrations) y C2 (hooks) completados. El schema `posts` existe
en `db/migrations/001_init.up.sql` y el runtime QuickJS + registry funcionan en
`internal/hooks/`. Este contrato añade la capa de acceso a datos para posts con
integración del hook `post.validate`.

> Capa: contrato de ejecución. Task contract CCDD: `knowledge/contracts/posts-crud.md`
> (validado por `scripts/validate_contracts.py`).

## T1-POSTS-CRUD — acceso a datos + hook integration

OBJETIVO: `internal/posts/posts.go` — struct `Posts` con `Create`, `Update`, `Publish`,
`Get`, `GetBySlug` sobre la tabla `posts`. Antes de cada escritura, ejecutar el hook
`post.validate` (payload `{ action, post }`); si retorna `{ ok:false, error }`, abortar
la operación sin escribir.

Hook `post.validate` (payload contract de `knowledge/data_models/hook_points.md`):
- `Create` → payload `{ action:"create", post:{ slug, title, content, status:"draft" } }`.
- `Update` → payload `{ action:"update", post:{ id, title, content, status } }`.
- `Publish` → payload `{ action:"publish", post:{ id, title, content, status:"published" } }`.

## T1-TESTS — oráculo congelado

OBJETIVO: `internal/posts/posts_test.go`. Verifica:
- `Create` exitoso → Post con `Status="draft"`, `ID` autoincrement, timestamps parseados.
- `Create` con slug duplicado → error wrappeado (`UNIQUE` violation).
- `post.validate` rechazando → `Create`/`Update`/`Publish` falla sin escribir (tabla
  inalterada).
- `Get(id)` / `GetBySlug(slug)` → read-only (no disparan hook).
- `Publish` → `Status="published"`.
- `Update` → `title`/`content` mutados, timestamps refrescados.

## Criterios de aceptación

- [ ] `python scripts/validate_contracts.py knowledge/contracts` exit 0.
- [ ] `python -m unittest discover -s tests -p "test_*.py"` verde (744/744).
- [ ] `go build ./...` exit 0 (CGO_ENABLED=1).
- [ ] `go vet ./internal/posts/...` limpio.
- [ ] `go test ./internal/posts/... -v` → N/N OK.
- [ ] `python scripts/preflight.py` → 19/19.

## Restricciones

### Tocar SOLO

- `internal/posts/posts.go` (implementación) y `internal/posts/posts_test.go` (oráculo
  PM-frozen — NO modificable por el implementador de código).
- `internal/posts/db_test.go` (fixture de test, crea DB en memoria + aplica migraciones).
- OKF node novedo: `knowledge/data_models/posts_crud.md` (referencia desde `index.md`).

- No tocar: `internal/hooks/*`, `internal/db/*`, `db/migrations/*`, contratos C1/C2.
- `deps_allowed`: `modernc.org/sqlite`, `github.com/buke/quickjs-go` (heredado).
- CGO_ENABLED=1 (heredado de C2).
- ABORTAR SI: `modernc.org/sqlite` no compila, o el hook `post.validate` no se ejecuta
  antes de INSERT (verificado con tabla inmutable tras reject), o un hook que rechaza no
  aborta la operación (test `TestCreate_HookRejectAborts` asertiva tabla = 0 filas).

## Backlinks OKF

- Schema baseline: `knowledge/data_models/cms_schema.md`.
- Hook points: `knowledge/data_models/hook_points.md` (`post.validate`).
- Data model posts CRUD: `knowledge/data_models/posts_crud.md` (nuevo).
