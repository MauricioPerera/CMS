---
type: 'Task Contract'
title: 'Hook runtime — QuickJS embebido'
description: 'Runtime QuickJS (github.com/buke/quickjs-go) con memory limit, timeout via interrupt handler, registerHook y Call. Sandbox básico para plugins JS del CMS.'
tags: ['ccdd', 'hooks', 'quickjs', 'runtime', 'sandbox']

task: hooks-runtime
intent: "Proveer un runtime QuickJS embebido con sandboxing básico (memory limit + timeout) que cargue JS, registre hooks por point y los invoque seguro."
target: internal/hooks/runtime.go
signature: |
  type Runtime struct { /* ... */ }
  func NewRuntime(opts ...RuntimeOption) *Runtime
  func (r *Runtime) Eval(expr string) (Value, error)
  func (r *Runtime) Register(point string, jsBody string) error
  func (r *Runtime) Call(point string, ctx Context, payload map[string]any) (map[string]any, error)
  func (r *Runtime) Close() error
  func WithMemoryLimit(mb int) RuntimeOption
  func WithTimeout(d time.Duration) RuntimeOption
test_command: "go test ./internal/hooks/... -v"
budget:
  cyclomatic_max: 12
  nesting_max: 4
  lines_max: 120
  params_max: 4
tests: "internal/hooks/hooks_test.go"
tests_sha256: "5f14c71be0bf8d2c01d9f5b7bcfd64d74aac4880e195050ea88d2c52187dbc2f"
touch_only: ['internal/hooks/runtime.go']
deps_allowed: ['github.com/buke/quickjs-go']
forbids: ['network', 'subprocess', 'llm', 'unsafe']
---

# Contract: hooks-runtime

## Intent
Runtime QuickJS embebido para el hook system del CMS GoPress. Debe proveer:
- `Eval(expr)` para ejecutar JS arbitrario (usado internamente y para tests de smoke).
- `Register(point, jsBody)` que toma el string JS de un hook y lo compila en el contexto del runtime.
- `Call(point, ctx, payload)` que ejecuta el hook registrado, convirtiendo `payload` a JS
  y de vuelta a `map[string]any`.
- Sandboxing: `WithMemoryLimit(n int)` impone `runtime.SetMemoryLimit`; `WithTimeout(d)`
  instala un interrupt handler que aborta hooks que exceden el deadline.

## Interface
```go
type Runtime struct { /* fields */ }

func NewRuntime(opts ...RuntimeOption) *Runtime
func (r *Runtime) Eval(expr string) (Value, error)
func (r *Runtime) Register(point string, jsBody string) error
func (r *Runtime) Call(point string, ctx Context, payload map[string]any) (map[string]any, error)
func (r *Runtime) Close() error

type RuntimeOption func(*Runtime)
func WithMemoryLimit(mb int) RuntimeOption
func WithTimeout(d time.Duration) RuntimeOption
```

## Invariants
- CGO_ENABLED=1 (QuickJS es C; requiere gcc/cc en build y CI).
- `Call` sobre hook no registrado retorna error wrappeado con `%w`.
- `Close` libera el runtime; operaciones posteriores a `Close` retornan error.
- `Value.Float()` retorna `float64` para valores numéricos (test lo verifica).
- Errores de parse JS o runtime JS se wrappean con `%w` (test `TestErrorWrapping`,
  `TestRuntime_EvalInvalidJS`).
- `payload` nil es válido (hook points que no usan payload).

## Examples
- `NewRuntime(WithMemoryLimit(64)).Eval("1 + 1")` → `Value.Float() == 2`.
- `rt.Register("post.validate", jsBody)` → hook disponible para `rt.Call`.
- `NewRuntime(WithTimeout(50ms)).Register("infinite", whileTrue)` → `Call` retorna
  error de timeout (test `TestRuntime_TimeoutConfiguresHandler`). NOTA: QuickJS-go
  `SetExecuteTimeout` es global en el runtime y NO aborta Eval sync busy-loops; el
  test verifica que el timeout se configura (handler instalado), no que mata loops.

## Do / Don't
- DO: usar `github.com/buke/quickjs-go` v0.7.x; mapear `map[string]any` ↔ JS objects
  convirtiendo nil → undefined.
- DO: envolver errores con `%w`.
- DON'T: hardcodear memory limit (proviene de `WithMemoryLimit`).
- DON'T: tocar `internal/hooks/hooks_test.go` (oráculo congelado).
- DON'T: usar `unsafe`.

## Tests
(oráculo congelado SHA256 `5f14c71be0bf8d2c01d9f5b7bcfd64d74aac4880e195050ea88d2c52187dbc2f` en
`internal/hooks/hooks_test.go`. Tests: `TestRuntime_EvalBasicJS`,
`TestRuntime_RegisterAndCall`, `TestRuntime_TimeoutConfiguresHandler`,
`TestRuntime_MemoryLimitPreventsOOM`, `TestRuntime_CallNoHookError`,
`TestRuntime_CloseReleasesMemory`, `TestErrorWrapping`, `TestRuntime_EvalInvalidJS`,
`TestRegistry_PriorityOrder`, `TestRegistry_DisabledHookSkipped`,
`TestRegistry_ConcurrentSafe`, `TestRegistry_UnregisterRemovesHook`,
`TestRegistry_CallBreakOnError`.)

## Restricciones
- Tocar SOLO: `internal/hooks/runtime.go`, `internal/hooks/registry.go`.
- No tocar: `internal/hooks/hooks_test.go` (oráculo PM-frozen).

## Constraints
- PARAR y reportar si: QuickJS no compila en Windows/CI (CGO/ gcc missing), o `Eval`
  de `1+1` ≠ 2, o registry no es thread-safe (panic en `TestRegistry_ConcurrentSafe`).
