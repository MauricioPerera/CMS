# Contrato 38 — Posts list/search

Prerrequisitos: C1 (schema `posts`), C36 (CRUD), C37 (render). El read path público filtra
a `published`; `post.render`/`content.filter` ya están operativos en C37.

> Capa: contrato de ejecución. Task contract CCDD: `knowledge/contracts/posts-list.md`.
> NOTE: re-sella el oráculo de C36/C37 (`tests_sha256` en `posts-crud.md`) — ver
> `docs/reports/CONTRACT-36-REPORT.md` → sección "Re-sellado del oráculo (C37)".

## T1-POSTS-LIST — paginado + filtro + search

OBJETIVO: `internal/posts/posts.go` — `List(ListInput)` / `ListRendered(ctx, ListInput)`.

- `ListInput.Status == ""` → `WHERE status='published'` (read path público).
- `ListInput.Status != ""` → `WHERE status='<value>'` (admin).
- `Query` non-empty → `AND (slug LIKE ? OR title LIKE ?)`.
- `LIMIT/OFFSET` paginado; `Limit` clampeado `[1, 100]` (default 20).
- `ListRendered` ejecuta `post.render`→`content.filter` (C37) sobre cada item.

## T1-RESELLO-ORACULO

`internal/posts/posts_test.go` se re-sella (mismo package `internal/posts`): 7 tests C36 +
7 tests C37 preservados + tests C38 nuevos. SHA en `posts-crud.md` actualizado.

## T1-TESTS — oráculo congelado (re-sellado)

Verifica:
- `List` con tabla vacía → `Items=[]`, `Total=0`, `HasMore=false`.
- `List` público (Status="") → solo `published`.
- `List` paginado: `Offset`/`Limit`/`HasMore` consistentes.
- `List` con `Query` → search en slug/title.
- `ListRendered` → `HTML` renderizado por cada item (hook o fallback).

## Criterios de aceptación

- [ ] `python scripts/validate_contracts.py knowledge/contracts` exit 0.
- [ ] `go build ./...` exit 0.
- [ ] `go vet ./...` limpio.
- [ ] `go test ./internal/posts/... -v` → N/N OK.
- [ ] `python scripts/preflight.py` → 19/19.
- [ ] `python -m unittest discover -s tests -p "test_*.py"` → 744/744 (intocado).

## Restricciones

### Tocar SOLO

- `internal/posts/posts.go`, `internal/posts/posts_test.go` (re-sellado),
  `knowledge/contracts/posts-crud.md` (tests_sha256), `docs/reports/CONTRACT-36-REPORT.md`
  (documentar re-sellado), `knowledge/data_models/posts_list.md` (nuevo OKF node).
- No tocar: `internal/hooks/*`, `internal/db/*`, `db/migrations/*`, contratos/specs C1-C37.

- ABORTAR SI: read path público devuelve drafts, `Limit` clamp falla, o `ListRendered` no
  ejecuta hooks por item.

## Backlinks OKF

- Posts CRUD: `knowledge/data_models/posts_crud.md`.
- Posts render: `knowledge/data_models/posts_render.md`.
- Posts list/search: `knowledge/data_models/posts_list.md` (nuevo).
