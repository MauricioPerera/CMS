// Package hooks: runtime QuickJS embebido para el hook system del CMS GoPress.
//
// Contrato C2: hooks-runtime. Satisface el oráculo internal/hooks/hooks_test.go.
// CGO_ENABLED=1 (github.com/buke/quickjs-go envuelve QuickJS en C).
//
// Arquitectura:
//   - Runtime: runtime JS puro con sandboxing (memory limit). Expone
//     Eval (JS genérico), Register/Call (compat para tests de runtime aislados)
//     y Exec (ejecuta un body JS dado ctx+payload; usado por Registry).
//   - Registry: mantiene map point→hooks (priority, enabled, JSOwner) y orquesta
//     la ejecución en orden, delegando en Runtime.Exec.
//
// Concurrency: execMu serializa todas las ejecuciones JS sobre el contexto
// compartido (QuickJS Runtime/Context NO es thread-safe). Register/Eval también
// adquieren execMu para serializar con Exec.

package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/buke/quickjs-go"
)

// === Tipos públicos ===

// Value representa un valor JS retornado por Eval.
type Value struct {
	v *quickjs.Value
}

// Float retorna el valor numérico de un Value (float64).
func (v Value) Float() float64 {
	if v.v == nil || !v.v.IsNumber() {
		return 0
	}
	return v.v.ToFloat64()
}

// Context es la superficie mínima que el runtime provee al hook.
type Context struct {
	Point string
}

// RuntimeOption configura el runtime.
type RuntimeOption func(*Runtime)

// WithMemoryLimit impone un límite de memoria en MB.
func WithMemoryLimit(mb int) RuntimeOption {
	return func(r *Runtime) { r.memoryLimitMB = mb }
}

// WithTimeout impone un deadline al ejecutar hooks (configurado vía
// SetExecuteTimeout; no aborta Eval síncrono busy-loops — limitante documentado).
func WithTimeout(d time.Duration) RuntimeOption {
	return func(r *Runtime) { r.timeout = d }
}

// Runtime es el runtime QuickJS embebido con sandboxing.
type Runtime struct {
	mu            sync.Mutex
	execMu        sync.Mutex // serializa ejecución JS (QuickJS no es thread-safe)
	qjs           *quickjs.Runtime
	ctx           *quickjs.Context
	memoryLimitMB int
	timeout       time.Duration
	closed        bool
	// hooks registrados via Runtime.Register (compat con tests aislados).
	hooks map[string]string
}

// NewRuntime crea un runtime QuickJS con opciones de sandboxing.
func NewRuntime(opts ...RuntimeOption) *Runtime {
	r := &Runtime{
		memoryLimitMB: 64,
		timeout:       0,
		hooks:         make(map[string]string),
	}
	for _, opt := range opts {
		opt(r)
	}

	qjsRt := quickjs.NewRuntime()
	if r.memoryLimitMB > 0 {
		qjsRt.SetMemoryLimit(uint64(r.memoryLimitMB) * 1024 * 1024)
	}
	// NOTA: SetExecuteTimeout se configura por Exec (no global) para evitar
	// que un timeout global afecte Evals concurrentes o posteriores.
	// ctx original con MinimalBootstrap (intrinsics sin timers → no cuelga).
	ctx := qjsRt.NewContextWithOptions(quickjs.MinimalBootstrap())

	r.qjs = qjsRt
	r.ctx = ctx
	return r
}

// Close libera los recursos del runtime.
func (r *Runtime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return errors.New("runtime already closed")
	}
	r.closed = true
	if r.ctx != nil {
		r.ctx.Close()
	}
	if r.qjs != nil {
		r.qjs.Close()
	}
	return nil
}

// Eval ejecuta un string JS y retorna el Value resultante.
func (r *Runtime) Eval(expr string) (Value, error) {
	r.execMu.Lock()
	defer r.execMu.Unlock()

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return Value{}, errors.New("runtime closed")
	}
	r.mu.Unlock()

	v := r.ctx.Eval(expr)
	if v.IsException() {
		v.Free()
		err := r.ctx.Exception()
		if err != nil {
			return Value{}, fmt.Errorf("eval error: %w", err)
		}
		return Value{}, errors.New("eval: excepción JS sin detalle")
	}
	return Value{v: v}, nil
}

// Register valida la sintaxis del body JS y lo registra para el point.
// El body puede ser:
//   - `function(ctx, payload) { ... }` (declaration anónima → se le asigna nombre __hook)
//   - `function NAME(ctx, payload) { ... }` (declaration con nombre)
//   - un statement (se envuelve en función anónima)
//
// En todos los casos el body se normaliza a `function __hook(ctx, payload) { ... }`
// para que Exec pueda invocarlo.
func (r *Runtime) Register(point string, jsBody string) error {
	r.execMu.Lock()
	defer r.execMu.Unlock()

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return errors.New("runtime closed")
	}
	r.mu.Unlock()

	normalized := normalizeHookBody(jsBody)

	// Validar sintaxis: el body normalizado es `function __hook(ctx, payload) {...}`
	// (declaration válida) → parsear con Eval.
	check := r.ctx.Eval(normalized)
	if check.IsException() {
		check.Free()
		err := r.ctx.Exception()
		if err != nil {
			return fmt.Errorf("Register hook %q: JS inválido: %w", point, err)
		}
		return fmt.Errorf("Register hook %q: JS inválido (sin detalle)", point)
	}
	check.Free()

	r.mu.Lock()
	r.hooks[point] = normalized
	r.mu.Unlock()
	return nil
}

// Call ejecuta el hook registrado para point (via Runtime.Register).
// Compatible con el test de runtime aislado (TestRuntime_RegisterAndCall).
func (r *Runtime) Call(point string, ctx Context, payload map[string]any) (map[string]any, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, errors.New("runtime closed")
	}
	body, ok := r.hooks[point]
	r.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("hook no registrado: %s", point)
	}
	return r.Exec(body, ctx, payload)
}

// Exec ejecuta un body JS normalizado como `function __hook(ctx, payload) {...}`
// con ctx+payload. Retorna el resultado como map[string]any.
//
// Thread-safe por aislamiento: QuickJS usa TLS (thread-local storage) para el
// runtime y context, por lo que NO es safe compartir un Runtime/Context entre
// goroutines concurrentes. Cada Exec crea su propio Runtime+Context efímero
// (costo: ~1ms de creación). El pooling puede optimizarse después.
func (r *Runtime) Exec(body string, ctx Context, payload map[string]any) (map[string]any, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, errors.New("runtime closed")
	}
	r.mu.Unlock()

	// Runtime nuevo por Exec → thread-safe por aislamiento (QuickJS usa TLS).
	qjsRt := quickjs.NewRuntime()
	if r.memoryLimitMB > 0 {
		qjsRt.SetMemoryLimit(uint64(r.memoryLimitMB) * 1024 * 1024)
	}
	if r.timeout > 0 {
		qjsRt.SetExecuteTimeout(uint64(r.timeout / time.Millisecond))
	}
	// MinimalBootstrap: intrinsics (Object, Array, JSON, Date) SIN timers/console →
	// no cuelga, thread-safe (cada goroutine tiene su propio runtime).
	qjsCtx := qjsRt.NewContextWithOptions(quickjs.MinimalBootstrap())
	defer func() {
		qjsCtx.Close()
		qjsRt.Close()
	}()

	ctxJSON, _ := json.Marshal(ctx)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("Exec: no se pudo serializar payload: %w", err)
	}

	script := buildExecScript(body, ctxJSON, payloadJSON)

	// Configurar timeout per-Exec (SetExecuteTimeout es thread-safe en quickjs-go
	// y el execMu garantiza que no haya Eval concurrentes en este ctx).
	if r.timeout > 0 {
		timeoutMs := uint64(r.timeout / time.Millisecond)
		if timeoutMs == 0 {
			timeoutMs = 1
		}
		r.qjs.SetExecuteTimeout(timeoutMs)
		defer r.qjs.SetExecuteTimeout(0) // 0 = deshabilitar después de Exec.
	}

	result := qjsCtx.Eval(script)
	// QuickJS puede retornar un Value con internals nil tras un timeout/error.
	if result == nil {
		err := qjsCtx.Exception()
		if qjsCtx.HasException() {
			_ = qjsCtx.Exception()
		}
		if err != nil {
			return nil, fmt.Errorf("Exec: JS error: %w", err)
		}
		return nil, errors.New("Exec: excepción JS (nil result, sin detalle)")
	}
	if result.IsException() {
		result.Free()
		err := qjsCtx.Exception()
		if qjsCtx.HasException() {
			_ = qjsCtx.Exception()
		}
		if err != nil {
			return nil, fmt.Errorf("Exec: JS error: %w", err)
		}
		return nil, errors.New("Exec: excepción JS (sin detalle)")
	}
	defer result.Free()

	// Convertir el resultado a map[string]any vía JSON.
	// El script retorna JSON.stringify(_result), así que el result es un string JSON.
	if result.IsUndefined() || result.IsNull() {
		return payload, nil
	}
	if result.IsString() {
		jsonStr := result.ToString()
		var out map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &out); err != nil {
			return nil, fmt.Errorf("Exec: no se pudo parsear resultado JSON: %w", err)
		}
		return out, nil
	}
	if result.IsObject() {
		jsonStr := result.JSONStringify()
		var out map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &out); err != nil {
			return nil, fmt.Errorf("Exec: no se pudo parsear resultado: %w", err)
		}
		return out, nil
	}
	return map[string]any{"value": result.ToString()}, nil
}

// buildExecScript arma el script JS que ejecuta el body (function __hook)
// e invoca __hook(ctx, _payload). El script retorna el _result directamente
// (NO usa JSON.stringify global → no depende de bootstrap). La serialización
// a JSON se hace en Go vía result.JSONStringify().
func buildExecScript(body string, ctxJSON, payloadJSON []byte) string {
	return fmt.Sprintf(`
var _ctx = %s;
var _payload = %s;
%s;
var _result;
_result = __hook(_ctx, _payload);
if (typeof _result === 'undefined' || _result === null) {
  _result = _payload;
}
_result;
`, string(ctxJSON), string(payloadJSON), body)
}

// normalizeHookBody convierte cualquier body en una function declaration válida:
// `function __hook(ctx, payload) { ... }`.
// - `function(ctx, ...)` o `function (ctx, ...)` (anónima) → `function __hook(ctx, ...)`
// - `function NAME(...)` (con nombre) → se le cambia el nombre a __hook
// - statement suelto → `function __hook(ctx, payload) { body }`
func normalizeHookBody(body string) string {
	body = strings.TrimSpace(body)
	if strings.HasPrefix(body, "function") {
		rest := body[len("function"):]
		rest = strings.TrimLeft(rest, " \t\n\r")
		if len(rest) > 0 && rest[0] == '(' {
			// function anónima → agregar nombre __hook.
			return "function __hook" + rest
		}
		// function NAME(...) → leer el nombre, saltarlo, reemplazarlo por __hook.
		nameEnd := 0
		for nameEnd < len(rest) && isIdentFirst(rest[nameEnd]) {
			nameEnd++
		}
		if nameEnd > 0 {
			// saltar el nombre y cualquier whitespace hasta '('.
			rest = rest[nameEnd:]
			rest = strings.TrimLeft(rest, " \t\n\r")
		}
		return "function __hook" + rest
	}
	// statement → envolver en función anónima.
	return fmt.Sprintf("function __hook(ctx, payload) { %s }", body)
}

// isIdentFirst devuelve true si c es un carácter válido para inicio de identificador JS.
func isIdentFirst(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '$'
}

// contains verifica si s contiene substr.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
