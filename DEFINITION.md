# GoPress — Definición

## Qué es

CMS monolítico escrito en Go con SQLite como base de datos y plugins extensible vía
JavaScript (QuickJS embebido con CGo). Un solo binario (`go build -o gopress`).
Filosofía WordPress: hook system clásico, admin server-side con Go templates, y un
núcleo que expone esquemas de payload tipados para que los plugins backend (QuickJS)
intercepten y modifiquen el flujo de petición.

## Arquitectura

Core monolítico único binario. Se comunican entre sí dentro del proceso:

```
HTTP Server (net/http)
  │
  ├── Router
  │     │
  │     ├── Hook Dispatcher  ←─ register_hook('before_request', ...)
  │     │      │
  │     │      └── Plugins JS (QuickJS / CGo, via yosinon/xgo)
  │     │
  │     ├── Handlers Go (posts, usuarios, admin)
  │     │      │
  │     │      ├── SQLite (govlna/sqlite, puro Go)
  │     │      │     migrations versionadas en db/migrations/ (golang-migrate)
  │     │      │
  │     │      └── Renderer Go templates (server-side, HTML inyecta plugin JS browser-ready)
  │     │
  │     └── Static assets (CSS/JS admin, embebidos con go:embed)
```

**Componentes:**

1. **HTTP Server** (Go `net/http`): listener único, router con mux estándar.
2. **Router**: dispatch de requests a handlers; trigger de hooks backend (`before_request`, `after_request`) y admin hooks (`render_toolbar`, `inject_admin_scripts`) que inyectan HTML/JS al template.
3. **Hook Dispatcher** (Go → QuickJS bridge): el motor de hooks. Registra callbacks desde plugins JS, ejecuta payloads tipados (DTOs) en la VM QuickJS. Los hooks admin producen fragmentos HTML/JS browser-ready (no ejecutados en QuickJS).
4. **Handlers Go**: lógica de dominio (posts CRUD, auth, taxonomy). Persiste en SQLite.
5. **SQLite** (`govlna/sqlite`, puro Go): storage. Migraciones versionadas con `golang-migrate` en `db/migrations/`.
6. **Renderer**: Go templates (`html/template`) que renderizan el admin y las páginas públicas, inyectando los fragmentos de plugins.
7. **Plugin JS** (QuickJS): archivos `.js` cargados desde `plugins/`. Registran hooks con `register_hook(name, callback)`. El callback recibe un DTO documentado por cada hook.

**No hay core reusable + skin.** Es un monolito: `cmd/gopress/main.go` produce el binario; `internal/` contiene paquetes `router`, `hooks`, `posts`, `users`, `admin`, `render`, `db`, `plugin`. La separación lógica se mantiene por paquetes, no por librería reusable exportada.

## Capacidades objetivo

- [ ] Blog estático: posts (create/read/update/delete) con slug, título, contenido, metadata.
- [ ] Autenticación de admin: login con password (bcrypt), session cookie.
- [ ] Admin UI: lista de posts, editor de posts, settings page, inyección de scripts plugins.
- [ ] Hook system: `register_hook(name, callback)` en plugins JS, con ~8 hooks baseline (before_request, after_request, before_post_save, after_post_save, render_toolbar, inject_admin_scripts, render_settings_page, init).
- [ ] Plugin loader: carga `.js` desde `plugins/`, sandbox básico (timeout, deny network).
- [ ] Base de datos: migraciones versionadas, schema baseline (posts, users, sessions, options).
- [ ] Migraciones estructuradas: `db/migrations/001_init.up.sql` + `.down.sql`, corridas al startup.
- [ ] Tests unitarios: dominio (posts validation, slugify), hooks dispatcher, migraciones apply.
- [ ] CI: `validate.yml` con gates KDD Nivel 1 + suite 2× + cross-compile `go build`.

## Por qué es un caso válido / motivación real

Un CMS monolítico en Go con plugins JS es un punto de tensión real que WordPress (PHP) no
aborda: quienes quieren performance y portabilidad (binario único) pero también quieren el
ecosistema de plugins de WordPress (JS, no PHP). La fricción esencial es CGo (QuickJS) +
CGO_ENABLED=1 — que se asume desde el diseño. El caso de usar es: desplegar un blog/admin en
cualquier Linux con un solo binario, y permitir que terceros extiendan funcionalidad con JS
(sin tocar el core). La motivación real además del dogfooding KDD: KDD no ha sido perfilado
sobre un backend en Go (el template tooling es Python), así que este proyecto también
ejercita los gates CCDD sobre un target no-Python (el `validate_contracts.py` valida forma,
pero el `test-command-gate` corre `go test`).

## Fuera de alcance

- Frontend SPA con React/Preact/Svelte (se usa Go templates).
- Editor de posts WYSIWYG (se usa textarea server-side; el JS inyectado por plugins es browser-ready, no QuickJS).
- Migraciones código-Go inline (se usan archivos `.sql` versionados con `golang-migrate`).
- Pure Go QuickJS sin CGo (`dop254/fastjs` ES5 limitado, no sirve para plugins modernos).
- WASM para plugins (no requiere step de compilación extra — plugins son `.js` interpretados).
- Multi-tenancy / multi-blog (es un CMS monolítico de blog único).
- API REST pública (la API es interna del router; plugins acceden vía hooks, no HTTP externo).
- Deployment / Docker / CI/CD detallado (la imagen Docker final usa debian:slim, no scratch, por CGo; eso es un task contract de infra, no definición).
- Theme system (los templates están fijos; los plugins hookuean render pero no reemplazan templates).
- Tests de integración e2e con navegador (se testean handlers con httptest + los hooks con la VM QuickJS en tests Go).
