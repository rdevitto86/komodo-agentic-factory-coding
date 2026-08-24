---
name: standards-security
description: Security baseline benchmarked on the OWASP Top 10 — secrets, injection and XSS, CSRF, authn/JWT, authz, crypto, PII, dependencies. Load before touching an auth path, a query, a template, a secret, or any external boundary, and before any security review, bug sweep, or dependency review.
paths: "**/auth/**, **/*auth*, **/middleware/**, **/session*, **/token*, **/crypto/**, **/.env*, **/secrets/**, **/*.sql, **/migrations/**, **/*.tf"
---

# Security

**The OWASP Top 10 is the benchmark.** It is the coverage floor for any review and the vocabulary for any finding — name the category, and severity and remediation are already understood. OWASP ASVS is the depth benchmark when a finding needs a defensible pass/fail. Never cite a rank or edition from memory; the list is re-ranked and merged between editions, so read the current one if the rank matters.

**[review.md](review.md)** carries the procedure — attack-surface enumeration, the severity bar, the per-vector sweep, and the report shape. Load it for a security review, an attack-vector review, a bug sweep, or a dependency check.

## Secrets

- **No hardcoded secrets, tokens, or credentials** in source, config, IaC, or a container layer.
- **Secret values resolve at runtime from a secrets manager.** An env var may name *where* to look; it never carries the value in production.
- **Rotate immediately on suspected exposure**, and treat git history as exposure — a rotated key still reachable in an earlier commit is still leaked.
- **Never log** secrets, tokens, PII, or full bodies containing sensitive fields. Redact at the logger, not at each call site.
- **`.env` is local-dev only** — never committed, always gitignored.

## Input validation

- **Validate and sanitise at every system boundary** — API edge, queue consumer, upload handler.
- **Server-side validation is mandatory.** Client-side validation is a UX concern only.
- **Check type, format, length, range, and allowed character set.**
- **Reject unexpected fields at untrusted external edges** (public API, upload, webhook). Internal and versioned payloads tolerate unknown fields for forward compatibility — reject at the perimeter, tolerate behind it.
- **Never pass unvalidated input** to a query, shell command, path, URL, deserialiser, or template renderer.

## Output encoding and XSS

- **XSS is an output bug.** Encode at the point of rendering, per context — HTML body, attribute, URL, JS, and CSS each need a different encoder. Input filtering is a second layer, never the first.
- **Every raw-HTML escape hatch needs a sanitiser or a justification** — `innerHTML`, `dangerouslySetInnerHTML`, `v-html`, `{@html}`, and any template `raw`/`safe` marker.
- **Require a CSP with no `unsafe-inline` and no `unsafe-eval`.** It is the backstop that downgrades an XSS, never the fix.
- **Ship the header set** — HSTS, `nosniff`, a frame-ancestors policy, and a referrer policy.

## Authentication and authorization

- **Every endpoint requires auth** unless explicitly documented as public. Deny by default; the public list is the allowlist.
- **Authorization checks live at the service layer**, not only at the gateway, and check ownership per resource on every request.
- **Least privilege** for every service account, IAM role, and user role. Never bind a request body straight onto a model — mass assignment sets `role` and `owner_id` for free.
- **Short-lived session tokens** with refresh rotation, a server-side revocation path, and identifier rotation on every privilege change.
- **MFA for admin access to production.** Rate-limit and lock out every credential endpoint — login, refresh, reset, MFA.

## Tokens and sessions

- **Pin the expected JWT algorithm server-side.** Never let the token's own `alg` header select the verifier, and never resolve a key from an unvalidated `kid`, `jku`, or `x5u`.
- **Verify `iss`, `aud`, `exp`, and `nbf` on every request.** A decode-without-verify call outside a test is a defect.
- **A claim is only as trustworthy as its signature check** — never authorize on an unverified `role` or `tenant_id`.
- **Cookie flags are non-negotiable** — `HttpOnly`, `Secure`, `SameSite` (`Strict` for admin), explicit `Path`.
- **Every mutating route needs anti-CSRF** when authenticated by an ambient credential. `SameSite` mitigates; a session-bound token is the control. Never route a state change through `GET`.

## Cryptography

- **Never implement a primitive, mode, or protocol.** Use the platform library at its defaults.
- **AEAD only** (GCM, ChaCha20-Poly1305). No ECB, no unauthenticated CBC. No MD5 or SHA-1 in a security context.
- **Passwords use a memory-hard KDF** — argon2id, scrypt, or bcrypt at a current cost. SHA-family password hashing is critical, salted or not.
- **CSPRNG for every token, salt, IV, and security identifier.** Never reuse a nonce under a fixed key; never hardcode one.
- **Compare secrets in constant time** — tokens, HMACs, reset codes, signatures.

## Dependencies

**A dependency manifest no longer auto-loads this skill.** Touching `go.mod` or `package.json` is not a security event — a scanner reads the manifest better than a model does, and loading a 90-line skill on every manifest edit cost far more than it caught. Load this deliberately for a dependency review.

- **Vulnerability and secret scanning in CI on every change.** The command belongs to the language skill (`standards-go`, `standards-typescript`, `standards-python`); `standards-cicd` defines the gate it blocks.
- **No merging with known high or critical CVEs** without a recorded exception carrying an owner, an expiry, and a compensating control.
- **Lock versions** — no floating ranges in production dependencies, a committed lockfile the install step honours, base images pinned by digest.
- **Review transitive dependencies** when adding a direct one, plus provenance and install-time scripts — a post-install hook runs with CI credentials.

## Data handling

- **Classify before storing** — public, internal, confidential, restricted.
- **PII encrypted at rest and in transit.** TLS 1.3 where both ends support it, 1.2 the floor; never disable certificate verification outside a local fixture.
- **Define retention before storing** any user data.
- **Deletion requests reach backups and derived datasets**, not just the primary store.
- **Never copy production data into a lower environment** without scrubbing PII first.

## OWASP Top 10 mitigations

Categories by name, no ranks — OWASP re-ranks and merges between editions. Read the current list if a rank matters.

| Category | Mitigation |
|---|---|
| Broken access control | Authorization at the service layer, per-resource ownership, tenant scoping in the query |
| Injection | Parameter binding for queries, argument arrays for processes, contextual output encoding for XSS |
| Cryptographic failures | AEAD at rest, TLS in transit, memory-hard password KDF, constant-time comparison |
| Insecure design | Threat-model the boundary before implementing it; enumerate actors and assets |
| Security misconfiguration | Deny by default; no wildcard CORS, no `0.0.0.0/0` ingress, no debug surface in prod |
| Vulnerable and outdated components | CI scanning, committed lockfiles, digest-pinned images, prompt patching |
| Identification and auth failures | Pinned JWT algorithm, verified `iss`/`aud`/`exp`, short-lived tokens, rate-limited credential paths |
| Software and data integrity failures | Verify signatures on anything executed or deserialised; audit install-time scripts |
| Logging and monitoring failures | Signal on auth failure, authz denial, and PII access; never log the secrets themselves |
| SSRF | Allowlist outbound hosts, re-validate after DNS and each redirect, deny link-local and private ranges |

Injection covers XSS and SSRF has merged into access control in some editions — review both regardless of where the current list files them.

## Incident response

Isolate, assess blast radius, rotate every affected credential, record the timeline, then run a blameless post-mortem with a concrete prevention step.

**Notification clocks are legal, not technical** — regulator, contract, and card-scheme deadlines all differ. Read the obligation that applies; never assume a window.
