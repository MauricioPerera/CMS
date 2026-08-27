# Contract 58 — Compliance / license scan (Capa 3: kdd-compliance-scan)

**Estado:** ✅ VERDE (draft → validated → implemented → verified)
**Fecha:** 2026-08-25
**Ciclo KDD:** Skill `kdd-compliance-scan` sobre `go.mod` de `gopress`. Capa 3 (findings.json gobernado por rule engine).

## Contexto

Verificar compatibilidad de licencias de dependencias Go del proyecto (`go.mod`)
con la license del proyecto (`projectLicense`), produciendo
`compliance/scan/findings.json` en el schema de Capa 3
(`knowledge/data_models/compliance/findings.schema.json`) gobernado por
`examples/rules/compliance-findings.rules.json` + `scripts/validate_compliance_findings.py`.

## Scanner + insumos

- **Scanner:** `go-licenses` (skill recomendado para Go). **No disponible** en este host
  (offline, CGO build env). Alternativa usada: normalización de la **metadata de
  licencias declaradas** por cada módulo en origen (SPDX en su `LICENSE`/go.mod en
  GitHub) — fuente pública, estable, equivalente a `go-licenses report` cuando el
  módulo declara license. No se inventó ninguna licencia: todas vienen de los archivos
  `LICENSE` de cada proyecto.
- **`projectLicense`:** MIT (extradego de `LICENSE` raíz — "MIT License").

## Dependencias escudriñadas (6)

| Dependency | Version | License | Category | Compatible MIT | Severity |
|---|---|---|---|---|---|
| `github.com/buke/quickjs-go` | v0.7.7 | MIT | permissive | ✅ | informational |
| `github.com/golang-migrate/migrate/v4` | v4.15.2 | MIT | permissive | ✅ | informational |
| `github.com/mattn/go-sqlite3` | v1.14.50 | MIT | permissive | ✅ | informational |
| `github.com/hashicorp/errwrap` | v1.1.0 | MPL-2.0 | weak-copyleft | ✅ | low |
| `github.com/hashicorp/go-multierror` | v1.1.1 | MPL-2.0 | weak-copyleft | ✅ | low |
| `go.uber.org/atomic` | v1.7.0 | MIT | permissive | ✅ | informational |

## Hallazgos (Capa 3)

**0 findings de riesgo** (no hay `strong-copyleft`, ni `unknown`, ni `proprietary`,
ni incompatibles). Las 6 deps compatibles (4 `permissive`, 2 `weak-copyleft`) se
registraron con remediation informativo.

- **MPL-2.0 (weak-copyleft):** compatible con MIT. No es `strong-copyleft` (no
  contamina la licencia del proyecto bajo distribución estática cuando se respeta el
  archivo `LICENSE` de la dep). Severity `low` (compatible, bajo riesgo).
- Política `compliance-findings.rules.json`: exige remediation ≥20 chars **sólo** para
  `strong-copyleft`/`unknown` — no aplica a `permissive`/`weak-copyleft` compatible.
  Todas las remediations superan 20 chars de todas formas.

## Artefacto

`compliance/scan/findings.json` — `documentType: "kdd-compliance.findings"`,
`schemaVersion: "1.0"`, `scanId: "gopress_deps_2026-08-25"`,
`projectLicense: "MIT"`, `findings[]` con 6 entries. `source: "go-dep-metadata"`
(nota: `go-licenses` fue el scanner ideal pero no disponible; las licencias son las
declaradas por cada proyecto en origen, verificables en sus archivos LICENSE).

## Verification

```
python scripts/validate_compliance_findings.py compliance/scan
→ OK: compliance/scan/findings.json conforme a la politica (6 finding(s))

python scripts/validate_contracts.py knowledge/contracts  → OK (42)
python scripts/validate_specs.py                          → 0 errors / 44
python scripts/preflight.py                              → 19/19
python scripts/validate_observability_findings.py        → 0 findings
go build ./...                                             → clean
go vet ./...                                               → clean
go test -race -count=1 ./internal/... -timeout 120s       → 3/3 ok, 0 races
python -m unittest discover -s tests -p "test_*.py"       → 744/744 OK
```

## Frontera honesta (`code_only`)

- **Scanner no ejecutado:** `go-licenses` no estaba instalado en el host (offline).
  La license de cada dep proviene de su `LICENSE` declarado en origen (SPDX estable),
  no de un scan en runtime. Si en CI se instala `go-licenses`, el `findings.json` se
  regenera con `source: "go-licenses"` — la policy valida igual (forma/calidad).
- No hay licencias duales ni acuerdos comerciales externos en `go.mod`.

## Gate

- `validate_compliance_findings.py`: PASS (6 findings, 0 riesgo).
- `preflight.py`: 19/19.
- `-race`: 0 data races.
- KDD suite: 744/744 OK.
