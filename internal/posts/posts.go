// Package posts: CRUD de posts sobre SQLite (schema C1) con integración del
// hook point post.validate (runtime QuickJS C2).
//
// Contrato C36: posts-crud. La operación es atómica respecto al hook: si
// post.validate rechaza, no se escribe. Los timestamps created_at/updated_at
// los gestiona el DB (DEFAULT datetime('now')); Go los parsea a time.Time.
package posts

import (
	"database/sql"
	"fmt"
	"time"

	"gopress/internal/hooks"
)

// Post es el modelo de dominio de un post.
type Post struct {
	ID        int64
	Slug      string
	Title     string
	Content   string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateInput parametriza Post.Create.
type CreateInput struct {
	Slug     string
	Title    string
	Content  string
	AuthorID int64
}

// UpdateInput parametriza Post.Update (sólo title/content; status no se muta).
type UpdateInput struct {
	ID      int64
	Title   string
	Content string
	Status  string
}

// Posts es el store de posts. Comparte el Runtime+Registry de QuickJS (C2) para
// el hook post.validate.
type Posts struct {
	dbh *sql.DB
	rt  *hooks.Runtime
	reg *hooks.Registry
}

// NewPosts construye el store. El runtime/registry pueden ser nil (desactiva hooks).
func NewPosts(dbh *sql.DB, rt *hooks.Runtime, reg *hooks.Registry) *Posts {
	return &Posts{dbh: dbh, rt: rt, reg: reg}
}

// validateHook ejecuta post.validate antes de una escritura. Si el hook retorna
// { ok:false, error:"msg" } aborta con ese error. Si rt/reg son nil, no hay hook
// (permitido en tests unitarios sin QuickJS).
func (s *Posts) validateHook(ctx hooks.Context, action string, post map[string]any) error {
	if s.reg == nil || s.rt == nil {
		return nil
	}
	payload := map[string]any{"action": action, "post": post}
	result, err := s.reg.Call(ctx.Point, payload)
	if err != nil {
		return fmt.Errorf("post.validate: %w", err)
	}
	ok, _ := result["ok"].(bool)
	if !ok {
		msg, _ := result["error"].(string)
		if msg == "" {
			msg = "post.validate rechazó"
		}
		return fmt.Errorf("post.validate: %s", msg)
	}
	return nil
}

// Create inserta un post con status="draft" tras pasar post.validate (action:"create").
func (s *Posts) Create(ctx hooks.Context, in CreateInput) (Post, error) {
	if in.Slug == "" || in.Title == "" || in.Content == "" {
		return Post{}, fmt.Errorf("Create: slug, title y content son requeridos")
	}
	post := map[string]any{
		"slug":    in.Slug,
		"title":   in.Title,
		"content": in.Content,
		"status":  "draft",
	}
	if err := s.validateHook(ctx, "create", post); err != nil {
		return Post{}, err
	}
	const q = `INSERT INTO posts (slug, title, content, status) VALUES (?, ?, ?, ?)`
	res, err := s.dbh.Exec(q, in.Slug, in.Title, in.Content, "draft")
	if err != nil {
		return Post{}, fmt.Errorf("Create insert: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.queryByID(id)
}

// Update muta title/content del post (NO toca status). Ejecuta post.validate (action:"update").
func (s *Posts) Update(ctx hooks.Context, in UpdateInput) (Post, error) {
	if in.ID == 0 {
		return Post{}, fmt.Errorf("Update: id requerido")
	}
	post := map[string]any{
		"id":      in.ID,
		"title":   in.Title,
		"content": in.Content,
		"status":  in.Status,
	}
	// El status real proviene del DB; in.Status es solo informativo para el hook.
	const qStatus = `SELECT status FROM posts WHERE id = ?`
	var realStatus string
	if err := s.dbh.QueryRow(qStatus, in.ID).Scan(&realStatus); err != nil {
		return Post{}, fmt.Errorf("Update lookup: %w", err)
	}
	post["status"] = realStatus
	if err := s.validateHook(ctx, "update", post); err != nil {
		return Post{}, err
	}
	const q = `UPDATE posts SET title = ?, content = ?, updated_at = datetime('now') WHERE id = ?`
	if _, err := s.dbh.Exec(q, in.Title, in.Content, in.ID); err != nil {
		return Post{}, fmt.Errorf("Update: %w", err)
	}
	return s.queryByID(in.ID)
}

// Publish setea status="published" (ejecuta post.validate con action:"publish").
func (s *Posts) Publish(ctx hooks.Context, id int64) (Post, error) {
	if id == 0 {
		return Post{}, fmt.Errorf("Publish: id requerido")
	}
	p, err := s.queryByID(id)
	if err != nil {
		return Post{}, err
	}
	post := map[string]any{
		"id":      p.ID,
		"slug":    p.Slug,
		"title":   p.Title,
		"content": p.Content,
		"status":  "published",
	}
	if err := s.validateHook(ctx, "publish", post); err != nil {
		return Post{}, err
	}
	const q = `UPDATE posts SET status = 'published', updated_at = datetime('now') WHERE id = ?`
	if _, err := s.dbh.Exec(q, id); err != nil {
		return Post{}, fmt.Errorf("Publish: %w", err)
	}
	return s.queryByID(id)
}

// Get retorna un post por ID (read-only, sin hook).
func (s *Posts) Get(id int64) (Post, error) {
	return s.queryByID(id)
}

// GetBySlug retorna un post por slug (read-only, sin hook).
func (s *Posts) GetBySlug(slug string) (Post, error) {
	p, err := s.getBySlug(slug)
	if err != nil {
		return Post{}, wrapErr("GetBySlug", err)
	}
	return p, nil
}

// queryByID SELECT + reconstrucción de Post con timestamps parseados.
func (s *Posts) queryByID(id int64) (Post, error) {
	const q = `SELECT id, slug, title, content, status, created_at, updated_at FROM posts WHERE id = ?`
	row := s.dbh.QueryRow(q, id)
	var p Post
	var createdAt, updatedAt string
	if err := row.Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.Status, &createdAt, &updatedAt); err != nil {
		return Post{}, fmt.Errorf("queryByID: %w", err)
	}
	p.CreatedAt = parseTime(createdAt)
	p.UpdatedAt = parseTime(updatedAt)
	return p, nil
}

// parseTime convierte el DATETIME del SQLite (datetime('now') = "2006-01-02 15:04:05")
// a time.Time. Si no parsea, retorna zero time (no falla el query).
func parseTime(s string) time.Time {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		return time.Time{}
	}
	return t
}
