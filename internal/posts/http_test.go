package posts

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
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
