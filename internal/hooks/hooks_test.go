// Tests del hook system (Contrato C2: hooks).
//
// Oráculo congelado del PM (Contrato C2: hooks). El implementador
// NO toca este archivo: si lo modifica, el tests_sha256 del task contract
// rompe el gate FM_TESTS_FROZEN.
//
// Ejecuta: go test ./internal/hooks/... -v
// sobre el runtime QuickJS (CGO_ENABLED=1, github.com/buke/quickjs-go).

package hooks

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// HookFunc es la firma que los plugins JS exponen al registry.
type HookFunc func(ctx Context, payload map[string]any) (map[string]any, error)

// NOTA: Context está definido en internal/hooks/runtime.go — este test
// lo reúsa del paquete hooks (no lo redeclara).

func TestRuntime_EvalBasicJS(t *testing.T) {
	rt := NewRuntime(WithMemoryLimit(64))
	defer rt.Close()

	// 1+1 debe evaluar a 2
	val, err := rt.Eval("1 + 1")
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if val.Float() != 2 {
		t.Fatalf("esperaba 2, got %v", val.Float())
	}
}

func TestRuntime_RegisterAndCall(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	// registerHook es global dentro del runtime
	if err := rt.Register("post.validate", `function(ctx, payload) {
		if (payload.title.length < 3) {
			return { ok: false, error: "title too short" };
		}
		return { ok: true };
	}`); err != nil {
		t.Fatalf("Register: %v", err)
	}

	result, err := rt.Call("post.validate", Context{Point: "post.validate"},
		map[string]any{"title": "Hello"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if ok, _ := result["ok"].(bool); !ok {
		t.Fatalf("esperaba ok=true, got %v", result)
	}
}

func TestRegistry_PriorityOrder(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	registry := NewRegistry(rt)

	// hook C (prioridad 10): prepend
	if err := registry.Register("content.filter", "lower", 10, true,
		`function(ctx, payload) { payload.value = payload.value.toLowerCase(); return payload; }`); err != nil {
		t.Fatal(err)
	}
	// hook A (prioridad 100): default
	if err := registry.Register("content.filter", "upper", 100, true,
		`function(ctx, payload) { payload.value = payload.value.toUpperCase(); return payload; }`); err != nil {
		t.Fatal(err)
	}
	// hook B (prioridad 50): middle
	if err := registry.Register("content.filter", "reverse", 50, true,
		`function(ctx, payload) { payload.value = payload.value.split("").reverse().join(""); return payload; }`); err != nil {
		t.Fatal(err)
	}

	result, err := registry.Call("content.filter", map[string]any{"value": "abc"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	got, _ := result["value"].(string)
	// orden: lower (10) → reverse (50) → upper (100)
	// abc → abc → cba → CBA
	if got != "CBA" {
		t.Fatalf("esperaba CBA (lower→reverse→upper), got %q", got)
	}
}

func TestRuntime_TimeoutConfiguresHandler(t *testing.T) {
	// WithTimeout configura SetInterruptHandler + SetExecuteTimeout. Un hook que
	// termina POR DEBAJO del timeout NO debe interrumpirse. (quickjs-go no
	// interrumpe Eval síncrono para busy-loops — limitante documentado en
	// CONTRACT-35-REPORT; aquí verificamos que WithTimeout no causa falsos
	// positives en hooks que terminan a tiempo.)
	rt := NewRuntime(WithTimeout(50 * time.Millisecond))
	defer rt.Close()

	err := rt.Register("loop.short", `function(ctx, payload) {
		var start = Date.now();
		while (Date.now() - start < 5) { /* busy-loop corto < timeout */ }
		return { ok: true, elapsed: true };
	}`)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	result, err := rt.Call("loop.short", Context{Point: "loop.short"}, nil)
	if err != nil {
		t.Fatalf("Call: no deberia interrumpir hook dentro del timeout: %v", err)
	}
	if ok, _ := result["ok"].(bool); !ok {
		t.Fatalf("esperaba ok=true, got %v", result)
	}
}

func TestRuntime_MemoryLimitPreventsOOM(t *testing.T) {
	rt := NewRuntime(WithMemoryLimit(1)) // 1MB — muy restrictivo
	defer rt.Close()

	err := rt.Register("grow", `function(ctx, payload) {
		let arr = [];
		for (let i = 0; i < 100000000; i++) { arr.push(i); }
		return { ok: true };
	}`)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, err = rt.Call("grow", Context{Point: "grow"}, nil)
	if err == nil {
		t.Fatal("esperaba error de memoria/límite, got nil")
	}
}

func TestRegistry_DisabledHookSkipped(t *testing.T) {
	rt := NewRuntime(WithMemoryLimit(64))
	defer rt.Close()
	registry := NewRegistry(rt)

	// hook deshabilitado
	if err := registry.Register("test.skip", "disabled-hook", 100, false,
		`function(ctx, payload) { return { skipped: true }; }`); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("test.skip", "enabled-hook", 50, true,
		`function(ctx, payload) { return payload; }`); err != nil {
		t.Fatal(err)
	}

	result, err := registry.Call("test.skip", map[string]any{"v": 1})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	// solo el hook enabled corre → payload intacto.
	// NOTA: JSON round-trip convierte int → float64.
	v, _ := result["v"].(float64)
	if v != 1 {
		t.Fatalf("esperaba v=1 (hook disabled saltado), got %v", result)
	}
}

func TestRegistry_ConcurrentSafe(t *testing.T) {
	rt := NewRuntime(WithTimeout(2 * time.Second))
	defer rt.Close()
	registry := NewRegistry(rt)

	if err := registry.Register("concurrent", "h", 100, true,
		`function(ctx, payload) { return payload; }`); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := registry.Call("concurrent", map[string]any{"x": 1})
			if err != nil {
				t.Errorf("Call concurrente: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestRuntime_CallNoHookError(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	_, err := rt.Call("nonexistent.point", Context{Point: "nonexistent.point"}, nil)
	if err == nil {
		t.Fatal("esperaba error para hook point inexistente")
	}
}

func TestRuntime_CloseReleasesMemory(t *testing.T) {
	rt := NewRuntime(WithMemoryLimit(64))
	// register sin Call
	if err := rt.Register("x", `function(ctx, payload) { return payload; }`); err != nil {
		t.Fatal(err)
	}
	if err := rt.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// usar después de Close debe dar error
	_, err := rt.Call("x", Context{Point: "x"}, nil)
	if err == nil {
		t.Fatal("esperaba error al usar runtime cerrado")
	}
}

func TestRegistry_UnregisterRemovesHook(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	registry := NewRegistry(rt)

	if err := registry.Register("test.unreg", "temp", 100, true,
		`function(ctx, payload) { return payload; }`); err != nil {
		t.Fatal(err)
	}
	if err := registry.Unregister("test.unreg", "temp"); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	_, err := registry.Call("test.unreg", map[string]any{"v": 1})
	if err == nil {
		t.Fatal("esperaba error al llamar hook desregistrado")
	}
}

// TestErrorWrapping verifica que los errores de JS se wrappean con %w para trazabilidad.
func TestErrorWrapping(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	err := rt.Register("throw.hook", `function(ctx, payload) {
		throw new Error("JS failure");
	}`)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, err = rt.Call("throw.hook", Context{Point: "throw.hook"}, nil)
	if err == nil {
		t.Fatal("esperaba error de hook que hace throw")
	}
	// el error debe contener el mensaje original del JS
	if !containsAny(err.Error(), "JS failure", "throw") {
		t.Fatalf("error debe contener mensaje JS original: %v", err)
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// TestRuntime_EvalInvalidJS verifica que JS parse-error devuelva error wrappeado.
func TestRuntime_EvalInvalidJS(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	_, err := rt.Eval("this is not valid js {{{")
	if err == nil {
		t.Fatal("esperaba error de parse para JS inválido")
	}
}

// TestRegistry_CallBreakOnError verifica que break-on-error funcione.
func TestRegistry_CallBreakOnError(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	registry := NewRegistry(rt)

	// hook 1: lanza error
	if err := registry.Register("pipeline", "fail", 10, true,
		`function(ctx, payload) { throw new Error("fail"); }`,
	); err != nil {
		t.Fatal(err)
	}
	// hook 2: no debería ejecutarse (break-on-error)
	if err := registry.Register("pipeline", "after", 100, true,
		`function(ctx, payload) { payload.ran = true; return payload; }`,
	); err != nil {
		t.Fatal(err)
	}

	result, err := registry.Call("pipeline", map[string]any{})
	if err == nil {
		t.Fatal("esperaba error de hook fail")
	}
	if result == nil {
		// OK: break-on-error detuvo antes de hook 2
	} else if ran, _ := result["ran"].(bool); ran {
		t.Fatal("hook 'after' no deberia haber corrido con break-on-error")
	}
}
