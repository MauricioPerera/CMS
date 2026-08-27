// Rate limiting para el write path HTTP (C55).
//
// Token bucket simple thread-safe, keyeado por usuario (userKey context) con
// fallback a IP remote (X-Forwarded-For / RemoteAddr). Configurado vía
// WithRateLimiter + WithRateLimitConfig. Aplica SOLO a write methods
// (POST/PUT/PATCH/DELETE) para no impactar el read path C41.
//
// Contrato C55: posts-http-api (extiende).
package posts

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimitResult es el resultado de Allow().
type RateLimitResult struct {
	Allowed   bool
	RetryAfter time.Duration
}

// RateLimiter decide si una request write es permitida.
type RateLimiter interface {
	Allow(key string) RateLimitResult
}

// TokenBucketRateLimiter implementa un token bucket por-key.
//
// - capacity: tokens máximos (burst).
// - refill: tokens/segundo (rate). Cada Allow consume 1 token; si no hay,
//   rellena proporcional al elapsed time desde la última consumición.
type TokenBucketRateLimiter struct {
	mu       sync.Mutex
	capacity int
	refill   float64 // tokens/segundo
	buckets  map[string]tokenState
}

type tokenState struct {
	tokens     float64
	lastFill   time.Time
}

// NewTokenBucketRateLimiter construye un limiter con la capacidad y rate dados.
func NewTokenBucketRateLimiter(capacity int, refillPerSec float64) *TokenBucketRateLimiter {
	if capacity < 1 {
		capacity = 1
	}
	if refillPerSec <= 0 {
		refillPerSec = 1
	}
	return &TokenBucketRateLimiter{
		capacity: capacity,
		refill:   refillPerSec,
		buckets:  make(map[string]tokenState),
	}
}

// Allow consume 1 token para key. Si no hay, rellena (token bucket).
func (l *TokenBucketRateLimiter) Allow(key string) RateLimitResult {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	st := l.buckets[key]
	if st.lastFill.IsZero() {
		// Primera visita: inicializamos al máximo (capacidad completa de burst).
		st.tokens = float64(l.capacity)
		st.lastFill = now
	}
	// Refill proporcional al elapsed time desde la última operación.
	elapsed := now.Sub(st.lastFill).Seconds()
	st.tokens += float64(elapsed) * l.refill
	if st.tokens > float64(l.capacity) {
		st.tokens = float64(l.capacity)
	}
	st.lastFill = now

	if st.tokens < 1 {
		// No hay token: calculamos retry-after para llenar el siguiente token.
		needed := 1 - st.tokens
		retry := time.Duration(needed/l.refill * float64(time.Second))
		l.buckets[key] = st
		return RateLimitResult{Allowed: false, RetryAfter: retry}
	}
	st.tokens -= 1
	l.buckets[key] = st
	return RateLimitResult{Allowed: true, RetryAfter: 0}
}

// rateLimitKey extrae la key de rate-limit: usuario autenticado (via userKey
// context, C47) o, si no hay user, la IP remote.
func rateLimitKey(r *http.Request) string {
	if u, _ := r.Context().Value(userKey).(string); u != "" && u != "<anonymous>" {
		return "user:" + u
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip := strings.TrimSpace(strings.Split(ffxSplitFirst(xff), ",")[0]); ip != "" {
			return "ip:" + ip
		}
	}
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && ip != "" {
		return "ip:" + ip
	}
	return "ip:<unknown>"
}

// ffxSplitFirst helper: toma el primer elemento de una lista CSV (X-Forwarded-For).
func ffxSplitFirst(s string) string {
	if i := strings.Index(s, ","); i >= 0 {
		return s[:i]
	}
	return s
}

// isWriteMethod indica si el método HTTP muta estado (POST/PUT/PATCH/DELETE).
// El rate limit (C55) aplica SOLO a writes; reads (GET/HEAD) no se throttlean.
func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
