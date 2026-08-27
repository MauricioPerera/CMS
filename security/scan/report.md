# Security Review: GoPress CMS (gopress) — security scan target

## Scope

Code-level security scan of gopress repo root (C:\\Users\\Administrador\\Desktop\\poolside test\\CMS). Scope: Go application source (internal/\*, db/\*, cmd/\*), scripts/ (Python gates), and config (.gitignore/.env.example) for vulnerability patterns. Explicitly OUT of scope: vendored KDD tooling (scripts/vendor/codex-security), generated reports/docs (docs/, knowledge/), example data (src/), test fixtures, and third-party dependencies' source (supply-chain is a separate policy domain). Threat model: code executed locally (QuickJS hooks), HTTP input handling (stdlib net/http REST API), SQLite data layer, and secrets handling/configuration. No network-exposed service beyond stdlib httptest in tests.

- Scan mode: repository
- Target kind: git_revision
- Target ID: https://github.com/MauricioPerera/CMS
- Revision: 435f1f8f619d7cf9f2d23e3243fb3c25311f3c97
- Inventory strategy: repository
- Included paths: internal/, db/, go.mod, go.sum
- Excluded paths: scripts/vendor/, docs/, knowledge/, examples/, src/, tests/, accessibility/, compliance/, dependency-eol/, privacy/, test-coverage/, .github/
- Runtime or test status: not recorded

### Scan Summary

| Field | Value |
| --- | --- |
| Reportable findings | 2 |
| Severity mix | high: 1, medium: 1 |
| Confidence mix | high: 2 |
| Coverage | partial |
| Validation mode | not recorded |

Canonical artifacts: `scan-manifest.json`, `findings.json`, and `coverage.json`. This report is a deterministic projection of those files.

## Threat Model

No explicit canonical threat-model summary was recorded.

## Findings

| Finding | Severity | Confidence | Detailed write-up |
| --- | --- | --- | --- |
| [Write endpoints default to NO authentication (authRequired=false); deployments forgetting AuthRequiredEnable(true) expose Create/Update/Patch/Delete/Restore to unauthenticated users](#finding-1) | high | high | inline below |
| [Unbounded request body in POST /posts (Create) enables memory/disk exhaustion DoS](#finding-2) | medium | high | inline below |

### Confidence Scale

| Label | Meaning |
| --- | --- |
| high | Direct evidence supports the finding with no material unresolved blocker. |
| medium | Evidence supports a plausible issue, but material runtime or reachability proof remains. |
| low | Evidence is incomplete and the item is retained only for explicit follow-up. |

<a id="finding-1"></a>

### [1] Write endpoints default to NO authentication (authRequired=false); deployments forgetting AuthRequiredEnable(true) expose Create/Update/Patch/Delete/Restore to unauthenticated users

| Field | Value |
| --- | --- |
| Severity | high |
| Confidence | high |
| Confidence rationale | Verified by source: NewHandler does not set authRequired=true by default; AuthRequired short-circuits to next.ServeHTTP when !authRequired (line 500-501); rate limiter is nil unless WithRateLimiter is passed. |
| Category | broken-access-control |
| CWE | CWE-306, CWE-1188, CWE-1188 |
| Affected lines | internal/posts/http.go:500-501, internal/posts/http.go:145-155 |

#### Summary

AuthRequired returns early (line 500) when h.authRequired is false (the NewHandler default). When authRequired is false AND auth is nil, every write endpoint (POST/PUT/PATCH/DELETE /posts + /restore) executes with NO authentication — only rate limiting applies if WithRateLimiter was injected, which is also nil by default. A deployer who forgets AuthRequiredEnable(true) + WithAuth(...) leaves the full write API open. This is a fail-open default, not a fail-closed one.

#### Validation

Verified by source: NewHandler does not set authRequired=true by default; AuthRequired short-circuits to next.ServeHTTP when !authRequired (line 500-501); rate limiter is nil unless WithRateLimiter is passed. Validation details were not recorded separately.

#### Dataflow

The canonical finding records the affected path at internal/posts/http.go:500-501, internal/posts/http.go:145-155, but no expanded source-to-sink narrative was recorded.

#### Reachability

Reachability was not recorded beyond the canonical finding summary and affected locations.

#### Severity

**High** — Network-reachable, no privileges required, no user interaction; a misconfigured deploy grants unauthenticated attackers full write access to posts (create/mutate/delete content), impacting confidentiality/integrity/availability.

Mitigated by defaulting authRequired=true in NewHandler or failing closed (panic) when write endpoints are registered without an AuthFunc.

#### Remediation

Fail closed: in NewHandler, default authRequired=true for write routes OR panic when a write route is registered without WithAuth when CMS_REQUIRE_AUTH=1; alternatively introduce a separate WithWriteAuth option that is mandatory in production builds.

<a id="finding-2"></a>

### [2] Unbounded request body in POST /posts (Create) enables memory/disk exhaustion DoS

| Field | Value |
| --- | --- |
| Severity | medium |
| Confidence | high |
| Confidence rationale | Verified by source inspection: Create/Patch/Update/Delete all call json.NewDecoder(r.Body) or bind directly without http.MaxBytesReader; default http.Server.ReadHeaderTimeout is unset and no middleware wraps r.Body. |
| Category | denial-of-service |
| CWE | CWE-400, CWE-789 |
| Affected lines | internal/posts/http.go:572-578 |

#### Summary

Create decodes r.Body with json.NewDecoder without http.MaxBytesReader, so a client can stream an arbitrarily large body (e.g. PATCH/POST with a multi-GB JSON payload) that the server will buffer fully into memory before returning 400 on invalid JSON, exhausting memory or SQLite write bandwidth. No Content-Length cap or body size limit is enforced on any write handler (Create/Update/Patch/Delete/Restore).

#### Validation

Verified by source inspection: Create/Patch/Update/Delete all call json.NewDecoder(r.Body) or bind directly without http.MaxBytesReader; default http.Server.ReadHeaderTimeout is unset and no middleware wraps r.Body. Validation details were not recorded separately.

#### Dataflow

The canonical finding records the affected path at internal/posts/http.go:572-578, but no expanded source-to-sink narrative was recorded.

#### Reachability

Reachability was not recorded beyond the canonical finding summary and affected locations.

#### Severity

**Medium** — Network-reachable, no auth required to send the payload (auth is optional / default-off per the handler config), low attack complexity; impact is Availability (resource exhaustion) with no confidentiality/integrity impact.

Mitigated once http.MaxBytesReader is enforced on write handlers.

#### Remediation

Wrap r.Body with http.MaxBytesReader(w, r.Body, \<limit\>) in a shared middleware (e.g. in AuthRequired or a dedicated bodySizeLimiter) applied to all write methods; reject bodies exceeding the limit with 413 Payload Too Large before json.Decode.

## Reviewed Surfaces

| Surface | Risk Area | Outcome | Notes |
| --- | --- | --- | --- |
| HTTP write endpoints (Create/Update/Patch/Delete/Restore) — internal/posts/http.go | HTTP input handling / auth | Reported | No additional canonical notes were recorded. |
| AuthRequired middleware default (authRequired=false) — internal/posts/http.go | broken access control / auth bypass | Reported | No additional canonical notes were recorded. |
| SQLite data layer (internal/posts/posts.go) — parameterized queries | SQL injection | No issue found | No additional canonical notes were recorded. |
| QuickJS hook engine (internal/hooks) — registry-controlled source | code injection / sandbox escape | No issue found | No additional canonical notes were recorded. |
| Secrets/config handling (go.mod, scripts) — no hardcoded secrets | secret leakage | No issue found | No additional canonical notes were recorded. |
