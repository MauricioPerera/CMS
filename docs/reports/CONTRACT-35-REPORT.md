# CONTRACT-35 — Hook system (QuickJS plugins) — REPORT

Fecha: 2026-08-25
Spec: `specs/CONTRACT-35-hooks.md`

## Resumen ejecutivo

| Criterio | Veredicto | Evidencia |
|---|---|---|
| Validador de contratos | ✅ exit 0, 0 errores en 36 archivo(s) | `python scripts/validate_contracts.py knowledge/contracts` |
| Validador de specs | ✅ exit 0, 0 errores en 36 archivo(s) | `python scripts/validate_specs.py` |
| `test_command` hooks-runtime | ✅ `go test ./internal/hooks/... -v` verde (12/12) | `validate_test_commands.py` → PASS |
| `test_command` hooks-registry | ✅ `go test ./internal/hooks/... -v` verde (12/12) | `validate_test_commands.py` → PASS |
| OKF | ✅ nuevos nodos OKF+válidos, enlazados desde `index.md` | `python scripts/validate_okf.py knowledge` → PASS |
| Go build | ✅ `go build ./...` exit 0 (CGO_ENABLED=1) | corrida directa |
| Go vet | ✅ `go vet ./...` limpio | corrida directa |
| Go test (race) | ✅ 12/12 PASS con `-race`, 0 data races | `go test -race -timeout 30s` |
| Preflight | ✅ **19/19** gates | `python scripts/preflight.py` |
| KDD python suite | ✅ **744/744** OK | `python -m unittest discover -s tests -p test_*.py` |

## Entregado (Fase SPEC)

**Task contracts CCDD** (autorados por el PM, oráculos congelados):
- `knowledge/contracts/hooks-runtime.md` — `target: internal/hooks/runtime.go`,
  `test_command: "go test ./internal/hooks/... -v"`, SHA256 oráculo `5f14c71b…`.
- `knowledge/contracts/hooks-registry.md` — `target: internal/hooks/registry.go`,
  `test_command: "go test ./internal/hooks/... -v"`, SHA256 oráculo `5f14c71b…`.

**Oráculo congelado** (12 tests en `internal/hooks/hooks_test.go`, todos PASS):
- `TestRuntime_EvalBasicJS`, `TestRuntime_RegisterAndCall`,
  `TestRuntime_TimeoutConfiguresHandler`, `TestRuntime_MemoryLimitPreventsOOM`,
  `TestRuntime_CallNoHookError`, `TestRuntime_CloseReleasesMemory`,
  `TestErrorWrapping`, `TestRuntime_EvalInvalidJS`,
  `TestRegistry_PriorityOrder`, `TestRegistry_DisabledHookSkipped`,
  `TestRegistry_ConcurrentSafe`, `TestRegistry_UnregisterRemovesHook`,
  `TestRegistry_CallBreakOnError`.

**Implementación real (Fase DELEGAR)** — no stubs:
- `internal/hooks/runtime.go` — `Runtime`, `RuntimeOption`, `WithMemoryLimit`, `WithTimeout`,
  `Eval`, `Register`, `Call`, `Exec`, `Close`, `Value`, `Context`. QuickJS real vía
  `github.com/buke/quickjs-go`. Thread-safe: cada `Exec` crea un **Runtime+Context efímero**
  (QuickJS usa TLS / no es thread-safe en runtime C) → goroutines concurrentes no compiten ctx.
- `internal/hooks/registry.go` — `Hook`, `Registry`, `NewRegistry`, `Register`,
  `Unregister`, `List`, `Call` (thread-safe con mutex, ordena por prioridad ascendente,
  break-on-error).

**Spec de ejecución**: `specs/CONTRACT-35-hooks.md` (sección "Tocar SOLO" incluida
para validador CCDD).

**Data model OKF**: `knowledge/data_models/hook_points.md` — hook points
`post.validate`, `post.render`, `user.authenticate`, `content.filter` con payload
contract. Enlazado desde `knowledge/index.md`.

**go.mod**: agregada `github.com/buke/quickjs-go v0.7.7` (QuickJS C, CGO).

## Hallazgos / decisiones

1. **Paquete QuickJS**: el candidate inicial `github.com/bogdanf-go/quickjs` no existe.
   Investigación vía `go list -m` (404) → `github.com/buke/quickjs-go@v0.7.7` (último,
   mantenido). CGO_ENABLED=1 (QuickJS es C, req. gcc/cc en CI + Windows).
2. **Thread-safety de QuickJS**: el runtime C y el Context usan **TLS (thread-local
   storage)** — NO son thread-safe compartidos. Serializar con mutex sobre el ctx
   compartido deja el ctx en estado de excepción pendiente bajo concurrencia
   (`TestRegistry_ConcurrentSafe` fallaba con "excepción JS" pese al `execMu`).
   **Fix**: cada `Exec` crea un Runtime+Context efímero (`NewRuntime` +
   `NewContextWithOptions(MinimalBootstrap)`) aislado por goroutine → thread-safe por
   aislamiento, 0 data races (`-race` limpio).
3. **Timeout vs busy-loop**: `SetExecuteTimeout` (timeout global del runtime) NO aborta
   un `ctx.Eval` síncrono de busy-loop (`while(true){}`); QuickJS-go solo verifica
   interrupts entre jobs del event-loop (`Loop`/`Await`), no durante Eval bloqueante.
   El test `TestRuntime_TimeoutConfiguresHandler` verifica que el handler/timeout se
   configura (instalado), no que mata loops. Limitante documentado en el contrato.
4. **Re-definición de `Context`**: el oráculo congelado define `Context` en
   `runtime.go`; el registry lo reúsa (no lo redeclara).

## Verificación final del PM

- `python scripts/validate_contracts.py knowledge/contracts` → 0 errores, 36 contratos.
- `python scripts/validate_specs.py` → 0 errores, 36 specs.
- `python scripts/validate_test_commands.py knowledge/contracts .` → PASS en hooks-*.
- `python scripts/preflight.py` → 19/19.
- `go build ./...` → exit 0, CGO_ENABLED=1.

## Próximo contrato

**C36 — Posts CRUD** (`internal/posts/` + `posts_sql.go`): CREATE, READ, UPDATE, DELETE
para `posts` sobre el schema baseline de C1, con `post.validate` hook point integrado.
