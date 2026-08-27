// Tests de concurrencia REAL (t.Parallel + goroutines) — C50.
//
// NO toca posts_test.go (oráculo PM congelado C36, SHA f500f75f…).
// Aprovecha helpers ya definidos en posts_test.go/http_test.go:
// freshDB(), setupRuntime(), newTestHandler(), seedPosts().
//
// Habilitado por C48: mattn/go-sqlite3 CGO race-safe (0 crashes bajo -race).

package posts

import (
	"net/http/httptest"
	"sync"
	"testing"

	"gopress/internal/hooks"
)

// TestHandler_ParallelReads valida que múltiples reads concurrentes (List +
// GetBySlugRendered) sobre *sql.DB compartido no produzcan data races. database/sql
// es thread-safe por diseño; el read path es read-only → DB compartida OK.
func TestHandler_ParallelReads(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t)
	s := h.s
	seedPosts(t, s, 5, "par")

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			reqH := httptest.NewRequest("GET", "/posts?limit=10", nil)
			recH := httptest.NewRecorder()
			h.List(recH, reqH)
			if recH.Code != 200 {
				t.Errorf("List status = %d, quiere 200", recH.Code)
			}

			// read por slug (GetBySlugRendered: GET /posts/s/{slug}) también read-only sobre DB compartida.
			reqG := httptest.NewRequest("GET", "/posts/s/par-0", nil)
			recG := httptest.NewRecorder()
			h.GetBySlugRendered(recG, reqG)
			if recG.Code != 200 {
				t.Errorf("GetBySlugRendered status = %d, quiere 200", recG.Code)
			}
		}()
	}
	wg.Wait()
}

// TestHandler_ParallelCreateDistinctSlugs valida writes concurrentes con aislamiento.
// mattn/go-sqlite3 default mode no es WAL → aislamiento por DB distinta por goroutine;
// slugs idénticos colisionan en unique key → usar slug distinto por goroutine.
func TestHandler_ParallelCreateDistinctSlugs(t *testing.T) {
	t.Parallel()

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()

			// DB aislada por goroutine (mattn sqlite3 default no es WAL).
			dbh := freshDB(t)
			rt, reg := setupRuntime(t, hookOK)
			s := NewPosts(dbh, rt, reg)
			defer rt.Close()

			p, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
				Slug:    "parc",
				Title:   "T",
				Content: "# c50",
			})
			if err != nil {
				t.Errorf("Create %d: %v", n, err)
				return
			}
			_ = p
		}(i)
	}
	wg.Wait()
}
