package posts

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopress/internal/hooks"
)

// newTestHandler construye un Handler sobre freshDB + runtime con hooks.
func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)
	// Logger de test silencioso (LevelError) para no ensuciar CI.
	return NewHandler(s, WithLogger(slog.New(slog.NewTextHandler(os.Stderr,
		&slog.HandlerOptions{Level: slog.LevelError}))))
}

// seedPosts crea N posts publicados con contenido Markdown.
func seedPosts(t *testing.T, s *Posts, n int, slugPrefix string) []Post {
	t.Helper()
	var out []Post
	for i := 0; i < n; i++ {
		p, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
			Slug:    slugPrefix + "-" + strconv.Itoa(i),
			Title:   "T" + strconv.Itoa(i),
			Content: "# Title " + strconv.Itoa(i),
		})
		if err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		if _, err := s.Publish(hooks.Context{Point: "post.validate"}, p.ID); err != nil {
			t.Fatalf("Publish %d: %v", i, err)
		}
		out = append(out, p)
	}
	return out
}

// === List ===

func TestHandler_List_OK(t *testing.T) {
	h := newTestHandler(t)
	s := h.s
	seedPosts(t, s, 3, "post")

	req := httptest.NewRequest("GET", "/posts?limit=10", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quiere 200", rec.Code)
	}
	var items []Post
	if err := json.NewDecoder(rec.Result().Body).Decode(&items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("items = %d, quiere 3", len(items))
	}
	if rec.Header().Get("X-Total") != "3" {
		t.Errorf("X-Total = %q, quiere 3", rec.Header().Get("X-Total"))
	}
}

func TestHandler_List_Pagination(t *testing.T) {
	h := newTestHandler(t)
	s := h.s
	seedPosts(t, s, 5, "post")

	req := httptest.NewRequest("GET", "/posts?limit=2&offset=2", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var items []Post
	if err := json.NewDecoder(rec.Result().Body).Decode(&items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("items = %d, quiere 2", len(items))
	}
	if rec.Header().Get("X-Has-More") != "true" {
		t.Errorf("X-Has-More = %q, quiere true", rec.Header().Get("X-Has-More"))
	}
}

func TestHandler_List_ErrorPath(t *testing.T) {
	// C41 remediation: verifica que un error de List se loguea y retorna 500 (no silent).
	h := newTestHandler(t)
	h.s.dbh.Close()

	req := httptest.NewRequest("GET", "/posts?limit=10", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, quiere 500 (error logueado, no silent)", rec.Code)
	}
}

// === GetRendered ===

func TestHandler_GetRendered_NotFound(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest("GET", "/posts/9999", nil)
	rec := httptest.NewRecorder()
	h.GetRendered(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, quiere 404", rec.Code)
	}
}

func TestHandler_GetRendered_OK(t *testing.T) {
	h := newTestHandler(t)
	s := h.s
	p := seedPosts(t, s, 1, "solo")[0]

	req := httptest.NewRequest("GET", "/posts/"+strconv.FormatInt(p.ID, 10), nil)
	rec := httptest.NewRecorder()
	h.GetRendered(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Header().Get("ETag") == "" {
		t.Error("ETag header ausente")
	}
	if rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
}

func TestHandler_GetBySlugRendered_ETag_304(t *testing.T) {
	h := newTestHandler(t)
	s := h.s
	seedPosts(t, s, 1, "etag")

	// Primera request → 200 con ETag.
	req1 := httptest.NewRequest("GET", "/posts/s/etag-0", nil)
	rec1 := httptest.NewRecorder()
	h.GetBySlugRendered(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status1 = %d", rec1.Code)
	}
	etag := rec1.Result().Header.Get("ETag")
	if etag == "" {
		t.Fatal("ETag ausente en 200")
	}

	// Segunda request con If-None-Match → 304.
	req2 := httptest.NewRequest("GET", "/posts/s/etag-0", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	h.GetBySlugRendered(rec2, req2)
	if rec2.Code != http.StatusNotModified {
		t.Errorf("status2 = %d, quiere 304", rec2.Code)
	}
}

// === Metrics ===

func TestMetrics_Increment(t *testing.T) {
	m := &Metrics{}
	m.IncRequests()
	m.IncRequests()
	m.IncErrors()
	m.IncSanitizeStripped()
	m.RecordLatency(5 * time.Millisecond)

	snap := m.Snapshot()
	if snap["requests"].(int64) != 2 {
		t.Errorf("requests = %v", snap["requests"])
	}
	if snap["errors"].(int64) != 1 {
		t.Errorf("errors = %v", snap["errors"])
	}
	if snap["sanitize_stripped"].(int64) != 1 {
		t.Errorf("sanitize_stripped = %v", snap["sanitize_stripped"])
	}
	lats := snap["latencies_ms"].([]int64)
	if len(lats) != 1 || lats[0] != 5 {
		t.Errorf("latencies = %v", lats)
	}
}

func TestHandler_MetricsEndpoint(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	h.Metrics(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var snap map[string]any
	if err := json.NewDecoder(rec.Result().Body).Decode(&snap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := snap["requests"]; !ok {
		t.Error("missing 'requests' en metrics")
	}
}

// === C45: wiring SanitizeStripped metrics (remedia C44 residual #2) ===

func TestSanitize_WiredViaHandler(t *testing.T) {
	// content inseguro → ListRendered → Sanitize estrippa → counter > 0.
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)
	m := &Metrics{}
	s.SetMetrics(m)

	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "xss-c45", Title: "X", Content: "<script>alert(1)</script># Hola",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := s.Publish(hooks.Context{Point: "post.validate"}, post.ID); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	res, err := s.ListRendered(hooks.Context{Point: "post.render"}, ListInput{Limit: 10, Status: ""})
	if err != nil {
		t.Fatalf("ListRendered: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("items = %d", len(res.Items))
	}
	if strings.Contains(res.Items[0].HTML, "<script>") {
		t.Errorf("HTML contiene <script>: %q", res.Items[0].HTML)
	}
	// El strip de <script> debe incrementar SanitizeStripped.
	if m.SanitizeStripped <= 0 {
		t.Errorf("SanitizeStripped = %d, quiere > 0 (C44 residual #2)", m.SanitizeStripped)
	}
}

func TestHandler_ListRendered_IncidentsMetric(t *testing.T) {
	// Verifica que el Handler expone sanitize_stripped vía /metrics después de
	// renderizar un post con content inseguro.
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)
	m := &Metrics{}
	s.SetMetrics(m)
	h := NewHandler(s)

	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "m-c45", Title: "M", Content: "<img src=x onerror=alert(1)>texto",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := s.Publish(hooks.Context{Point: "post.validate"}, post.ID); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	// Render vía handler GetBySlugRendered (ejerce el chain completo + metrics).
	req := httptest.NewRequest("GET", "/posts/s/m-c45", nil)
	rec := httptest.NewRecorder()
	h.GetBySlugRendered(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	// El onerror=... debe haber sido estrippeado → SanitizeStripped > 0.
	if m.SanitizeStripped <= 0 {
		t.Errorf("SanitizeStripped = %d, quiere > 0", m.SanitizeStripped)
	}
}

// === C49: fail-fast logger injection (cierra C44 residual #1) ===

func TestNewHandler_PanicsOnNilLogger(t *testing.T) {
	// WithLogger(nil) debe PANIC (fail-fast, no rely en slog.Default).
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("NewHandler(WithLogger(nil)) no panic, quiere fail-fast C49")
		}
		msg, _ := r.(string)
		if !strings.Contains(msg, "WithLogger(nil)") {
			t.Fatalf("panic message inesperado: %v", r)
		}
	}()
	NewHandler(s, WithLogger(nil))
}

func TestNewHandler_DefaultLoggerOK(t *testing.T) {
	// Sin WithLogger explícito, NewHandler usa slog.Default() (tests/local) sin panic.
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewHandler sin WithLogger panickeo: %v", r)
		}
	}()
	_ = NewHandler(s)
}

// === C47: Write API (Create/Update/Publish + auth) ===

// authOK retorna AuthResult OK con user "tester" (para tests C47 que requieren auth).
func authOK(_ *http.Request) AuthResult { return AuthResult{OK: true, User: "tester"} }

func setupAuthHandler(t *testing.T) (*Handler, *Posts) {
	t.Helper()
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)
	h := NewHandler(s, WithAuth(authOK), AuthRequiredEnable(true))
	return h, s
}

// rejectAll retorna AuthResult rechazado (para tests C52 de auth-bloqueado).
func rejectAll(_ *http.Request) AuthResult { return AuthResult{OK: false} }

// setupAuthHandlerWithReject construye un handler con auth enabled pero que rechaza todo
// (usado por C52 para validar que AuthRequired intercepta antes del handler Delete).
func setupAuthHandlerWithReject(t *testing.T) (*Handler, *Posts) {
	t.Helper()
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)
	h := NewHandler(s, WithAuth(rejectAll), AuthRequiredEnable(true))
	return h, s
}

func TestHandler_Create_OK(t *testing.T) {
	h, _ := setupAuthHandler(t)
	body := strings.NewReader(`{"slug":"hello-c47","title":"Hola","content":"mundo"}`)
	req := httptest.NewRequest("POST", "/posts", body)
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, quiere 201; body: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["id"] == nil || out["status"] != "draft" {
		t.Errorf("unexpected response: %v", out)
	}
}

func TestHandler_Create_ValidationFail(t *testing.T) {
	h, _ := setupAuthHandler(t)
	body := strings.NewReader(`{"slug":"","title":"","content":""}`)
	req := httptest.NewRequest("POST", "/posts", body)
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quiere 400", rec.Code)
	}
}

func TestHandler_Create_BadJSON(t *testing.T) {
	h, _ := setupAuthHandler(t)
	body := strings.NewReader(`{not-json`)
	req := httptest.NewRequest("POST", "/posts", body)
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quiere 400", rec.Code)
	}
}

func TestHandler_Create_AuthRejected(t *testing.T) {
	// Auth habilitada pero sin AuthFunc → middleware rechaza con 500 (misconfigured).
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)
	h := NewHandler(s, AuthRequiredEnable(true)) // enable auth pero sin AuthFunc → 500
	// Llamamos al middleware directamente (simula POST /posts vía mux).
	rec := httptest.NewRecorder()
	h.AuthRequired(h.Create)(rec, httptest.NewRequest("POST", "/posts", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, quiere 500 (auth misconfigured)", rec.Code)
	}
}

func TestHandler_WritePath_AuthBlocksWhenEnabled(t *testing.T) {
	// Con AuthFunc que rechaza → 401 (aunque el JSON sea válido).
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)
	rejectAll := func(*http.Request) AuthResult { return AuthResult{OK: false} }
	h := NewHandler(s, WithAuth(rejectAll), AuthRequiredEnable(true))
	body := strings.NewReader(`{"slug":"x","title":"T","content":"C"}`)
	req := httptest.NewRequest("POST", "/posts", body)
	rec := httptest.NewRecorder()
	h.AuthRequired(h.Create)(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, quiere 401 (auth rejected)", rec.Code)
	}
}

func TestHandler_Update_OK(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)
	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "upd-c47", Title: "Old", Content: "old content",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	h := NewHandler(s, WithAuth(authOK), AuthRequiredEnable(true))
	body := strings.NewReader(`{"title":"New","content":"new content"}`)
	req := httptest.NewRequest("PUT", "/posts/"+strconv.FormatInt(post.ID, 10), body)
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quiere 200; body: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["title"] != "New" || out["content"] != "new content" {
		t.Errorf("unexpected response: %v", out)
	}
}

func TestHandler_Publish_OK(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)
	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "pub-c47", Title: "P", Content: "content",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	h := NewHandler(s, WithAuth(authOK), AuthRequiredEnable(true))
	req := httptest.NewRequest("POST", "/posts/"+strconv.FormatInt(post.ID, 10)+"/publish", nil)
	rec := httptest.NewRecorder()
	h.Publish(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quiere 200", rec.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["status"] != "published" {
		t.Errorf("status = %v, quiere published", out["status"])
	}
}

func TestHandler_Publish_NotFound(t *testing.T) {
	h, _ := setupAuthHandler(t)
	req := httptest.NewRequest("POST", "/posts/99999/publish", nil)
	rec := httptest.NewRecorder()
	h.Publish(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("status = 200, quiere error (no existe id 99999)")
	}
}

func TestHandler_Create_WriteMetricsIncremented(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)
	m := &Metrics{}
	s.SetMetrics(m)
	h := NewHandler(s, WithAuth(authOK), AuthRequiredEnable(true), WithMetrics(m))
	body := strings.NewReader(`{"slug":"m-c47","title":"M","content":"texto"}`)
	req := httptest.NewRequest("POST", "/posts", body)
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
	if m.Requests <= 0 {
		t.Errorf("write endpoint no incrementa Metrics.Requests (got %d)", m.Requests)
	}
}

// === C51: Delete (DELETE /posts/{id}) ===

func TestHandler_Delete_OK(t *testing.T) {
	// Crea un post, luego DELETE → 204 y el post deja de existir.
	h, s := setupAuthHandler(t)
	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "del-c51", Title: "D", Content: "c51 content",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	req := httptest.NewRequest("DELETE", "/posts/"+strconv.FormatInt(post.ID, 10), nil)
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, quiere 204; body: %s", rec.Code, rec.Body.String())
	}
	// Verificar que el post fue eliminado (Get → ErrNoRows).
	if _, err := s.Get(post.ID); err == nil {
		t.Error("el post sigue existiendo tras DELETE")
	}
}

func TestHandler_Delete_NotFound(t *testing.T) {
	// DELETE sobre id inexistente → 404 (no panic).
	h, _ := setupAuthHandler(t)
	req := httptest.NewRequest("DELETE", "/posts/99999", nil)
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quiere 404", rec.Code)
	}
}

func TestHandler_Delete_WriteMetricsIncremented(t *testing.T) {
	// Verifica que el DELETE endpoint incrementa Metrics.Requests (observabilidad C41).
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)
	m := &Metrics{}
	s.SetMetrics(m)
	h := NewHandler(s, WithAuth(authOK), AuthRequiredEnable(true), WithMetrics(m))
	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "del-m", Title: "D", Content: "c51",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	req := httptest.NewRequest("DELETE", "/posts/"+strconv.FormatInt(post.ID, 10), nil)
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	if m.Requests <= 0 {
		t.Errorf("DELETE no incrementa Metrics.Requests (got %d)", m.Requests)
	}
}

// === C52: auth-reject + bad-ID paths para Delete (cobertura write path) ===

func TestHandler_Delete_AuthRejected(t *testing.T) {
	// Auth habilitada con AuthFunc que rechaza → 401 (middleware intercepta antes del handler).
	h, s := setupAuthHandlerWithReject(t)
	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "auth-reject-c52", Title: "R", Content: "c52",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	req := httptest.NewRequest("DELETE", "/posts/"+strconv.FormatInt(post.ID, 10), nil)
	rec := httptest.NewRecorder()
	// AuthRequired(Delete) ejecuta el middleware → 401 antes de tocar el store.
	h.AuthRequired(h.Delete)(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, quiere 401 (auth rejected)", rec.Code)
	}
	// El post debe seguir existiendo (delete fue bloqueado por auth).
	if _, err := s.Get(post.ID); err != nil {
		t.Errorf("post fue eliminado pese a auth rechazada: %v", err)
	}
}

func TestHandler_Delete_BadID(t *testing.T) {
	// id no numérico → 400 (parseIDFromPath falla antes del store/hook).
	h, _ := setupAuthHandler(t)
	req := httptest.NewRequest("DELETE", "/posts/not-a-number", nil)
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quiere 400 (bad id)", rec.Code)
	}
}
