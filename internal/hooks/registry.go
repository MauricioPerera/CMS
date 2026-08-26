// Hook registry: dispatch por point + prioridad, thread-safe.
//
// Contrato C2: hooks-registry. Satisface el oráculo internal/hooks/hooks_test.go.
// Delega la ejecución JS a Runtime.Exec; mantiene el map point→hooks con
// prioridad, enabled y cuerpo JS (JSOwner).

package hooks

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Hook representa un hook registrado en el registry.
type Hook struct {
	Name     string // nombre legible
	Point    string // hook point (ej: "post.validate")
	Priority int    // lower runs first; default 100
	Enabled  bool
	Runtime  *Runtime
	JSOwner  string // body JS (function(ctx, payload) {...} o statement)
}

// Registry mapea hook points → hooks, thread-safe.
type Registry struct {
	mu    sync.RWMutex
	rt    *Runtime
	hooks map[string][]Hook // point → []Hook
}

// NewRegistry crea un registry vinculado a un runtime.
func NewRegistry(rt *Runtime) *Registry {
	return &Registry{
		rt:    rt,
		hooks: make(map[string][]Hook),
	}
}

// Register agrega un hook al point con prioridad y estado enabled.
// Si ya existe un hook con el mismo point+name, lo sobreescribe.
func (r *Registry) Register(point, name string, priority int, enabled bool, jsBody string) error {
	if r.rt == nil {
		return errors.New("runtime nulo")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validar sintaxis JS vía el runtime antes de registrar.
	// Validar sintaxis JS vía el runtime y almacenar el body normalizado.
	normalized := normalizeHookBody(jsBody)
	if err := r.rt.Register(point, jsBody); err != nil {
		return err
	}

	for i, h := range r.hooks[point] {
		if h.Name == name {
			r.hooks[point][i] = Hook{
				Name: name, Point: point, Priority: priority,
				Enabled: enabled, Runtime: r.rt, JSOwner: normalized,
			}
			return nil
		}
	}
	r.hooks[point] = append(r.hooks[point], Hook{
		Name:     name,
		Point:    point,
		Priority: priority,
		Enabled:  enabled,
		Runtime:  r.rt,
		JSOwner:  normalized,
	})
	return nil
}

// Unregister remueve un hook del point.
func (r *Registry) Unregister(point, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	hooks := r.hooks[point]
	for i, h := range hooks {
		if h.Name == name {
			r.hooks[point] = append(hooks[:i], hooks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("hook no encontrado: %s", name)
}

// List retorna los hooks registrados para un point (ordenados por prioridad ascendente).
func (r *Registry) List(point string) []Hook {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Hook, len(r.hooks[point]))
	copy(out, r.hooks[point])
	sort.Slice(out, func(i, j int) bool { return out[i].Priority < out[j].Priority })
	return out
}

// Call ejecuta todos los hooks del point en orden de prioridad.
// break-on-error: el primer hook que falle detiene la cadena.
// El payload se propaga acumulado entre hooks.
func (r *Registry) Call(point string, payload map[string]any) (map[string]any, error) {
	hooks := r.List(point)
	if len(hooks) == 0 {
		return nil, fmt.Errorf("no hay hooks registrados para: %s", point)
	}

	current := payload
	for _, h := range hooks {
		if !h.Enabled {
			continue
		}
		result, err := r.rt.Exec(h.JSOwner, Context{Point: point}, current)
		if err != nil {
			// break-on-error: propagar, detener cadena.
			return current, err
		}
		current = result
	}
	return current, nil
}
