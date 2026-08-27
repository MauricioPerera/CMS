package posts

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"gopress/internal/hooks"
)

// e2eSetupConfig construye un Handler con auth + rate-limit para el test server real.
func e2eSetupConfig(t *testing.T) (*Handler, *Posts, http.Handler) {
	t.Helper()
	h := newTestHandler(t)
	h.authRequired = true
	h.auth = authOK
	h.rl = NewTokenBucketRateLimiter(5, 0.1)
	return h, h.s, h
}

// e2ePost crea + publica un post para tests, devolviendo su ID.
func e2ePost(t *testing.T, s *Posts, slug, title string) int64 {
	t.Helper()
	p, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: slug, Title: title, Content: "# Hola",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := s.Publish(hooks.Context{Point: "post.validate"}, p.ID); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	return p.ID
}

// === C56 E2E: routing real + auth middleware + rate-limit ===

func TestE2E_MetricsAndRead(t *testing.T) {
	// C56: GET /metrics público (read) → 200 + JSON counters.
	h, s, _ := e2eSetupConfig(t)
	e2ePost(t, s, "e2e-metrics", "T")
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics status=%d quiere 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "requests") {
		t.Errorf("/metrics no expone 'requests': %s", body)
	}
}

func TestE2E_WriteAuthRejects(t *testing.T) {
	// C56: POST /posts con authRequired ON + AuthFunc rejectAll → 401 vía routing real del mux.
	h := newTestHandler(t)
	h.authRequired = true
	h.auth = rejectAll // rechaza TODO request → 401
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL+"/posts", strings.NewReader(`{"slug":"x","title":"X","content":"c"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST sin auth status=%d quiere 401", resp.StatusCode)
	}
}

func TestE2E_PatchUnauthAndRateLimit(t *testing.T) {
	// C56: PATCH sin auth (rejectAll) → 401; rate-limit (capacidad 3, authOK) → 429.
	h, s, mux := e2eSetupConfig(t)
	h.rl = NewTokenBucketRateLimiter(3, 0.01) // capacidad 3 para saturar rápido
	id := e2ePost(t, s, "e2e-patch", "P")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Sin auth (rejectAll) → 401 (AuthRequired bloquea antes del handler).
	h.auth = rejectAll
	patchReq, _ := http.NewRequest("PATCH", srv.URL+"/posts/"+strconv.FormatInt(id, 10), strings.NewReader(`{"title":"x"}`))
	patchReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(patchReq)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("PATCH sin auth status=%d quiere 401", resp.StatusCode)
	}

	// Authed + rate-limit (capacidad 3): 3× 200, 4ª → 429.
	h.auth = authOK
	var lastCode int
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest("PATCH", "/posts/"+strconv.FormatInt(id, 10), strings.NewReader(`{"title":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.AuthRequired(h.Patch).ServeHTTP(rec, req)
		lastCode = rec.Code
		if i < 3 && rec.Code != http.StatusOK {
			t.Fatalf("PATCH #%d status=%d quiere 200", i, rec.Code)
		}
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("4ª PATCH (rate limit) status=%d quiere 429", lastCode)
	}
}

func TestE2E_CreateWithAuth(t *testing.T) {
	// C56: POST /posts con auth OK (authOK ignora request → siempre OK) → 201.
	h, _, _ := e2eSetupConfig(t)
	body := `{"slug":"e2e-create","title":"E2E","content":"hola"}`
	req := httptest.NewRequest("POST", "/posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.AuthRequired(h.Create).ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		b, _ := io.ReadAll(rec.Result().Body)
		t.Fatalf("POST authed status=%d quiere 201: %s", rec.Code, b)
	}
}

func TestE2E_GetRenderedRouting(t *testing.T) {
	// C56: GET /posts/{id} (routing mux Go 1.22) → 200 con HTML renderizado.
	_, s, mux := e2eSetupConfig(t)
	id := e2ePost(t, s, "e2e-render", "Render")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/posts/" + strconv.FormatInt(id, 10))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /posts/{id} status=%d quiere 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	// "# Hola" → <h1>Hola</h1> en el HTML (JSON-escaped como \u003ch1\u003e).
	if !strings.Contains(string(body), "h1") {
		t.Errorf("render: HTML no contiene <h1>: %s", body)
	}
}

func TestE2E_DeleteThenRestoreRouting(t *testing.T) {
	// C56 E2E: DELETE → 204, GET /posts/{id} → 404 (soft-delete read filter), RESTORE → 200, GET → 200.
	h, s, mux := e2eSetupConfig(t)
	id := e2ePost(t, s, "e2e-del", "Del")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// DELETE (authed vía AuthRequired wrapper + authOK).
	delRec := httptest.NewRecorder()
	h.AuthRequired(h.Delete).ServeHTTP(delRec, httptest.NewRequest("DELETE", "/posts/"+strconv.FormatInt(id, 10), nil))
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("DELETE status=%d quiere 204", delRec.Code)
	}

	// GET /posts/{id} → 404 (soft-delete filtra deleted_at, read path no authed).
	resp, err := http.Get(srv.URL + "/posts/" + strconv.FormatInt(id, 10))
	if err != nil {
		t.Fatalf("GET post-borrado: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET post-borrado status=%d quiere 404", resp.StatusCode)
	}

	// RESTORE (authed).
	restRec := httptest.NewRecorder()
	h.AuthRequired(h.Restore).ServeHTTP(restRec, httptest.NewRequest("POST", "/posts/"+strconv.FormatInt(id, 10)+"/restore", nil))
	if restRec.Code != http.StatusOK {
		b, _ := io.ReadAll(restRec.Result().Body)
		t.Fatalf("RESTORE status=%d quiere 200: %s", restRec.Code, b)
	}

	// GET /posts/{id} → 200 (restaurado).
	resp2, err := http.Get(srv.URL + "/posts/" + strconv.FormatInt(id, 10))
	if err != nil {
		t.Fatalf("GET post-restaurado: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("GET post-restaurado status=%d quiere 200", resp2.StatusCode)
	}
}
