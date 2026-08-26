---
type: 'Task Contract'
title: 'Hook registry — dispatch por point + prioridad'
description: 'Registry thread-safe: mapea hook points → []Hook con prioridad, enabled, y Call que ejecuta en orden rompiendo ante error configurable.'
tags: ['ccdd', 'hooks', 'registry', 'dispatch', 'priority']

task: hooks-registry
intent: "Proveer un registry concurrentemente seguro que registre hooks por point con prioridad y los dispatch en orden correcto."
target: internal/hooks/registry.go
signature: |
  type Hook struct { Name string; Point string; Priority int; Enabled bool; Runtime *Runtime; JSOwner string }
  func NewRegistry(rt *Runtime) *Registry
  func (r *Registry) Register(point, name string, priority int, enabled bool, jsBody string) error
  func (r *Registry) Unregister(point, name string) error
  func (r *Registry) Call(point string, payload map[string]any) (map[string]any, error)
  func (r *Registry) List(point string) []Hook
test_command: "go test ./internal/hooks/... -v"
budget:
  cyclomatic_max: 10
  nesting_max: 4
  lines_max: 100
  params_max: 5
tests: "internal/hooks/hooks_test.go"
tests_sha256: "5f14c71be0bf8d2c01d9f5b7bcfd64d74aac4880e195050ea88d2c52187dbc2f"
touch_only: ['internal/hooks/registry.go']
deps_allowed: ['github.com/buke/quickjs-go']
forbids: ['network', 'subprocess', 'llm']
---

# Contract: hooks-registry

## Intent
Registry thread-safe que mapea hook points → hooks. Cada hook tiene prioridad
(lower = runs first), flag enabled, y referencia al runtime QuickJS. `Call` ejecuta
hooks en orden de prioridad, propagando el payload acumulado; con break-on-error (default
true), un hook que falle detiene la cadena.

## Interface
```go
type Hook struct {
  Name     string
  Point    string
  Priority int     // lower runs first; default 100
  Enabled  bool
  Runtime  *Runtime
  JSOwner  string  // body JS compilado (interno)
}

func NewRegistry(rt *Runtime) *Registry
func (r *Registry) Register(point, name string, priority int, enabled bool, jsBody string) error
func (r *Registry) Unregister(point, name string) error
func (r *Registry) Call(point string, payload map[string]any) (map[string]any, error)
func (r *Registry) List(point string) []Hook
```

## Invariants
- Registry usa mutex (thread-safe) — test `TestRegistry_ConcurrentSafe`.
- Hooks ejecutados en orden ASCENDENTE de prioridad.
- Hooks `Enabled=false` son saltados — test `TestRegistry_DisabledHookSkipped`.
- `Call` con break-on-error: primer error detiene la cadena y lo propaga —
  test `TestRegistry_CallBreakOnError`.
- `Unregister(point, name)` remueve el hook; `Call` posterior sobre ese point
  retorna error si no quedan hooks — test `TestRegistry_UnregisterRemovesHook`.
- `Register` con mismo `point+name` sobreescribe (no duplica).

## Examples
- `registry.Register("content.filter", "lower", 10, true, js)` → hook con prioridad 10.
- `registry.Register("content.filter", "reverse", 50, true, js)` → prioridad 50.
- `registry.Call("content.filter", {value:"abc"})` → ejecuta lower(10)→reverse(50)→upper(100).

## Do / Don't
- DO: ordenar hooks por prioridad antes de ejecutar.
- DO: usar mutex para acceso concurrente al map de hooks.
- DON'T: tocar `internal/hooks/hooks_test.go` (oráculo congelado).
- DON'T: hardcodear hook points (el registry es genérico; los points vienen del caller).

## Restricciones
- Tocar SOLO: `internal/hooks/registry.go`, `internal/hooks/runtime.go`.
- No tocar: `internal/hooks/hooks_test.go` (oráculo PM-frozen).

## Constraints
- PARAR y reportar si: registry no es thread-safe (race en tests concurrentes), o
  hooks no se ejecutan en orden de prioridad, o break-on-error no funciona.
- Comparte `Runtime` con `hooks-runtime.md` (misma dep `github.com/buke/quickjs-go`).

## Tests
(oráculo congelado SHA256 `5f14c71be0bf8d2c01d9f5b7bcfd64d74aac4880e195050ea88d2c52187dbc2f` en
`internal/hooks/hooks_test.go`. Tests: `TestRegistry_PriorityOrder`,
`TestRegistry_DisabledHookSkipped`, `TestRegistry_ConcurrentSafe`,
`TestRegistry_UnregisterRemovesHook`, `TestRegistry_CallBreakOnError`.)
