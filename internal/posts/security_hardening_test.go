package posts

// Tests C60 — remediación C59 findings (security scan). NO tocan los oráculos
// frozen de http_test.go/hooks_test.go/posts_test.go/migrations_test.go.

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopress/internal/hooks"
)

// c60SetupHandler construye un Handler con store real (sqlite in-memory) y
// opciones para testing, reutilizando helpers existentes del paquete.
func c60SetupHandler(t *testing.T, opts ...Option) *Handler {
	t.Helper()
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)
	return NewHandler(s, opts...)
}

// --- C60.1: unbounded request body DoS → 413 (MaxBytesReader) ---

// TestC60_BodyLimitRejectsOversizedWrite verifica el finding C59: cuando el
// deployer provee WithBodyLimit, un POST /posts con Content-Length > límite
// retorna 413 (Content-Length) o 413/400 (chunked/MaxBytesReader), en vez de
// buffer infinito.
func TestC60_BodyLimitRejectsOversizedWrite(t *testing.T) {
	const limit = int64(1024) // 1 KiB límite de prueba
	h := c60SetupHandler(t, WithBodyLimit(limit))
	if h.bodyLimit != limit {
		t.Fatalf("bodyLimit=%d want %d", h.bodyLimit, limit)
	}

	body := strings.Repeat("a", int(limit)+512)
	req := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	got := rr.Code
	// 413 (Content-Length path) es el contracto exacto.
	if got != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized POST: status=%d want %d", got, http.StatusRequestEntityTooLarge)
	}
	if ct := rr.Header().Get("Retry-After"); ct != "" {
		t.Fatalf("413 no debería incluir Retry-After; got %q", ct)
	}
}

// TestC60_BodyLimitAllowsUnderLimitWrite confirma que under el límite el write
// procede — evita false positives del límite.
func TestC60_BodyLimitAllowsUnderLimitWrite(t *testing.T) {
	h := c60SetupHandler(t, WithBodyLimit(4096))
	in := `{"slug":"c60-ok","title":"ok","content":"ok","authorId":1}`
	req := httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(in))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// Auth OFF + rate nil → Create procede (201) o 400/500 si hook falla.
	// Lo que NO debe ser es 413 (límite no debería aplicar al under-limit).
	if rr.Code == http.StatusRequestEntityTooLarge {
		t.Fatalf("under-limit POST no debería dar 413; got %d body=%s",
			rr.Code, rr.Body.String())
	}
}

// TestC60_BodyLimitDoesNotApplyToReads verifica que el límite afecte
// exclusivamente writes (GET no se corta).
func TestC60_BodyLimitDoesNotApplyToReads(t *testing.T) {
	h := c60SetupHandler(t, WithBodyLimit(1)) // límite ridículo solo en writes
	req := httptest.NewRequest(http.MethodGet, "/posts", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET con bodyLimit=1 no debería afectar; status=%d", rr.Code)
	}
}

// --- C60.2: auth fail-open hardening (fail-fast) ---

// TestC60_AuthRequiredFailFastWhenMisconfigured verifica el hardening del
// finding C59 (HIGH, auth fail-open): con authRequired=true PERO auth=nil,
// el middleware retorna 500 "auth not configured" (fail-fast) en lugar de
// dejar pasar writes. El default sigue authRequired=false (BC con HTTP suite);
// este test cubre el path de producción `AuthRequiredEnable(true)` sin WithAuth.
func TestC60_AuthRequiredFailFastWhenMisconfigured(t *testing.T) {
	h := c60SetupHandler(t, AuthRequiredEnable(true)) // auth ON, auth=nil
	if h.auth != nil {
		t.Fatalf("auth=nil expected con fail-closed pero auth != nil")
	}

	in := `{"slug":"c60-fail","title":"t","content":"c","authorId":1}`
	req := httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(in))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("auth required + auth=nil: status=%d want 500 (fail-fast)", rr.Code)
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("auth not configured")) {
		t.Fatalf("esperaba 'auth not configured'; body=%s", rr.Body.String())
	}

	// Sanity: con AuthFunc OK el mismo path procede (no 500) → fail-fast no
	// es un falso positivo sobre todos los writes.
	ok := c60SetupHandler(t,
		AuthRequiredEnable(true),
		WithAuth(func(*http.Request) AuthResult { return AuthResult{OK: true, User: "tester"} }),
	)
	rr2 := httptest.NewRecorder()
	ok.ServeHTTP(rr2, httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(in)))
	if rr2.Code == http.StatusInternalServerError {
		t.Fatalf("auth OK + authRequired=true no debería dar 500; got %d body=%s",
			rr2.Code, rr2.Body.String())
	}
	_ = hooks.Context{}
}
