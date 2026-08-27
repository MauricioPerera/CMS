# CONTRACT-61-REPORT — Privacy / PII scan (kdd-privacy-scan)

**Estado:** ✅ VERDE
**Fecha:** 2026-08-25
**Skill:** `kdd-privacy-scan` (Capa 3D).

## Resultado
- 3 findings de PII mapeados; base legal `none-declared` en el schema DB.
- Runtime HTTP actual no recolecta PII directamente → PII es *latente* en schema.

## Mapeo de datos personales (data-flow)

| Data | Category | Collection point | Storage | Legal basis (declared) | Severity |
|---|---|---|---|---|---|
| `users.email` | personal | `db/migrations/001_init.up.sql` | SQLite `users.email` | none-declared | high |
| `users.password_hash` | personal | `db/migrations/001_init.up.sql` | SQLite `users.password_hash` | none-declared (derivado del finding email) | high |
| `sessions.id` / `sessions.user_id` | personal | `db/migrations/001_init.up.sql` | SQLite `sessions` | none-declared | high |
| `posts.author_id` | personal (FK indirecta) | `db/migrations/002_add_post_authors.up.sql` | SQLite `posts.author_id`→`users.id` | none-declared | high |
| posts handlers inputs (slug/title/content/authorId) | none | `internal/posts/http.go` | SQLite `posts` | none-declared | informational |

## Hallazgos

### `high` — PII en schema con base legal none-declared (3 findings)
- **`pf_users_email_not_operative`:** `users.email`/`password_hash`/`display_name` definidos en migración `001` pero el runtime Go no implementa auth (AuthFunc stub).
- **`pf_sessions_no_retention`:** tabla `sessions` definida pero no generada por handlers.
- **`pf_author_id_references_pii`:** `posts.author_id` FK referencia `users.id` (email/IP indirecto).

### Conclusiones
- El runtime HTTP actual (`internal/posts/http.go`) **no colecciona** email/nombre/IP de lectores/clientes → no hay base legal que declarar en el runtime activo.
- La PII es **latente** en el schema: si se activa auth/login, los findings `high` se vuelven operativos y requieren consentimiento + retención declarada.

## Remedios (post-C61)
1. Si `users`/`sessions` no es feature → `DROP` tablas + FK `author_id` (reduce superficie).
2. Si auth se implementa → declarar base legal (GDPR Art. 6.1b/6.1f/consent) + política retención + consent banner.
3. Documentar visibilidad de `posts.author_id` (público vs anónimo).

## Verification
```
python scripts/validate_privacy_findings.py privacy/scan   → OK (3 findings, policy conforme)
python scripts/preflight.py                                → 19/19
```
