// Package posts: tests del CRUD de posts (oráculo PM-frozen, C36).
// NO es modificable por el implementador — toca internal/posts/posts.go.
package posts

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite" // register driver "sqlite".

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
	dbh, err := sql.Open("sqlite", filepath.Join(dir, "test.db"))
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
