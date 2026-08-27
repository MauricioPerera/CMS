// Tests de concurrencia REAL (t.Parallel + goroutines) — C50.
//
// NO toca hooks_test.go (oráculo PM congelado C2, SHA 5f14c71b…).
// Estos tests validan race-safety a nivel de test (no solo WaitGroup dentro
// de un test): t.Parallel() fuerza al scheduler Go a ejecutar tests en paralelo.
//
// Habilitado por C48: driver mattn C50 CGO race-safe. Sobre -race: 0 data races.

package hooks

import (
	"sync"
	"testing"
)

// TestRuntime_ParallelCall valida que múltiples Calls concurrentes (con t.Parallel
// del scheduler) sobre runtimes distintos no produzcan data races. QuickJS Runtime
// no es thread-safe a nivel de instancia → aislamiento: 1 runtime por goroutine.
func TestRuntime_ParallelCall(t *testing.T) {
	t.Parallel()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()

			rt := NewRuntime()
			defer rt.Close()

			if err := rt.Register("post.validate", `function(ctx, payload) {
				return { ok: typeof payload.value === "number" };
			}`); err != nil {
				t.Errorf("Register: %v", err)
				return
			}

			result, err := rt.Call("post.validate", Context{Point: "post.validate"},
				map[string]any{"value": n})
			if err != nil {
				t.Errorf("Call: %v", err)
				return
			}
			if ok, _ := result["ok"].(bool); !ok {
				t.Errorf("esperaba ok=true para value=%d", n)
			}
		}(i)
	}
	wg.Wait()
}

// TestRegistry_ParallelDistinctPoints valida registries distintos (cada uno con
// su runtime) ejecutándose en paralelo sin shared state → 0 races.
func TestRegistry_ParallelDistinctPoints(t *testing.T) {
	t.Parallel()

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()

			rt := NewRuntime()
			defer rt.Close()
			registry := NewRegistry(rt)

			if err := registry.Register("concurrent.distinct", "h", 100, true,
				`function(ctx, payload) { return { echoed: payload.in }; }`); err != nil {
				t.Errorf("Register: %v", err)
				return
			}

			result, err := registry.Call("concurrent.distinct", map[string]any{"in": n})
			if err != nil {
				t.Errorf("Call: %v", err)
				return
			}
			_ = result
		}(i)
	}
	wg.Wait()
}
