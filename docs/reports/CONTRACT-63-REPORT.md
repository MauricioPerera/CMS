# CONTRACT-63-REPORT — Dependency bump (apply C62 remediation)

**Estado:** ✅ VERDE (parcial — 1 de 2 bumps aplicado)
**Fecha:** 2026-08-25

## Decisión de diseño
- **C62 remediation propuso** 2 bumps `medium`: `golang-migrate v4.15.2→v4.19.1` y `go.uber.org/atomic v1.7.0→v1.11.0`.

## Cambios aplicados (C63)
| Dependency | Before | After | Go directive impact | Status |
|---|---|---|---|---|
| `go.uber.org/atomic` | v1.7.0 (indirect) | **v1.11.0** | ninguno (compatible Go 1.22) | ✅ bump aplicado |
| `golang-migrate/migrate/v4` | v4.15.2 | v4.19.1 | **sube `go 1.22`→`go 1.24.0`** ❌ | ⏸ NO bump (constraint violado) |

## Rationale: migrate bump RECHAZADO
- `golang-migrate/migrate/v4@v4.19.1` exige **Go directive ≥ 1.24**; `go get` fuerza `go.mod` a `go 1.24.0`, rompiendo el project constraint (`go 1.22` — CMS target Go 1.22 en `go.mod`).
- v4.15.2 sigue **supported** (no EOL, no unmaintained, no CVE conocida en el pin actual) → el finding C62-medium se mantiene como `low`/`informational` (tracking).
- **Fail-fast conservador:** no se modifica el toolchain del repo bajo presión de un scan. El bump de migrate requerirá un decision de proyecto (Go 1.24+) separado.

## Atomic bump VALIDADO
- `go get go.uber.org/atomic@v1.11.0` → `go.uber.org/atomic v1.11.0 // indirect`, `go 1.22` preservado.
- `go build ./...` → OK.
- `go vet ./internal/...` → OK.
- `go test -race -count=1 ./internal/...` → **3/3 ok, 0 data races**.
- Oráculos frozen preservados: `migrations_test.go` (SHA fijada) PASS con migrate v4.15.2.

## Remediation state de C62
- `go.uber.org/atomic`: **resolved** (v1.11.0).
- `golang-migrate/migrate/v4`: **deferred** (tracking finding `eol_golang-migrate_blocked_go124`, informational) — re-evaluate cuando CMS acepte Go 1.24+.

## Verification
```
python scripts/validate_dependency_eol_findings.py dependency-eol/scan  → OK (7 findings: 4 low, 2 medium→low, 1 informational)
python scripts/preflight.py                                             → 19/19
go test -race ./internal/...                                            → 3/3 ok, 0 races
```
