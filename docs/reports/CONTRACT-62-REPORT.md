# CONTRACT-62-REPORT — Dependency EOL / supply-chain scan (kdd-dependency-eol-scan)

**Estado:** ✅ VERDE
**Fecha:** 2026-08-25
**Skill:** `kdd-dependency-eol-scan` (Capa 3D).

## Metodología
- **Herramienta:** `go list -m -versions` (off-line; `go-licenses`/`go list -u` no disponible, pero `-versions` trae los tags desde el proxy local cacheado). Cross-ref `go.mod` graph para transitividad.
- **Fuente de EOL:** `endoflife.date` no modela libs de Go individualmente → uso `go list -m -versions` como fuente autoritativa de "latest"; clasificación `supported`/`eol` se basa en: (a) latest existe, (b) el módulo publica releases recientes, (c) ausencia de advisory conocida.
- **Supply-chain vulnerabilities (CVE):** fuera de scope de ESTE scan (dominio de `go-licenses`/govulncheck sobre `go.sum`). Este scan evalúa **vigencia/maintenance health** solamente.

## Inventario (`go.mod` — 6 dependencias)

| Dependency | Used | Latest | Status | Severity | Finding ID |
|---|---|---|---|---|---|
| `github.com/buke/quickjs-go` | v0.7.7 | v0.7.7 | supported | low | `eol_buke_quickjs-go_v0.7.7` |
| `github.com/golang-migrate/migrate/v4` | v4.15.2 | v4.19.1 | supported (3 minor behind) | medium | `eol_golang-migrate…4.15.2` |
| `github.com/mattn/go-sqlite3` | v1.14.50 | v1.14.50 | supported | low | `eol_mattn_go-sqlite3_v1.14.50` |
| `github.com/hashicorp/errwrap` | v1.1.0 | v1.1.0 | supported | low | `eol_hashicorp_errwrap_v1.1.0` |
| `github.com/hashicorp/go-multierror` | v1.1.1 | v1.1.1 | supported | low | `eol_hashicorp_go-multierror_v1.1.1` |
| `go.uber.org/atomic` *(indirect)* | v1.7.0 | v1.11.0 | supported (behind, transitive) | medium | `eol_go.uber.org_atomic_v1.7.0` |

## Hallazgos

### 2 findings `medium` (desactualizados, NO EOL)
1. **`golang-migrate/migrate/v4 v4.15.2`** → `v4.19.1`: 3 minor versions behind. No es EOL ni unmaintained (v4.19.x publicado recientemente). Remedio: bump a v4.19.1 + validar con `go test -race ./internal/db` (las migraciones `001-003` deben seguir passando).
2. **`go.uber.org/atomic v1.7.0`** (indirect, transitivo de quickjs-go) → `v1.11.0`: behind. El bump requiere upgrade de quickjs-go o `go mod` manual override. No urgent (v1.7.0 sin CVE conocida).

### 4 findings `low` (current latest, supported)
- quickjs-go, go-sqlite3, hashicorp/errwrap, hashicorp/go-multierror → **current latest, no action**.

## Supply-chain (out of scope este scan)
- **CVE scanning:** dominio separado (`go-licenses`/govulncheck sobre `go.sum` hashes). No se reportan vulnerabilities aquí.
- **No hay dependencias deprecated/unmaintained** en el módulo.
- **`scripts/vendor/codex-security/`** vendorizado con schema Apache-2.0 → no es dependencia Go (excluido del inventario).

## Verification
```
go list -m -versions <dep>           # 6 deps, latest confirmado
python scripts/validate_dependency_eol_findings.py dependency-eol/scan   → OK (6 findings)
python scripts/preflight.py           → 19/19
```

## Próximos pasos (no en C62)
- Bump `golang-migrate` a v4.19.1 + `go.uber.org/atomic` a v1.11.0 en `go.mod`/`go.sum` → validar con `go test -race ./internal/db` (oráculos `migrations_test.go` fijos).
- Si se quiere supply-chain CVE scanning real → `go install golang.org/x/vuln/cmd/govulncheck` (requiere red) o `pip install cyclonedx`/`go-licenses`.
