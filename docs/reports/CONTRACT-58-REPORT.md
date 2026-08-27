# CONTRACT-58-REPORT — Compliance / license scan (kdd-compliance-scan)

**Estado:** ✅ VERDE
**Fecha:** 2026-08-25
**Skill:** `kdd-compliance-scan` (Capa 3D).

## Resultado
- 6 dependencias en `go.mod`; **0 riesgo de licencia**.
- 4 permissive (MIT) + 2 weak-copyleft (MPL-2.0) → todas compatibleWithProjectLicense (proyecto raíz = MIT).

## Inventario de licencias
| Dependency | Declared license | Type | Compatible con MIT |
|---|---|---|---|
| `github.com/buke/quickjs-go` v0.7.7 | MIT | permissive | ✅ |
| `github.com/golang-migrate/migrate/v4` v4.15.2 | MIT (`mit`) | permissive | ✅ |
| `github.com/mattn/go-sqlite3` v1.14.50 | MIT | permissive | ✅ |
| `github.com/hashicorp/errwrap` v1.1.0 | MPL-2.0 | weak-copyleft | ✅ |
| `github.com/hashicorp/go-multierror` v1.1.1 | MPL-2.0 | weak-copyleft | ✅ |
| `go.uber.org/atomic` v1.7.0 | MIT | permissive | ✅ |

## Methodology
- Metadata de licencias declaradas en origen (LICENSE file de cada módulo en GitHub), consultada vía `go list -m -json` + clonar el LICENSE remoto. `go-licenses` no instalado en host offline → se usó fuente pública estable (GitHub LICENSE).
- No se encontraron: strong-copyleft (GPL), unknown, proprietary, dual-license-without-permission.

## Verification
```
python scripts/validate_compliance_findings.py compliance/scan   → OK (6 findings, 0 incompatible)
python scripts/preflight.py                                     → 19/19
```

## Gate
- `validate_compliance_findings.py`: PASS (6 findings).
- Ningún hallazgo `compatibleWithProjectLicense=false`.
