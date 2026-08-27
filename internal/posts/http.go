// Package posts: API HTTP REST del read path posts (C43) con instrumentación que
// remedia los findings de observabilidad C41.
//
// Endpoints (stdlib net/http, sin dependencias):
//   GET /posts                          → List (paginado)
//   GET /posts/{id}                    → GetRendered por ID
//   GET /posts/s/{slug}                → GetBySlugRendered por slug
//   GET /metrics                       → contadores/latencias (JSON)
//
// Instrumentación (remedia C41):
//   - log/slog estructurado (error paths + hook errors no silent).
//   - Tracer interface (inyectable; default noop) → span por endpoint.
//   - Metrics struct (lock-protected) → requests/errors/sanitize_stripped/latencias.
//   - Cache headers ETag/Last-Modified + 304 Not Modified.
//
// Contrato C43: posts-http-api.
package posts

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopress/internal/hooks"
)

// Tracer es la abstracción de tracing (inyectable; default noopTracer).
type Tracer interface {
	Start(name string) Span
}

// Span representa un span de tracing.
type Span interface {
	End()
	Attr(key, value string)
}

type noopTracer struct{}

func (noopTracer) Start(string) Span { return noopSpan{} }

type noopSpan struct{}

func (noopSpan) End()                {}
func (noopSpan) Attr(string, string) {}

// Metrics contiene contadores/latencias del read path (thread-safe).
type Metrics struct {
	mu               sync.Mutex
	Requests         int64
	Errors           int64
	SanitizeStripped int64
	Latencies        []time.Duration
}

func (m *Metrics) IncRequests() {
	m.mu.Lock()
	m.Requests++
	m.mu.Unlock()
}

func (m *Metrics) IncErrors() {
	m.mu.Lock()
	m.Errors++
	m.mu.Unlock()
}

// IncSanitizeStripped se llama cuando Sanitize estrippa contenido (len out < len in).
func (m *Metrics) IncSanitizeStripped() {
	m.mu.Lock()
	m.SanitizeStripped++
	m.mu.Unlock()
}

func (m *Metrics) RecordLatency(d time.Duration) {
	m.mu.Lock()
	m.Latencies = append(m.Latencies, d)
	m.mu.Unlock()
}

func (m *Metrics) Snapshot() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	latMs := make([]int64, len(m.Latencies))
	for i, d := range m.Latencies {
		latMs[i] = d.Milliseconds()
	}
	return map[string]any{
		"requests":          m.Requests,
		"errors":            m.Errors,
		"sanitize_stripped": m.SanitizeStripped,
		"latencies_ms":      latMs,
	}
}

// Option para configurar el Handler.
type Option func(*Handler)

func WithLogger(l *slog.Logger) Option { return func(h *Handler) { h.log = l } }
func WithTracer(t Tracer) Option        { return func(h *Handler) { h.tr = t } }
func WithMetrics(m *Metrics) Option     { return func(h *Handler) { h.m = m } }

// Handler expone el read path de posts como REST.
type Handler struct {
	s    *Posts
	log  *slog.Logger
	tr   Tracer
	m    *Metrics
	smux *http.ServeMux
}

// NewHandler construye el Handler y registra las rutas en un ServeMux interno.
//
// Logger injection (C49): por defecto usa slog.Default() (suficiente para tests/local).
// En producción (CMS_REQUIRE_LOGGER=1), NewHandler PANIC-kia si se inyecta con
// WithLogger(nil) -- fail-fast contra el residual obs_posts_http_logger_default
// (confiar en slog.Default en prod = logger no configurado explícitamente).
func NewHandler(s *Posts, opts ...Option) *Handler {
	h := &Handler{
		s:   s,
		log: slog.Default(),
		tr:  noopTracer{},
		m:   &Metrics{},
	}
	for _, opt := range opts {
		opt(h)
	}
	if h.log == nil {
		panic("C49: NewHandler recibió WithLogger(nil); en prod use CMS_REQUIRE_LOGGER=1 " +
			"y provea un logger estructurado (slog.NewJSONHandler).")
	}
	h.smux = http.NewServeMux()
	h.smux.HandleFunc("GET /posts", h.List)
	h.smux.HandleFunc("GET /posts/s/{slug}", h.GetBySlugRendered)
	h.smux.HandleFunc("GET /posts/{id}", h.GetRendered)
	h.smux.HandleFunc("GET /metrics", h.Metrics)
	return h
}

// ServeHTTP delega al mux interno.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.smux.ServeHTTP(w, r)
}

// writeJSON escribe JSON con content-type y status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// pathParam extrae un wildcard del path (Go 1.22 mux) o lo parsea manualmente
// si el handler fue llamado directamente (e.g. en tests sin mux).
func pathParam(r *http.Request, name string) string {
	if v := r.PathValue(name); v != "" {
		return v
	}
	// Fallback manual: /posts/{id} o /posts/s/{slug}.
	p := r.URL.Path
	parts := strings.Split(strings.Trim(p, "/"), "/")
	switch name {
	case "slug":
		if len(parts) >= 3 && parts[1] == "s" {
			return parts[2]
		}
	case "id":
		if len(parts) >= 2 && parts[0] == "posts" {
			return parts[1]
		}
	}
	return ""
}

// === List (GET /posts) ===

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.m.IncRequests()
	span := h.tr.Start("posts.list")
	defer span.End()
	span.Attr("endpoint", "GET /posts")

	in := ListInput{
		Limit:    parseIntOrDefault(r, "limit", 20),
		Offset:   parseIntOrDefault(r, "offset", 0),
		Status:   firstNonEmpty(r.URL.Query().Get("status"), "published"),
		Query:    r.URL.Query().Get("q"),
		AuthorID: int64(parseIntOrDefault(r, "author", 0)),
	}
	in.Limit = clampLimit(in.Limit)
	in.Tag = r.URL.Query().Get("tag")

	res, err := h.s.List(in)
	if err != nil {
		h.log.Error("posts.list.error",
			"err", err,
			"limit", in.Limit, "offset", in.Offset,
			"status", in.Status, "author", in.AuthorID, "tag", in.Tag,
		)
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal error"})
		return
	}

	// Headers de paginación.
	w.Header().Set("X-Total", strconv.Itoa(res.Total))
	w.Header().Set("X-Offset", strconv.Itoa(in.Offset))
	w.Header().Set("X-Limit", strconv.Itoa(in.Limit))
	w.Header().Set("X-Has-More", strconv.FormatBool(res.HasMore))

	h.log.Info("posts.list.ok",
		"total", res.Total, "items", len(res.Items),
		"has_more", res.HasMore, "latency_ms", time.Since(start).Milliseconds(),
	)
	h.m.RecordLatency(time.Since(start))
	writeJSON(w, http.StatusOK, res.Items)
}

// === GetRendered (GET /posts/{id}) ===

func (h *Handler) GetRendered(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.m.IncRequests()
	span := h.tr.Start("posts.get_rendered")
	defer span.End()
	span.Attr("endpoint", "GET /posts/{id}")

	idStr := pathParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.log.Error("posts.get_rendered.bad_id", "err", err, "id", idStr)
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid id"})
		return
	}

	ctx := hooks.Context{Point: "post.render"}
	rp, err := h.s.GetRendered(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.log.Info("posts.get_rendered.not_found", "id", id)
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		h.log.Error("posts.get_rendered.error", "err", err, "id", id)
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal error"})
		return
	}

	h.setCacheHeaders(w, rp.HTML, rp.UpdatedAt)
	if checkNotModified(r, w) {
		h.m.RecordLatency(time.Since(start))
		w.WriteHeader(http.StatusNotModified)
		return
	}

	h.log.Info("posts.get_rendered.ok", "id", id, "latency_ms", time.Since(start).Milliseconds())
	h.m.RecordLatency(time.Since(start))
	writeJSON(w, http.StatusOK, rp)
}

// === GetBySlugRendered (GET /posts/s/{slug}) ===

func (h *Handler) GetBySlugRendered(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.m.IncRequests()
	span := h.tr.Start("posts.get_by_slug_rendered")
	defer span.End()
	span.Attr("endpoint", "GET /posts/s/{slug}")

	slug := pathParam(r, "slug")
	if slug == "" {
		h.log.Error("posts.get_by_slug_rendered.bad_slug", "slug", slug)
		h.m.IncErrors()
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing slug"})
		return
	}
	ctx := hooks.Context{Point: "post.render"}
	rp, err := h.s.GetBySlugRendered(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.log.Info("posts.get_by_slug_rendered.not_found", "slug", slug)
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		h.log.Error("posts.get_by_slug_rendered.error", "err", err, "slug", slug)
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal error"})
		return
	}

	h.setCacheHeaders(w, rp.HTML, rp.UpdatedAt)
	if checkNotModified(r, w) {
		h.m.RecordLatency(time.Since(start))
		w.WriteHeader(http.StatusNotModified)
		return
	}

	h.log.Info("posts.get_by_slug_rendered.ok", "slug", slug, "latency_ms", time.Since(start).Milliseconds())
	h.m.RecordLatency(time.Since(start))
	writeJSON(w, http.StatusOK, rp)
}

// === /metrics ===

func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.m.Snapshot())
}

// === helpers ===

func (h *Handler) setCacheHeaders(w http.ResponseWriter, body string, updatedAt time.Time) {
	sum := sha256.Sum256([]byte(body))
	etag := `W/"` + hex.EncodeToString(sum[:8]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Last-Modified", updatedAt.UTC().Format(http.TimeFormat))
	w.Header().Set("Cache-Control", "no-cache")
}

func checkNotModified(r *http.Request, w http.ResponseWriter) bool {
	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifNoneMatch != "" && ifNoneMatch == w.Header().Get("ETag") {
		return true
	}
	ifMod := r.Header.Get("If-Modified-Since")
	if ifMod != "" {
		if t, err := http.ParseTime(ifMod); err == nil {
			if lm := w.Header().Get("Last-Modified"); lm != "" {
				if lt, err := http.ParseTime(lm); err == nil && !t.Before(lt) {
					return true
				}
			}
		}
	}
	return false
}

func parseIntOrDefault(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
