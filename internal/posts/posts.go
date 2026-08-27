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
// el hook post.validate. El campo metrics (C45) es opcional (nil-safe): si se
// inyecta, filterHook/Sanitize reportan strips XSS a través de él.
type Posts struct {
	dbh     *sql.DB
	rt      *hooks.Runtime
	reg     *hooks.Registry
	metrics *Metrics
}

// NewPosts construye el store. El runtime/registry pueden ser nil (desactiva hooks).
func NewPosts(dbh *sql.DB, rt *hooks.Runtime, reg *hooks.Registry) *Posts {
	return &Posts{dbh: dbh, rt: rt, reg: reg}
}

// SetMetrics inyecta el collector de observabilidad (C45). nil-safe: si m es nil
// no reporta (comportamiento preservado para tests/oráculos anteriores).
func (s *Posts) SetMetrics(m *Metrics) {
	s.metrics = m
}

// incSanitizeStripped reporta un strip de Sanitize (C45); no-op si no hay metrics.
func (s *Posts) incSanitizeStripped() {
	if s.metrics != nil {
		s.metrics.IncSanitizeStripped()
	}
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
// author_id es nullable (C40): se inserta sólo si >0.
func (s *Posts) Create(ctx hooks.Context, in CreateInput) (Post, error) {
	if in.Slug == "" || in.Title == "" || in.Content == "" {
		return Post{}, fmt.Errorf("Create: slug, title y content son requeridos")
	}
	post := map[string]any{
		"slug":     in.Slug,
		"title":    in.Title,
		"content":  in.Content,
		"status":   "draft",
		"authorId": in.AuthorID,
	}
	if err := s.validateHook(ctx, "create", post); err != nil {
		return Post{}, err
	}
	var err error
	var res sql.Result
	if in.AuthorID > 0 {
		const q = `INSERT INTO posts (slug, title, content, status, author_id) VALUES (?, ?, ?, ?, ?)`
		res, err = s.dbh.Exec(q, in.Slug, in.Title, in.Content, "draft", in.AuthorID)
	} else {
		const q = `INSERT INTO posts (slug, title, content, status) VALUES (?, ?, ?, ?)`
		res, err = s.dbh.Exec(q, in.Slug, in.Title, in.Content, "draft")
	}
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

// Delete elimina un post por ID (hard-delete). Ejecuta post.validate con
// action:"delete" antes del DELETE (C51 — consistente con C36: hook antes de cada escritura).
// Retorna sql.ErrNoRows wrapeado si el post no existe.
func (s *Posts) Delete(ctx hooks.Context, id int64) error {
	if id == 0 {
		return fmt.Errorf("Delete: id requerido")
	}
	// Verificar existencia antes del hook (evita disparar hook sobre inexistente).
	const qExists = `SELECT id FROM posts WHERE id = ?`
	var existsID int64
	if err := s.dbh.QueryRow(qExists, id).Scan(&existsID); err != nil {
		return fmt.Errorf("Delete lookup: %w", err)
	}
	post := map[string]any{"id": existsID, "action": "delete"}
	if err := s.validateHook(ctx, "delete", post); err != nil {
		return err
	}
	const q = `DELETE FROM posts WHERE id = ?`
	res, err := s.dbh.Exec(q, id)
	if err != nil {
		return fmt.Errorf("Delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("Delete: no se eliminó el post id=%d: %w", id, sql.ErrNoRows)
	}
	return nil
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

// === C38: Posts list/search (paginado + filtro por status) ===

// ListInput parametriza el listado paginado.
type ListInput struct {
	Limit    int
	Offset   int
	Status   string // "" = published (público); "draft"/"archived" para admin.
	Query    string // search en slug/title.
	AuthorID int64  // filtro por author (C40); 0 = sin filtro.
	Tag      string // filtro por tag (C40); "" = sin filtro.
}

// ListResult es una página de posts.
type ListResult struct {
	Items   []Post
	Total   int
	HasMore bool
}

// RenderedListResult es una página de posts renderizados.
type RenderedListResult struct {
	Items   []RenderedPost
	Total   int
	HasMore bool
}

// clampLimit fuerza Limit a [1, 100]; 0/default → 20.
func clampLimit(n int) int {
	const (
		def = 20
		max = 100
	)
	if n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

// buildListQuery arma el WHERE clause + args para List.
// Status="" → filtra a published (read path público); otro valor filtra exacto (admin).
func buildListQuery(in ListInput) (where string, args []any) {
	var parts []string
	if in.Status == "" {
		parts = append(parts, "status = 'published'")
	} else {
		parts = append(parts, "status = ?")
		args = append(args, in.Status)
	}
	if in.AuthorID > 0 {
		parts = append(parts, "author_id = ?")
		args = append(args, in.AuthorID)
	}
	if in.Query != "" {
		parts = append(parts, "(slug LIKE ? OR title LIKE ?)")
		args = append(args, "%"+in.Query+"%", "%"+in.Query+"%")
	}
	if in.Tag != "" {
		// EXISTS sobre la tabla join post_tags (C40 migración 002).
		parts = append(parts, "EXISTS (SELECT 1 FROM post_tags WHERE post_tags.post_id = posts.id AND post_tags.tag = ?)")
		args = append(args, in.Tag)
	}
	where = ""
	for i, p := range parts {
		if i > 0 {
			where += " AND "
		}
		where += p
	}
	return where, args
}

// scanRow escanea una fila de posts SELECT → Post.
func scanRow(row *sql.Row) (Post, error) {
	var p Post
	var createdAt, updatedAt string
	if err := row.Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.Status, &createdAt, &updatedAt); err != nil {
		return Post{}, err
	}
	p.CreatedAt = parseTime(createdAt)
	p.UpdatedAt = parseTime(updatedAt)
	return p, nil
}

// List retorna una página de posts según ListInput.
func (s *Posts) List(in ListInput) (ListResult, error) {
	in.Limit = clampLimit(in.Limit)
	if in.Offset < 0 {
		in.Offset = 0
	}
	where, args := buildListQuery(in)

	// Count total (sin LIMIT).
	var total int
	countQ := "SELECT COUNT(*) FROM posts"
	if where != "" {
		countQ += " WHERE " + where
	}
	if err := s.dbh.QueryRow(countQ, args...).Scan(&total); err != nil {
		return ListResult{}, wrapErr("List count", err)
	}

	// Page.
	rowsQ := `SELECT id, slug, title, content, status, created_at, updated_at FROM posts`
	if where != "" {
		rowsQ += " WHERE " + where
	}
	rowsQ += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, in.Limit, in.Offset)

	rows, err := s.dbh.Query(rowsQ, args...)
	if err != nil {
		return ListResult{}, wrapErr("List query", err)
	}
	defer rows.Close()

	var items []Post
	for rows.Next() {
		var p Post
		var createdAt, updatedAt string
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.Status, &createdAt, &updatedAt); err != nil {
			return ListResult{}, wrapErr("List scan", err)
		}
		p.CreatedAt = parseTime(createdAt)
		p.UpdatedAt = parseTime(updatedAt)
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, wrapErr("List rows", err)
	}

	return ListResult{
		Items:   items,
		Total:   total,
		HasMore: in.Offset+len(items) < total,
	}, nil
}

// ListRendered retorna una página de posts con HTML renderizado (C37).
func (s *Posts) ListRendered(ctx hooks.Context, in ListInput) (RenderedListResult, error) {
	lr, err := s.List(in)
	if err != nil {
		return RenderedListResult{}, err
	}
	items := make([]RenderedPost, 0, len(lr.Items))
	for _, p := range lr.Items {
		html, err := s.render(ctx, p)
		if err != nil {
			return RenderedListResult{}, fmt.Errorf("ListRendered %d: %w", p.ID, err)
		}
		items = append(items, RenderedPost{Post: p, HTML: html})
	}
	return RenderedListResult{
		Items:   items,
		Total:   lr.Total,
		HasMore: lr.HasMore,
	}, nil
}
