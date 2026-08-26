# Contrato 35 — Hook system (QuickJS plugins)

Spec de ejecución (nivel proyecto). La tarea de código (T1-HOOKS-RUNTIME,
T1-HOOKS-REGISTRY) lleva task contracts CCDD en `knowledge/contracts/hooks-runtime.md`
y `knowledge/contracts/hooks-registry.md`; el oráculo es autorado por el PM y congelado
antes de delegar.

> Capa: contrato de ejecución. Motor JS embebido vía `github.com/buke/quickjs-go`
> (QuickJS C, build con CGO; soporte para memory limit + interrupt handler).
> CGO_ENABLED=1 requerido.

## T1-HOOKS-RUNTIME — runtime QuickJS embebido

OBJETIVO: `internal/hooks/runtime.go` — envoltura mínima sobre QuickJS que:
- Carga un archivo `.js` o un string en memoria.
- Expose `registerHook(point, fn)` desde Go → JS: el hook JS recibe `(ctx, payload)`
  y retorna `(result, error)`.
- `Call(point, payload) → (result, error)` ejecuta todos los hooks registrados en un
  point en orden, con `break-on-error` configurable.
- Sandboxing básico: límite de memoria (default 64MB), timeout (default 5s) vía
  `SetMemoryLimit`, `SetInterruptHandler`.

## T1-HOOKS-REGISTRY — registry de hooks por point

OBJETIVO: `internal/hooks/registry.go` — registry thread-safe que mapea
`point(string) → []Hook`. Cada `Hook` tiene: nombre, runtime (referencia a QuickJS),
prioridad (default 100), enabled (bool).

**Hook points del CMS (contrato con el core):**
- `post.validate` — antes de guardar un post; hook puede rejectar con error.
- `post.render` — transforma Markdown → HTML; hook puede mutar el HTML de salida.
- `user.authenticate` — middleware de auth; hook puede rejectar login.
- `content.filter` — filtra/sanitiza HTML de `content` post-render.

## T1-TESTS — oráculo congelado

OBJETIVO: `internal/hooks/hooks_test.go` (tests autorados por el PM, SHA256 sellado en
el task contract). Verifican: runtime carga JS y ejecuta hook, registry registra y
ejecuta en orden de prioridad, timeout mata hooks infinitos, memory limit previene OOM.

## Criterios de aceptación

- [ ] `python scripts/validate_contracts.py knowledge/contracts` exit 0.
- [ ] `go build ./...` exit 0 (CGO_ENABLED=1).
- [ ] `go test ./internal/hooks/... -v` → N/N OK (runtime + registry + sandbox).
- [ ] `python scripts/preflight.py` → 19/19.
- [ ] Python suite KDD (`tests/`) verde (intocada).

## Restricciones

### Tocar SOLO

- T1-HOOKS-* tocan SOLO: `internal/hooks/{runtime.go, registry.go}` y `go.mod`/`go.sum`
  (para agregar `github.com/buke/quickjs-go`).
- T1-TESTS NO se toca (oráculo congelado, `FM_TOUCH_TESTS`): `internal/hooks/hooks_test.go`.
- Nodos OKF (`knowledge/data_models/hook_points.md`) NO se tocan (reference, no editable
  por el implementador de código).

- `deps_allowed: ['github.com/buke/quickjs-go']` (única dep nueva; versiona en go.mod).
- No network, no subprocess, no LLM.
- CGO_ENABLED=1 requerido (QuickJS envuelve C; gcc/cc en build y CI).
- ABORTAR SI: QuickJS no compila en Windows/CI; o el runtime no ejecuta JS básico
  (`1+1`) en < 50ms.

## Backlinks OKF

- Data model: `knowledge/data_models/hook_points.md` (hook points + payload contract).
- Data model schema baseline: `knowledge/data_models/cms_schema.md` (hook points referencian tablas).
