---
type: 'Task Contract'
title: 'Security hardening — remediar findings C59 (DoS body + auth fail-open)'
description: 'Fail-fast: WithBodyLimit (MaxBytesReader) en writes + 500 cuando authRequired=true y auth=nil. No rompe oráculos frozen.'
tags: ['ccdd', 'posts', 'http', 'security', 'fail-fast', 'c60']

task: posts-security-hardening
intent: "Aplicar controles fail-fast que mitiguen los 2 findings del escaneo C59 (gopress/posts-create-no-body-size-limit MEDIUM y gopress/posts-auth-default-disabled HIGH) sin breaking changes sobre el default authRequired=false / bodyLimit=0, preservando los oráculos frozen http_test.go/hooks_test.go/posts_test.go/migrations_test.go."
target: internal/posts/http.go
signature: |
  func WithBodyLimit(maxBytes int64) Option
  func (h *Handler) AuthRequired(next http.HandlerFunc) http.HandlerFunc
test_command: "go test ./internal/posts/... -v -run TestC60"
budget:
  cyclomatic_max: 16
  nesting_max: 4
  lines_max: 240
  params_max: 3
tests: "internal/posts/security_hardening_test.go"
tests_sha256: "3a22774462b3f4d70d2278ac383f39eefa34b8000ecf9d14c31b8dbc4bdba2ae"
touch_only: ['internal/posts/http.go', 'knowledge/contracts/C60-security-hardening.md', 'docs/reports/CONTRACT-60-REPORT.md', 'CHANGELOG.md']
deps_allowed: ['std']
forbids: ['network', 'subprocess', 'llm']
---

# Contract: posts-security-hardening

## Intent
Aplicar controles **fail-fast** que mitiguen los 2 findings del escaneo C59:
1. `gopress/posts-create-no-body-size-limit` (MEDIUM, CWE-400): body sin límite en writes.
2. `gopress/posts-auth-default-disabled` (HIGH, CWE-306): fail-open silente si `AuthRequiredEnable(true)` se olvida + auth=nil.

Restricción: **NO romper** el default (`authRequired=false`, `bodyLimit=0`) ni los oráculos frozen (`http_test.go` SHA `854cbb82…`, `hooks_test.go`, `posts_test.go`, `migrations_test.go`).

## Interface
- `WithBodyLimit(maxBytes int64) Option` — nuevo option.
- `Handler.bodyLimit int64` — campo nuevo (0 = unlimited, BC).
- `AuthRequired` envuelve `r.Body` con `http.MaxBytesReader` + Content-Length check **writes-only**.
- Fail-fast preservado: `authRequired=true` + `auth=nil` → 500 "auth not configured" (path ya existente, ahora documentado/testeado).

## Invariants
- `bodyLimit=0` (default) → comportamiento idénttico pre-C60 (BC).
- `bodyLimit>0` afecta **solo** writes (`POST/PUT/PATCH/DELETE/{slug,publish,restore}`); GET/List no se corta.
- 413 (no 400) cuando `Content-Length > limit`.
- `AuthRequired` con `auth=nil` + `authRequired=true` → SIEMPRE 500 (fail-fast, no 200).

## Examples
```go
// Prod: limitar body + auth fail-closed.
h := NewHandler(s,
  WithBodyLimit(2<<20),                    // 2 MiB max / write
  WithAuth(jwtVerify),
  AuthRequiredEnable(true),                // writes requieren auth
)
// Si olvidas WithAuth frente a AuthRequiredEnable(true) → 500 fail-fast
// (no fail-open silente como documentaba C59).
```

Lista de ejemplos verificados:
- `WithBodyLimit(1024)` + POST oversized → `413` (TestC60_BodyLimitRejectsOversizedWrite).
- `AuthRequiredEnable(true)` + `WithAuth(nil)` → `500 "auth not configured"` (TestC60_AuthRequiredFailFastWhenMisconfigured).
- `WithBodyLimit(4096)` + POST under-limit → procede, no `413` (TestC60_BodyLimitAllowsUnderLimitWrite).
- `WithBodyLimit(1)` + GET → `200`, límite no afecta reads (TestC60_BodyLimitDoesNotApplyToReads).

## Do / Don't
- **Do:** proveer `WithBodyLimit` en prod (config de deploy).
- **Do:** usar `AuthRequiredEnable(true)` + `WithAuth` juntos; el orden no importa.
- **Don't:** confiar que `AuthRequiredEnable(false)` (default) protege writes — auth está OFF por diseño (BC).
- **Don't:** setear `bodyLimit` sin `MaxBytesReader` para chunked (el middleware lo cubre).

## Tests
- `internal/posts/security_hardening_test.go` (nuevo, no frozen, SHA256 `3a227744…`).
- `TestC60_BodyLimitRejectsOversizedWrite` → 413.
- `TestC60_BodyLimitAllowsUnderLimitWrite` → procede (no 413).
- `TestC60_BodyLimitDoesNotApplyToReads` → GET no afectado.
- `TestC60_AuthRequiredFailFastWhenMisconfigured` → 500 con auth=nil + authRequired=true.

## Constraints
- Go 1.22, `std` solo (sin deps nuevas).
- No tocar oráculos: `http_test.go`, `hooks_test.go`, `posts_test.go`, `migrations_test.go`.
- `go vet` + `go build ./...` clean; `go test -race` 0 races.
- **PARAR y reportar si** `go test -race` muestra data races o `preflight.py` cae de 19/19: el fail-fast no debe introducir concurrency bugs en `AuthRequired`.
