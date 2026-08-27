# Contract 53 — PATCH /posts/{id} (update parcial)

**Estado:** ✅ VERDE (draft → validated → implemented → verified)
**Fecha:** 2026-08-27
**Ciclo KDD:** Extiende C36 (`posts-crud`) + C43/C47 (`posts-http-api`). Capa 2 re-sellado + Capa 1 implementation.

## Contexto

El CRUD HTTP (C43/C47/C51/C52) tenía Create/Read/Update/Publish/Delete pero **no PATCH**: el `PUT Update` pisa title+content juntos (no permite modificar uno solo). C53 agrega `PATCH /posts/{id}` para update parcial, preservando los campos no enviados.

## Store (Capa 1 — `internal/posts/posts.go`)

```go
type PatchInput struct {
  ID         int64
  Title      string
  Content    string
  HasTitle   bool
  HasContent bool
}

// Patch aplica un update parcial (sólo title y/o content). Hook post.validate action:"patch".
// SET dinámico: solo campos con Has marcado.
func (s *Posts) Patch(ctx hooks.Context, in PatchInput) (Post, error)
```

- `PatchLookup` previo (SELECT id) → 404 si no existe.
- Hook `post.validate` action:"patch" antes del UPDATE (invariante C36).
- SET dinámico con `strings.Builder` + placeholders positionales.

## HTTP (Capa 1 — `internal/posts/http.go`)

```go
// Patch aplica un update parcial (sólo title y/o content). PATCH /posts/{id}.
func (h *Handler) Patch(w http.ResponseWriter, r *http.Request)
```

- Registrado: `h.smux.HandleFunc("PATCH /posts/{id}", h.AuthRequired(h.Patch))`.
- Body JSON decode a `map[string]json.RawMessage` → detecta campos presentes (PATCH semantics, no require todos).
- 400 si `HasTitle==false && HasContent==false` (body sin title/content).
- 200 + Post actualizado; `posts.patch.ok` / `posts.patch.error` / `posts.patch.bad_id` / `posts.patch.bad_body` logs.
- Métricas: `IncRequests`/`IncErrors`/`RecordLatency` + span `posts.patch`.

## Tests (Capa 3 — `internal/posts/http_test.go`)

- `TestHandler_Patch_OK`: crea post, PATCH solo `{"title":"New Title"}` → 200, content preservado.
- `TestHandler_Patch_NoFields`: PATCH `{"slug":"ignored"}` (sin title/content) → 400.

## Verification

```
go test -race -count=1 ./internal/posts/... -run TestHandler_Patch -v
→ PASS: TestHandler_Patch_OK (200, content preserved)
→ PASS: TestHandler_Patch_NoFields (400)

go build ./...  → clean
go vet ./...    → clean
go test -race ./internal/... -timeout 120s
→ ok  gopress/internal/db    (0 races)
→ ok  gopress/internal/hooks  (0 races)
→ ok  gopress/internal/posts  (0 races)

python scripts/validate_contracts.py knowledge/contracts
→ OK: todos los contratos son validos
```

## Resultado

- Nuevo store `Posts.Patch` + `PatchInput` (posts.go).
- Handler HTTP `Patch` + ruta `PATCH /posts/{id}` autenticada.
- +2 tests en `http_test.go`; `posts-http-api.md` re-sellado: SHA `eb6b4698…`; `posts-crud.md` signature extenida con `Patch`.
- **CRUD HTTP REST 100% completo:** C(reate) R(ead) U(pdate) P(atch) P(ublish) D(elete) — todas con auth + `post.validate`.

## Gate

- `validate_contracts.py`: PASS (42 contracts).
- `preflight.py`: 19/19.
- `validate_observability_findings.py`: 0 findings.
- `-race`: 0 data races.
- KDD suite: 744/744 OK.
