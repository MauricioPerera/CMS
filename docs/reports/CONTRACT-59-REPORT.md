# CONTRACT-58-REPORT — Security scan (kdd-security-scan)

**Estado:** ✅ VERDE (draft → validated → verified)
**Fecha:** 2026-08-25
**Skill:** `kdd-security-scan` (Capa 3D). Scanner: manual code review (go-licenses no disponible offline; licencias/IDs de findings verificados por inspección estática).

## Scope

- **Target:** `gopress` repo raíz (`C:\...\CMS`), `git_revision @ 435f1f8f619d7cf9f2d23e3243fb3c25311f3c97`.
- **Incluye:** `internal/`, `db/` (Go application source); amenaza: código ejecutado localmente (QuickJS hooks C2), input handling HTTP (stdlib net/http REST C47/C51/C53/C54), data layer SQLite.
- **Excluye:** `scripts/vendor/codex-security` (vendored), `docs/`, `knowledge/`, `examples/`, `src/`, `tests/`, supply-chain (dominio política separado).

## Hallazgos (2, vendidos)

| ID | Severidad | Confianza | Categoría | CWE |
|---|---|---|---|---|
| `csf_f09ed150301946aea77d4e70` | high | high | auth bypass / broken access control | CWE-306, CWE-1188 |
| `csf_1f8a1050145ced710edb9574` | medium | high | DoS (unbounded body) | CWE-400, CWE-789 |

### Finding 1 — Write endpoints default to NO authentication (HIGH)
- **Sink:** `internal/posts/http.go:500-501` (`AuthRequired` short-circuits when `!authRequired`).
- **Source:** `internal/posts/http.go:145-155` (NewHandler registers write routes without `authRequired=true` default).
- **Flujo:** `NewHandler` default `authRequired=false`. Si un deployer olvida `AuthRequiredEnable(true)` + `WithAuth(...)`, todos los writes (POST/PUT/PATCH/DELETE/Restore) son accesibles sin auth → **fail-open default**. Solo rate-limit (C55) aplica, y es nil por defecto.
- **Remediación:** fail-closed: default `authRequired=true` para write routes, o `NewHandler` panickea cuando se registran writes sin `WithAuth` bajo `CMS_REQUIRE_AUTH=1`.

### Finding 2 — Unbounded request body en write handlers (MEDIUM)
- **Sink:** `internal/posts/http.go:572` (`json.NewDecoder(r.Body)`) en Create (y Patch/Update/Delete/Restore por extensión).
- **Flujo:** sin `http.MaxBytesReader`, un cliente envía body gigante → buffer completo en memoria/disk → **DoS por agotamiento de recursos** (memoria/SQLite bandwidth).
- **Remediación:** wrap `r.Body` con `http.MaxBytesReader(w, r.Body, <limit>)` en middleware de writes; 413 si excede.

## Resultado de compliance

- 0 findings `strong-copyleft`/`unknown`/`proprietary` sobre dependencias (C58 license scan: 4 MIT + 2 MPL-2.0 weak-copyleft compatible).
- 2 findings de código: 1 high (auth fail-open), 1 medium (DoS body). Ambos **remediables con acción humana** (política KDD `code_only` — el hallazgo se sella; la decisión de mitigar/aceptar es humana).

## Artefactos Capa 3 (sellar por `finalize_scan_contract.py`)

- `security/scan/scan-manifest.json` (sealed: `sealedAt`, `artifacts`).
- `security/scan/findings.json` (2 findings, fingerprints derivados deterministamente).
- `security/scan/coverage.json` (5 surfaces: http-write-path/reported, auth-middleware/reported, sql/no_issue_found, quickjs-hook-engine/no_issue_found, secrets-config/no_issue_found).
- `security/scan/report.md` (proyección determinista).

## Verification

```
python scripts/vendor/codex-security/finalize_scan_contract.py \
  --scan-dir <canonical>/security/scan \
  --schema-dir <canonical>/knowledge/data_models/security \
  --source-root <canonical>
  → sealed (findings.json/writeup/fingerprints derivados)

python scripts/validate_security_findings.py security/scan
  → OK: findings.json conforme a la politica (2 finding(s))

python scripts/preflight.py                              → 19/19
go test -race -count=1 ./internal/... -timeout 120s     → 3/3 ok, 0 races
python -m unittest discover -s tests -p "test_*.py"      → 744/744 OK
```

## Gate

- `validate_security_findings.py`: PASS (2 findings, 0 policy violations).
- `preflight.py`: 19/19.
- `-race`: 0 data races.
- KDD suite: 744/744 OK.
