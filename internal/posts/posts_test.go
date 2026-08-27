// Package posts: tests del CRUD de posts (oráculo PM-frozen, C36).
// NO es modificable por el implementador — toca internal/posts/posts.go.
package posts

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3" // register driver "sqlite3" (CGO, race-safe en Windows; C48: sustituye modernc.org/sqlite).

	"gopress/internal/db"
	"gopress/internal/hooks"
)

// freshDB crea una DB en memoria, aplica las migraciones C1 y retorna *sql.DB.
func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	dir, err := os.MkdirTemp("", "posts_test_")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	dbh, err := sql.Open("sqlite3", filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	migrationsDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("MkdirAll migrations: %v", err)
	}
	// Copiar migraciones del repo (desde internal/posts/, ../.. = repo root).
	src := filepath.Clean("../../db/migrations")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("ReadDir migrations %q: %v", src, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatalf("ReadFile %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(migrationsDir, e.Name()), b, 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", e.Name(), err)
		}
	}
	if err := db.ApplyMigrations(dbh, "file://"+filepath.ToSlash(migrationsDir)); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	t.Cleanup(func() {
		dbh.Close()
		os.RemoveAll(dir)
	})
	return dbh
}

// setupRuntime crea un runtime QuickJS + registry con el hook post.validate inyectado.
func setupRuntime(t *testing.T, hookBody string) (*hooks.Runtime, *hooks.Registry) {
	t.Helper()
	rt := hooks.NewRuntime(
		hooks.WithMemoryLimit(32),
		hooks.WithTimeout(0),
	)
	t.Cleanup(func() { rt.Close() })
	registry := hooks.NewRegistry(rt)
	if err := registry.Register("post.validate", "validate-title", 50, true, hookBody); err != nil {
		t.Fatalf("Register hook: %v", err)
	}
	return rt, registry
}

// hookOK es el cuerpo JS de post.validate que siempre acepta.
const hookOK = `function(ctx, payload) { return { ok: true }; }`

// hookReject es el cuerpo JS de post.validate que siempre rechaza.
const hookReject = `function(ctx, payload) { return { ok: false, error: "rechazado por hook" }; }`

func TestCreate_PostCreatedWithDraft(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)

	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "hola-mundo", Title: "Hola Mundo", Content: "Contenido",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if post.Status != "draft" {
		t.Errorf("Status = %q, quiere %q", post.Status, "draft")
	}
	if post.Slug != "hola-mundo" {
		t.Errorf("Slug = %q, quiere %q", post.Slug, "hola-mundo")
	}
	if post.ID == 0 {
		t.Error("ID autoincrement esperado")
	}
}

func TestCreate_DuplicateSlugError(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)

	if _, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "dup", Title: "T1", Content: "C1",
	}); err != nil {
		t.Fatalf("primer Create: %v", err)
	}
	if _, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "dup", Title: "T2", Content: "C2",
	}); err == nil {
		t.Error("Create con slug duplicado debería fallar")
	}
}

func TestCreate_HookRejectAborts(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookReject)
	s := NewPosts(dbh, rt, reg)

	_, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "blocked", Title: "T", Content: "C",
	})
	if err == nil {
		t.Fatal("Create con hook reject debería fallar")
	}
	// Verificar que NADA se insertó.
	var count int
	if err := dbh.QueryRow("SELECT COUNT(*) FROM posts").Scan(&count); err != nil {
		t.Fatalf("COUNT: %v", err)
	}
	if count != 0 {
		t.Errorf("posts table tiene %d filas, esperaba 0", count)
	}
}

func TestGet_ByIdAndSlug(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)

	created, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "get-test", Title: "Get", Content: "Body",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != created.ID || got.Slug != "get-test" {
		t.Errorf("Get: %+v", got)
	}
	got2, err := s.GetBySlug("get-test")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got2.ID != created.ID {
		t.Errorf("GetBySlug: %+v", got2)
	}
}

func TestPublish_SetsStatus(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)

	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "pub", Title: "Pub", Content: "C",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	published, err := s.Publish(hooks.Context{Point: "post.validate"}, post.ID)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if published.Status != "published" {
		t.Errorf("Status = %q, quiere %q", published.Status, "published")
	}
}

func TestUpdate_MutatesTitleContent(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)

	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "upd", Title: "Old", Content: "OldBody",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	updated, err := s.Update(hooks.Context{Point: "post.validate"}, UpdateInput{
		ID: post.ID, Title: "New", Content: "NewBody",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "New" || updated.Content != "NewBody" {
		t.Errorf("Update: %+v", updated)
	}
	if updated.Status != "draft" {
		t.Errorf("Update mutó status: %q", updated.Status)
	}
}

func TestGet_NotFound(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)

	if _, err := s.Get(999); err == nil {
		t.Error("Get(999) debería fallar")
	}
	if _, err := s.GetBySlug("nope"); err == nil {
		t.Error("GetBySlug('nope') debería fallar")
	}
}

// === C37: posts-render (Markdown→HTML + hooks) ===

func TestRenderMarkdown_Fallback(t *testing.T) {
	got := renderMarkdown("# Hola\n\n**negrita** y `code`\n\n- item1\n- item2")
	if !strings.Contains(got, "<h1>Hola</h1>") {
		t.Errorf("header h1 faltante: %s", got)
	}
	if !strings.Contains(got, "<strong>negrita</strong>") {
		t.Errorf("strong faltante: %s", got)
	}
	if !strings.Contains(got, "<code>code</code>") {
		t.Errorf("code faltante: %s", got)
	}
	if !strings.Contains(got, "<ul>") || !strings.Contains(got, "<li>item1</li>") {
		t.Errorf("lista faltante: %s", got)
	}
}

func TestRenderMarkdown_Empty(t *testing.T) {
	if renderMarkdown("") != "" {
		t.Errorf("empty debe dar empty: %q", renderMarkdown(""))
	}
}

func TestGetRendered_FallbackMarkdown(t *testing.T) {
	dbh := freshDB(t)
	// runtime con hook post.validate pero SIN post.render → fallback Markdown.
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)

	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "render", Title: "R", Content: "# Título\n\npárrafo",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	rp, err := s.GetRendered(hooks.Context{Point: "post.render"}, post.ID)
	if err != nil {
		t.Fatalf("GetRendered: %v", err)
	}
	if !strings.Contains(rp.HTML, "<h1>Título</h1>") {
		t.Errorf("HTML no tiene h1: %s", rp.HTML)
	}
	// Content raw preservado.
	if rp.Content != "# Título\n\npárrafo" {
		t.Errorf("Content raw mutado: %q", rp.Content)
	}
}

// hookRenderJS retorna HTML desde el Markdown.
const hookRenderJS = `function(ctx, payload) { return { ok: true, html: "<p>rendered:" + payload.post.content + "</p>" }; }`

func TestGetRendered_WithHook(t *testing.T) {
	dbh := freshDB(t)
	rt := hooks.NewRuntime(hooks.WithMemoryLimit(32), hooks.WithTimeout(0))
	t.Cleanup(func() { rt.Close() })
	reg := hooks.NewRegistry(rt)
	if err := reg.Register("post.render", "render", 50, true, hookRenderJS); err != nil {
		t.Fatalf("Register render: %v", err)
	}
	if err := reg.Register("post.validate", "v", 50, true, hookOK); err != nil {
		t.Fatalf("Register validate: %v", err)
	}

	s := NewPosts(dbh, rt, reg)
	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "hooked", Title: "H", Content: "# Raw",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	rp, err := s.GetRendered(hooks.Context{Point: "post.render"}, post.ID)
	if err != nil {
		t.Fatalf("GetRendered: %v", err)
	}
	if rp.HTML != "<p>rendered:# Raw</p>" {
		t.Errorf("HTML = %q, quiere %q", rp.HTML, "<p>rendered:# Raw</p>")
	}
}

// hookrejectJS rechaza el render.
const hookRejectRender = `function(ctx, payload) { return { ok: false, error: "no render" }; }`

func TestGetRendered_HookRejects(t *testing.T) {
	dbh := freshDB(t)
	rt := hooks.NewRuntime(hooks.WithMemoryLimit(32), hooks.WithTimeout(0))
	t.Cleanup(func() { rt.Close() })
	reg := hooks.NewRegistry(rt)
	if err := reg.Register("post.render", "reject", 50, true, hookRejectRender); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := reg.Register("post.validate", "v", 50, true, hookOK); err != nil {
		t.Fatalf("Register validate: %v", err)
	}

	s := NewPosts(dbh, rt, reg)
	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "rejected", Title: "R", Content: "x",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := s.GetRendered(hooks.Context{Point: "post.render"}, post.ID); err == nil {
		t.Error("GetRendered con hook rechazando debería fallar")
	}
}

// hookFilterJS muta el HTML (content.filter).
const hookFilterJS = `function(ctx, payload) { return { ok: true, html: payload.html + " [filtered]" }; }`

func TestContentFilterApplied(t *testing.T) {
	dbh := freshDB(t)
	rt := hooks.NewRuntime(hooks.WithMemoryLimit(32), hooks.WithTimeout(0))
	t.Cleanup(func() { rt.Close() })
	reg := hooks.NewRegistry(rt)
	if err := reg.Register("post.render", "r", 50, true, `function(ctx, p) { return { ok: true, html: "<p>" + p.post.content + "</p>" }; }`); err != nil {
		t.Fatalf("Register render: %v", err)
	}
	if err := reg.Register("content.filter", "f", 50, true, hookFilterJS); err != nil {
		t.Fatalf("Register filter: %v", err)
	}
	if err := reg.Register("post.validate", "v", 50, true, hookOK); err != nil {
		t.Fatalf("Register validate: %v", err)
	}

	s := NewPosts(dbh, rt, reg)
	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "filtered", Title: "F", Content: "hola",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	rp, err := s.GetRendered(hooks.Context{Point: "post.render"}, post.ID)
	if err != nil {
		t.Fatalf("GetRendered: %v", err)
	}
	if !strings.Contains(rp.HTML, "<p>hola</p>") {
		t.Errorf("post.render HTML: %s", rp.HTML)
	}
	if !strings.Contains(rp.HTML, "[filtered]") {
		t.Errorf("content.filter no aplicado: %s", rp.HTML)
	}
}

func TestGet_StillReturnsRawContent(t *testing.T) {
	dbh := freshDB(t)
	rt := hooks.NewRuntime(hooks.WithMemoryLimit(32), hooks.WithTimeout(0))
	t.Cleanup(func() { rt.Close() })
	reg := hooks.NewRegistry(rt)
	// Registrar post.render que MUTARÍA content — Get NO debe usarlo.
	if err := reg.Register("post.render", "mutate", 50, true, `function(ctx, p) { return { ok: true, html: "MUTATED" }; }`); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := reg.Register("post.validate", "v", 50, true, hookOK); err != nil {
		t.Fatalf("Register validate: %v", err)
	}
	s := NewPosts(dbh, rt, reg)
	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "raw", Title: "R", Content: "# Markdown raw",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.Get(post.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	// Get preserva Content raw (NO ejecuta post.render).
	if got.Content != "# Markdown raw" {
		t.Errorf("Get mutó Content: %q", got.Content)
	}
}

// === C38: posts list/search (paginado + filtro) ===

func TestList_Empty(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)

	res, err := s.List(ListInput{Limit: 10, Offset: 0, Status: ""})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Items) != 0 {
		t.Errorf("Items = %d, quiere 0", len(res.Items))
	}
	if res.Total != 0 {
		t.Errorf("Total = %d, quiere 0", res.Total)
	}
	if res.HasMore {
		t.Error("HasMore=true en tabla vacía")
	}
}

func TestList_PublishedOnly(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)

	// Draft (no aparece en público).
	if _, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "draft-post", Title: "D", Content: "x",
	}); err != nil {
		t.Fatal(err)
	}
	// Published (creamos con status draft, luego Publish).
	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "pub-post", Title: "P", Content: "y",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish(hooks.Context{Point: "post.validate"}, post.ID); err != nil {
		t.Fatal(err)
	}

	res, err := s.List(ListInput{Limit: 10, Offset: 0, Status: ""})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("Items = %d, quiere 1", len(res.Items))
	}
	if res.Items[0].Slug != "pub-post" {
		t.Errorf("solo published: slug = %q", res.Items[0].Slug)
	}
}

func TestList_PaginationAndSearch(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)

	for i := 0; i < 5; i++ {
		post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
			Slug: fmt.Sprintf("post-%d", i), Title: fmt.Sprintf("Title %d", i), Content: "c",
		})
		if err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		if i%2 == 0 {
			if _, err := s.Publish(hooks.Context{Point: "post.validate"}, post.ID); err != nil {
				t.Fatalf("Publish: %v", err)
			}
		}
	}

	// Page 1: limit 2.
	res, err := s.List(ListInput{Limit: 2, Offset: 0, Status: ""})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Items) != 2 {
		t.Errorf("Items = %d, quiere 2", len(res.Items))
	}
	if !res.HasMore {
		t.Error("HasMore=false, quiere true (5 published... 3 published)")
	}
	if res.Total != 3 {
		t.Errorf("Total = %d, quiere 3", res.Total)
	}

	// Search "post-0".
	res2, err := s.List(ListInput{Limit: 10, Offset: 0, Status: "", Query: "post-0"})
	if err != nil {
		t.Fatalf("List search: %v", err)
	}
	if len(res2.Items) != 1 || res2.Items[0].Slug != "post-0" {
		t.Errorf("search post-0: %+v", res2.Items)
	}
}

func TestListRendered_AppliesHooks(t *testing.T) {
	dbh := freshDB(t)
	rt := hooks.NewRuntime(hooks.WithMemoryLimit(32), hooks.WithTimeout(0))
	t.Cleanup(func() { rt.Close() })
	reg := hooks.NewRegistry(rt)
	if err := reg.Register("post.render", "r", 50, true, `function(ctx, p) { return { ok: true, html: "<p>" + p.post.title + "</p>" }; }`); err != nil {
		t.Fatalf("Register render: %v", err)
	}
	if err := reg.Register("post.validate", "v", 50, true, hookOK); err != nil {
		t.Fatalf("Register validate: %v", err)
	}

	s := NewPosts(dbh, rt, reg)
	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "r1", Title: "RenderMe", Content: "raw",
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
		t.Fatalf("Items = %d, quiere 1", len(res.Items))
	}
	if res.Items[0].HTML != "<p>RenderMe</p>" {
		t.Errorf("HTML = %q, quiere %q", res.Items[0].HTML, "<p>RenderMe</p>")
	}
}

// === C39: content.filter XSS sanitization (fallback defensivo) ===

func TestContentFilter_XSS(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"script-tag", "<script>alert(1)</script>", ""},
		// Sanitize neutraliza el scheme javascript: → unsafe: (no escupa el path).
		{"javascript-url", "<a href='javascript:alert(1)'>x</a>", "<a href='unsafe:alert(1)'>x</a>"},
		// on* attrs con o sin comillas.
		{"onerror-quoted", `<img src=x onerror="alert(1)">`, "<img src=x>"},
		{"onerror-bare", "<img src=x onerror=alert(1)>", "<img src=x>"},
		{"safe-html", "<p>safe</p>", "<p>safe</p>"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Sanitize(c.in)
			if got != c.want {
				t.Errorf("Sanitize(%q) = %q, quiere %q", c.in, got, c.want)
			}
		})
	}
}

func TestContentFilter_ListRenderedChainEndToEnd(t *testing.T) {
	dbh := freshDB(t)
	// Registry con post.validate (obligatorio para Create) pero SIN post.render ni
	// content.filter → fallback renderMarkdown + Sanitize.
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)

	post, err := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "xss", Title: "XSS", Content: "<script>alert(1)</script># Hola",
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
		t.Fatalf("Items = %d, quiere 1", len(res.Items))
	}
	// El <script> debe ser eliminado por Sanitize (fallback).
	if strings.Contains(res.Items[0].HTML, "<script>") {
		t.Errorf("HTML contiene <script> activo: %q", res.Items[0].HTML)
	}
}

// === C40: posts filter por author + tags (migración 002) ===

func TestList_FilterByAuthor(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)

	// Dos posts con distinto author_id; ambos publicados.
	p1, _ := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "a1", Title: "A1", Content: "x", AuthorID: 1,
	})
	if _, err := s.Publish(hooks.Context{Point: "post.validate"}, p1.ID); err != nil {
		t.Fatal(err)
	}
	p2, _ := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "a2", Title: "A2", Content: "x", AuthorID: 2,
	})
	if _, err := s.Publish(hooks.Context{Point: "post.validate"}, p2.ID); err != nil {
		t.Fatal(err)
	}

	// Filtra por author_id=1.
	res, err := s.List(ListInput{Limit: 10, Status: "", AuthorID: 1})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != p1.ID {
		t.Errorf("AuthorID=1: %+v", res.Items)
	}
}

func TestList_FilterByTag(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)

	p1, _ := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "t1", Title: "T1", Content: "x",
	})
	if _, err := s.Publish(hooks.Context{Point: "post.validate"}, p1.ID); err != nil {
		t.Fatal(err)
	}
	// Insertamos tags vía SQL directo (la tabla post_tags es C40).
	for _, tag := range []string{"go", "cms"} {
		if _, err := dbh.Exec(`INSERT INTO post_tags (post_id, tag) VALUES (?, ?)`, p1.ID, tag); err != nil {
			t.Fatalf("insert tag %s: %v", tag, err)
		}
	}

	// Filtra por tag "go".
	res, err := s.List(ListInput{Limit: 10, Status: "", Tag: "go"})
	if err != nil {
		t.Fatalf("List tag go: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != p1.ID {
		t.Errorf("Tag=go: %+v", res.Items)
	}

	// Tag inexistente → 0.
	res2, err := s.List(ListInput{Limit: 10, Status: "", Tag: "inexistente"})
	if err != nil {
		t.Fatalf("List tag missing: %v", err)
	}
	if len(res2.Items) != 0 {
		t.Errorf("Tag=inexistente: quiere 0 items, %d", len(res2.Items))
	}
}

func TestList_CombinedFilters(t *testing.T) {
	dbh := freshDB(t)
	rt, reg := setupRuntime(t, hookOK)
	s := NewPosts(dbh, rt, reg)

	p1, _ := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "c1", Title: "Go CMS", Content: "x", AuthorID: 1,
	})
	if _, err := s.Publish(hooks.Context{Point: "post.validate"}, p1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := dbh.Exec(`INSERT INTO post_tags (post_id, tag) VALUES (?, ?)`, p1.ID, "go"); err != nil {
		t.Fatal(err)
	}
	// Post de otro autor con el mismo tag (no debe aparecer filtrando author).
	p2, _ := s.Create(hooks.Context{Point: "post.validate"}, CreateInput{
		Slug: "c2", Title: "Otro", Content: "x", AuthorID: 2,
	})
	if _, err := s.Publish(hooks.Context{Point: "post.validate"}, p2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := dbh.Exec(`INSERT INTO post_tags (post_id, tag) VALUES (?, ?)`, p2.ID, "go"); err != nil {
		t.Fatal(err)
	}

	// author=1 + tag=go + query="CMS" → sólo p1.
	res, err := s.List(ListInput{
		Limit: 10, Status: "", AuthorID: 1, Tag: "go", Query: "CMS",
	})
	if err != nil {
		t.Fatalf("List combined: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != p1.ID {
		t.Errorf("combined: %+v", res.Items)
	}
}
