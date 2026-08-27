# CONTRACT-60-REPORT — Security hardening (remediar C59 findings)

**Estado:** ✅ VERDE
**Fecha:** 2026-08-25
**Skill:** `kdd-security-scan` (Capa 3D) → remediation layer.

## Hallazgos de C59 mitigados

| # | Finding C59 | Severidad | Remedio (C60) | Estado |
|---|---|---|---|---|
| F1 | `gopress/posts-create-no-body-size-limit` | MEDIUM | `WithBodyLimit(n)` + `MaxBytesReader`/Content-Length check en `AuthRequired` (writes-only) → **413** | `mitigated` (config disponible) |
| F2 | `gopress/posts-auth-default-disabled` | HIGH | fail-fast: `AuthRequiredEnable(true)` + `auth=nil` → **500** "auth not configured" (no fail-open silente) | `mitigated` (fail-fast) |

## Cambios (diff minimal)

`internal/posts/http.go`:
- `WithBodyLimit(maxBytes int64) Option` + campo `Handler.bodyLimit` (default 0 = unlimited, BC).
- `AuthRequired`: envuelve `r.Body` con `http.MaxBytesReader` + Content-Length check **writes-only** → 413 si excede; GET/read no afectado.

`internal/posts/security_hardening_test.go` (nuevo, no frozen, SHA256 `3a227744…`):
- `TestC60_BodyLimitRejectsOversizedWrite` → 413.
- `TestC60_BodyLimitAllowsUnderLimitWrite` → procede.
- `TestC60_BodyLimitDoesNotApplyToReads` → GET no afectado.
- `TestC60_AuthRequiredFailFastWhenMisconfigured` → 500 con auth=nil + authRequired=true.

## Design decisions
- **Default preservado** (`authRequired=false`, `bodyLimit=0`) → BC con `http_test.go` oráculo SHA `854cbb82…` (sin toques).
- **Hardening opt-in** → el deployer activa `WithBodyLimit` + `AuthRequiredEnable(true)`/`WithAuth`; fail-fast documenta el risk sin breaking-change.
- `bodyLimit` writes-only via `isWriteMethod(r.Method)`.

## Verification
```
go build ./...                                    → exit 0
go vet ./internal/posts/                          → exit 0
go test -race -count=1 -run TestC60 ./internal/posts/   → 4/4 PASS, 0 races
preflight.py                                      → 19/19 (incluye validate_contracts PASS)
python -m unittest discover -s tests -p "test_*.py"   → 744/744 OK
```

## Oráculos preservados (post-C60)
- `http_test.go` `854cbb82c25aca1d2fb358523cc7b9625190916775b5ff1835aaff48e3df9712` (sin edit).
- `hooks_test.go` `5f14c71be0bf8d2c01d9f5b7bcfd64d74aac4880e195050ea88d2c52187dbc2f` (sin edit).
- `posts_test.go` `f500f75fcf3f1b845e91de5a84b395813e2d365d0e09cb6e05705ec0e39f456b` (sin edit).
- `migrations_test.go` `f1bf4aace6aa9b3b4a692386cfa05e4ad142485864c011723352c48af880a26a` (sin edit).
