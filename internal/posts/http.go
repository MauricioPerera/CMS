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
	"context"
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
	s            *Posts
	log          *slog.Logger
	tr           Tracer
	m            *Metrics
	smux         *http.ServeMux
	auth         AuthFunc
	authRequired bool
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
	h.smux.HandleFunc("POST /posts", h.AuthRequired(h.Create))                // C47 write
	h.smux.HandleFunc("PUT /posts/{id}", h.AuthRequired(h.Update))            // C47 write
	h.smux.HandleFunc("POST /posts/{id}/publish", h.AuthRequired(h.Publish))  // C47 write
	h.smux.HandleFunc("DELETE /posts/{id}", h.AuthRequired(h.Delete))           // C51 write
	h.smux.HandleFunc("POST /posts/{id}/restore", h.AuthRequired(h.Restore))      // C54 write (undelete)
	h.smux.HandleFunc("PATCH /posts/{id}", h.AuthRequired(h.Patch))             // C53 write
	h.smux.HandleFunc("GET /posts/s/{slug}", h.GetBySlugRendered)
	h.smux.HandleFunc("GET /posts/{id}", h.GetRendered)
	h.smux.HandleFunc("GET /metrics", h.Metrics)
	return h
}

// === Delete (DELETE /posts/{id}) ===

// Delete elimina un post (DELETE /posts/{id}). Requiere auth (C47/C51).
// Dispara post.validate (action:"delete") antes del hard-delete.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.m.IncRequests()
	span := h.tr.Start("posts.delete")
	defer span.End()
	span.Attr("endpoint", "DELETE /posts/{id}")

	id, err := parseIDFromPath(r, "id")
	if err != nil {
		h.log.Error("posts.delete.bad_id", "err", err, "user", userFromContext(r.Context()))
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid id"})
		return
	}
	ctx := hooks.Context{Point: "post.validate"}
	if err := h.s.Delete(ctx, id); err != nil {
		// Hook reject o not-found → 404 (no se filtra el mensaje interno, C41 hardening).
		h.log.Error("posts.delete.error", "err", err, "id", id,
			"user", userFromContext(r.Context()))
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		code := http.StatusNotFound
		if !errors.Is(err, sql.ErrNoRows) && !strings.Contains(err.Error(), "no se eliminó") {
			// Hook reject (post.validate) o error real → 400 (no 404).
			code = http.StatusBadRequest
		}
		writeJSON(w, code, map[string]any{"error": "no se pudo eliminar el post"})
		return
	}
	h.log.Info("posts.delete.ok", "id", id, "user", userFromContext(r.Context()),
		"latency_ms", time.Since(start).Milliseconds())
	h.m.RecordLatency(time.Since(start))
	w.WriteHeader(http.StatusNoContent)
}

// === Patch (PATCH /posts/{id}) ===

// Patch aplica un update parcial (sólo title y/o content). Requiere auth (C53).
// El body JSON usa campos opcionales: {"title":"..."} o {"content":"..."} o ambos.
func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.m.IncRequests()
	span := h.tr.Start("posts.patch")
	defer span.End()
	span.Attr("endpoint", "PATCH /posts/{id}")

	id, err := parseIDFromPath(r, "id")
	if err != nil {
		h.log.Error("posts.patch.bad_id", "err", err, "user", userFromContext(r.Context()))
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid id"})
		return
	}
	// Decode a raw map para detectar qué campos vienen (PATCH parcial).
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		h.log.Error("posts.patch.bad_body", "err", err, "id", id,
			"user", userFromContext(r.Context()))
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	in := PatchInput{ID: id}
	if t, ok := raw["title"]; ok {
		in.HasTitle = true
		if err := json.Unmarshal(t, &in.Title); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "title invalido"})
			return
		}
	}
	if c, ok := raw["content"]; ok {
		in.HasContent = true
		if err := json.Unmarshal(c, &in.Content); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "content invalido"})
			return
		}
	}
	ctx := hooks.Context{Point: "post.validate"}
	p, err := h.s.Patch(ctx, in)
	if err != nil {
		h.log.Error("posts.patch.error", "err", err, "id", id,
			"user", userFromContext(r.Context()))
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	h.log.Info("posts.patch.ok", "id", id, "user", userFromContext(r.Context()),
		"latency_ms", time.Since(start).Milliseconds())
	h.m.RecordLatency(time.Since(start))
	writeJSON(w, http.StatusOK, map[string]any{
		"id": p.ID, "slug": p.Slug, "title": p.Title, "content": p.Content, "status": p.Status,
		"updated_at": p.UpdatedAt,
	})
}

// === Restore (POST /posts/{id}/restore) ===

// Restore deshace un soft-delete (C54). Requiere auth.
// POST /posts/{id}/restore → 200 + Post restaurado | 404 si no existe o no está borrado.
func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.m.IncRequests()
	span := h.tr.Start("posts.restore")
	defer span.End()
	span.Attr("endpoint", "POST /posts/{id}/restore")

	id, err := parseIDFromPath(r, "id")
	if err != nil {
		h.log.Error("posts.restore.bad_id", "err", err, "user", userFromContext(r.Context()))
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid id"})
		return
	}
	ctx := hooks.Context{Point: "post.validate"}
	p, err := h.s.Restore(ctx, id)
	if err != nil {
		h.log.Error("posts.restore.error", "err", err, "id", id,
			"user", userFromContext(r.Context()))
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "no se pudo restaurar el post"})
		return
	}
	h.log.Info("posts.restore.ok", "id", id, "user", userFromContext(r.Context()),
		"latency_ms", time.Since(start).Milliseconds())
	h.m.RecordLatency(time.Since(start))
	writeJSON(w, http.StatusOK, map[string]any{
		"id": p.ID, "slug": p.Slug, "title": p.Title, "content": p.Content, "status": p.Status,
		"updated_at": p.UpdatedAt,
	})
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

// === Write path (C47: Create/Update/Publish + auth opcional) ===

// AuthResult es el resultado de autenticar una request (C47).
type AuthResult struct {
	OK   bool
	User string // identidad del usuario autenticado (para logging/auditoría).
}

// AuthFunc autentica una request. Por defecto auth OFF (tests/local).
// En prod, inyectar vía WithAuth; NewHandler panickea (C49) si WithAuth(nil)
// se combina con AuthRequiredEnable(true).
type AuthFunc func(r *http.Request) AuthResult

// AuthRequiredEnable activa la exigencia de auth para write endpoints (C47).
// Por defecto FALSE (write endpoints requieren auth sólo si esto es true).
func AuthRequiredEnable(b bool) Option { return func(h *Handler) { h.authRequired = b } }
func WithAuth(f AuthFunc) Option        { return func(h *Handler) { h.auth = f } }

// AuthRequired es middleware que rechaza (401) requests no autenticadas
// en los write endpoints (C47).
func (h *Handler) AuthRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.authRequired {
			next.ServeHTTP(w, r)
			return
		}
		if h.auth == nil {
			h.log.Error("posts.auth.misconfigured", "endpoint", r.URL.Path)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "auth not configured"})
			return
		}
		res := h.auth(r)
		if !res.OK {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		// Inyectamos el usuario en el contexto para auditoría.
		ctx := contextWithUser(r.Context(), res.User)
		*r = *r.WithContext(ctx)
		next.ServeHTTP(w, r)
	}
}

// Context keys para C47.
type ctxKey string

const userKey ctxKey = "user"

func contextWithUser(ctx context.Context, u string) context.Context {
	return context.WithValue(ctx, userKey, u)
}

func userFromContext(ctx context.Context) string {
	if v, _ := ctx.Value(userKey).(string); v != "" {
		return v
	}
	return "<anonymous>"
}

// Create escribe un post en draft (POST /posts). Requiere auth (C47).
// POST {slug,title,content,authorId} → 201 + CreatedPost JSON.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.m.IncRequests()
	span := h.tr.Start("posts.create")
	defer span.End()
	span.Attr("endpoint", "POST /posts")

	user := userFromContext(r.Context())
	var in CreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		h.log.Error("posts.create.bad_body", "err", err, "user", user)
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	// Validation básica (el hook post.validate C2 hace validación de dominio).
	if in.Slug == "" || in.Title == "" || in.Content == "" {
		h.log.Error("posts.create.validation_failed",
			"slug", in.Slug != "", "title", in.Title != "", "content", in.Content != "",
			"user", user)
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "slug, title y content son requeridos",
		})
		return
	}

	ctx := hooks.Context{Point: "post.validate"}
	p, err := h.s.Create(ctx, in)
	if err != nil {
		h.log.Error("posts.create.error", "err", err,
			"slug", in.Slug, "user", user)
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	h.log.Info("posts.create.ok", "id", p.ID, "slug", p.Slug,
		"user", user, "latency_ms", time.Since(start).Milliseconds())
	w.Header().Set("Location", "/posts/"+strconv.FormatInt(p.ID, 10))
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": p.ID, "slug": p.Slug, "title": p.Title, "content": p.Content, "status": p.Status,
		"created_at": p.CreatedAt, "updated_at": p.UpdatedAt,
	})
}

// Update muta title/content (PUT /posts/{id}). Requiere auth (C47).
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.m.IncRequests()
	span := h.tr.Start("posts.update")
	defer span.End()
	span.Attr("endpoint", "PUT /posts/{id}")

	id, err := parseIDFromPath(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid id"})
		return
	}
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.log.Error("posts.update.bad_body", "err", err, "id", id)
		h.m.IncErrors()
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	in := UpdateInput{ID: id, Title: body.Title, Content: body.Content}
	ctx := hooks.Context{Point: "post.validate"}
	p, err := h.s.Update(ctx, in)
	if err != nil {
		h.log.Error("posts.update.error", "err", err, "id", id)
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	h.log.Info("posts.update.ok", "id", id, "user", userFromContext(r.Context()))
	writeJSON(w, http.StatusOK, map[string]any{
		"id": p.ID, "slug": p.Slug, "title": p.Title, "content": p.Content, "status": p.Status,
		"updated_at": p.UpdatedAt,
	})
}

// Publish setea status="published" (POST /posts/{id}/publish). Requiere auth (C47).
func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.m.IncRequests()
	span := h.tr.Start("posts.publish")
	defer span.End()
	span.Attr("endpoint", "POST /posts/{id}/publish")

	id, err := parseIDFromPath(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid id"})
		return
	}
	ctx := hooks.Context{Point: "post.validate"}
	p, err := h.s.Publish(ctx, id)
	if err != nil {
		h.log.Error("posts.publish.error", "err", err, "id", id)
		h.m.IncErrors()
		h.m.RecordLatency(time.Since(start))
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	h.log.Info("posts.publish.ok", "id", id, "user", userFromContext(r.Context()))
	writeJSON(w, http.StatusOK, map[string]any{
		"id": p.ID, "slug": p.Slug, "status": "published", "updated_at": p.UpdatedAt,
	})
}

func parseIDFromPath(r *http.Request, name string) (int64, error) {
	idStr := pathParam(r, name)
	return strconv.ParseInt(idStr, 10, 64)
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
