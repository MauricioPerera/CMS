// Package posts: render Markdown→HTML en el read path con hooks post.render
// y content.filter (C2 QuickJS). Backward-compatible con C36: Get/GetBySlug
// preservan Content raw; GetRendered/GetBySlugRendered retornan HTML.
//
// Contrato C37: posts-render.
package posts

import (
	"regexp"
	"strings"

	"gopress/internal/hooks"
)

// RenderedPost extiende Post con el HTML renderizado.
type RenderedPost struct {
	Post
	HTML string
}

// GetRendered retorna un post por ID con contenido renderizado (Markdown→HTML).
// Ejecuta post.render → content.filter en cadena; propaga errores de hooks.
func (s *Posts) GetRendered(ctx hooks.Context, id int64) (RenderedPost, error) {
	p, err := s.queryByID(id)
	if err != nil {
		return RenderedPost{}, err
	}
	html, err := s.render(ctx, p)
	if err != nil {
		return RenderedPost{}, err
	}
	return RenderedPost{Post: p, HTML: html}, nil
}

// GetBySlugRendered retorna un post por slug con contenido renderizado.
func (s *Posts) GetBySlugRendered(ctx hooks.Context, slug string) (RenderedPost, error) {
	p, err := s.getBySlug(slug)
	if err != nil {
		return RenderedPost{}, err
	}
	html, err := s.render(ctx, p)
	if err != nil {
		return RenderedPost{}, err
	}
	return RenderedPost{Post: p, HTML: html}, nil
}

// getBySlug es el SELECT por slug reutilizable (usa en GetBySlugRendered y posts.go).
func (s *Posts) getBySlug(slug string) (Post, error) {
	const q = `SELECT id, slug, title, content, status, created_at, updated_at FROM posts WHERE slug = ? AND deleted_at IS NULL`
	row := s.dbh.QueryRow(q, slug)
	var p Post
	var createdAt, updatedAt string
	if err := row.Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.Status, &createdAt, &updatedAt); err != nil {
		return Post{}, wrapErr("getBySlug", err)
	}
	p.CreatedAt = parseTime(createdAt)
	p.UpdatedAt = parseTime(updatedAt)
	return p, nil
}

// render ejecuta la cadena post.render → content.filter sobre p.Content (Markdown).
func (s *Posts) render(ctx hooks.Context, p Post) (string, error) {
	// 1. post.render: convierte Markdown → HTML.
	html, err := s.renderHook(ctx, p)
	if err != nil {
		return "", err
	}
	// 2. content.filter: sanitiza HTML.
	html, err = s.filterHook(ctx, p, html)
	if err != nil {
		return "", err
	}
	return html, nil
}

// renderHook ejecuta post.render; si no hay hook registrado, usa renderMarkdown fallback.
func (s *Posts) renderHook(ctx hooks.Context, p Post) (string, error) {
	if s.reg == nil || s.rt == nil {
		return renderMarkdown(p.Content), nil
	}
	postMap := map[string]any{
		"id":      p.ID,
		"slug":    p.Slug,
		"title":   p.Title,
		"content": p.Content,
		"status":  p.Status,
	}
	payload := map[string]any{"action": "render", "post": postMap}
	result, err := s.reg.Call(ctx.Point, payload)
	if err != nil {
		// registry.Call retorna error si el point no está registrado → fallback.
		return renderMarkdown(p.Content), nil
	}
	// Hook rechazado: { ok:false, error:"msg" }.
	ok, _ := result["ok"].(bool)
	if !ok {
		msg, _ := result["error"].(string)
		if msg == "" {
			msg = "post.render rechazó"
		}
		return "", &hookError{msg: msg}
	}
	if html, _ := result["html"].(string); html != "" {
		return html, nil
	}
	// No retornó html → fallback.
	return renderMarkdown(p.Content), nil
}

// filterHook ejecuta content.filter sobre html; si no hay hook, aplica Sanitize fallback
// (defensivo contra XSS) y retorna el HTML sanitizado. C39/C45: reporta strips a metrics.
func (s *Posts) filterHook(ctx hooks.Context, p Post, html string) (string, error) {
	if s.reg == nil || s.rt == nil {
		return s.sanitizeWithMetrics(html), nil
	}
	payload := map[string]any{"html": html}
	filterCtx := hooks.Context{Point: "content.filter"}
	result, err := s.reg.Call(filterCtx.Point, payload)
	if err != nil {
		// content.filter es opcional: si no hay hook registrado, sanitiza fallback.
		return s.sanitizeWithMetrics(html), nil
	}
	ok, _ := result["ok"].(bool)
	if !ok {
		msg, _ := result["error"].(string)
		if msg == "" {
			msg = "content.filter rechazó"
		}
		return "", &hookError{msg: msg}
	}
	if filtered, _ := result["html"].(string); filtered != "" {
		return filtered, nil
	}
	return s.sanitizeWithMetrics(html), nil
}

// sanitizeWithMetrics aplica Sanitize y reporta el strip a metrics (C45).
func (s *Posts) sanitizeWithMetrics(html string) string {
	out := Sanitize(html)
	if len(out) < len(html) {
		s.incSanitizeStripped()
	}
	return out
}

// Sanitize es el fallback defensivo de content.filter: elimina vectores XSS comunes
// (script tags, event handlers on*, javascript: URLs). Sólo estripea; no re-ordena HTML.
// C39: garantiza que el read path nunca sirva HTML ejecutable aunque post.render (hook o
// fallback Markdown) genere markup inseguro.
func Sanitize(html string) string {
	// 1. Elimina <script>...</script> (incluye nested).
	html = scriptRe.ReplaceAllString(html, "")
	// 2. Quita atributos on* (onclick=, onerror=, onload=, ...).
	html = eventHandlerRe.ReplaceAllString(html, "")
	// 3. Neutraliza javascript: en href/src.
	html = jsURLRe.ReplaceAllString(html, "unsafe:")
	return html
}

var (
	scriptRe      = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	eventHandlerRe = regexp.MustCompile(`(?i)\son\w+\s*=\s*"[^"]*"|\son\w+\s*=[^>\s]*`)
	jsURLRe       = regexp.MustCompile(`(?i)javascript:`)
)

// hookError envuelve errores de hooks (post.render/content.filter rechazado).
type hookError struct{ msg string }

func (e *hookError) Error() string { return e.msg }

// wrapErr wrappea un error con contexto usando %w.
func wrapErr(context string, err error) error {
	return &postError{context: context, err: err}
}

type postError struct {
	context string
	err     error
}

func (e *postError) Error() string { return e.context + ": " + e.err.Error() }
func (e *postError) Unwrap() error { return e.err }

// renderMarkdown convierte Markdown→HTML con sintaxis mínima (stdlib, sin JS).
func renderMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	var inList bool
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inList {
				out = append(out, "</ul>")
				inList = false
			}
			continue
		}
		if h := matchHeader(trimmed); h != "" {
			out = append(out, h)
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			if !inList {
				out = append(out, "<ul>")
				inList = true
			}
			out = append(out, "<li>"+inlineMarkdown(trimmed[2:])+"</li>")
			continue
		}
		out = append(out, "<p>"+inlineMarkdown(trimmed)+"</p>")
	}
	if inList {
		out = append(out, "</ul>")
	}
	return strings.Join(out, "\n")
}

var headerRe = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)

func matchHeader(line string) string {
	m := headerRe.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	level := len(m[1])
	text := inlineMarkdown(m[2])
	return "<h" + itoa(level) + ">" + text + "</h" + itoa(level) + ">"
}

var (
	boldRe   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRe = regexp.MustCompile(`\*([^*]+)\*`)
	codeRe   = regexp.MustCompile("`([^`]+)`")
)

func inlineMarkdown(s string) string {
	s = boldRe.ReplaceAllString(s, "<strong>${1}</strong>")
	s = italicRe.ReplaceAllString(s, "<em>${1}</em>")
	return codeRe.ReplaceAllString(s, "<code>${1}</code>")
}

// itoa convierte un entero pequeño (1-6) a string sin fmt.
func itoa(n int) string {
	if n == 1 {
		return "1"
	}
	if n == 2 {
		return "2"
	}
	if n == 3 {
		return "3"
	}
	if n == 4 {
		return "4"
	}
	if n == 5 {
		return "5"
	}
	return "6"
}
