# Security review

The procedure for a security sweep, an attack-vector review, or a dependency check. `SKILL.md` states the rules this file hunts for violations of.

## The benchmark

**The OWASP Top 10 is the coverage floor — a review that has not touched every category is incomplete, not thorough.** It is the shared vocabulary for reporting: name the category, and severity, scope, and remediation are already understood.

**OWASP ASVS is the depth benchmark.** The Top 10 tells you *what* to look for; ASVS L1/L2 tells you how far to verify each one. Reach for ASVS whenever a finding needs a defensible pass/fail rather than an opinion.

**Never cite a rank from memory.** OWASP re-ranks and merges categories between editions — categories have moved, and SSRF and XSS have each been folded into a broader parent. Cite the category by name; if a rank or edition matters, read the current list from `owasp.org` first.

---

## 1. Scope before reading

**Enumerate the attack surface, then review it — never open files at random.** A sweep that starts in the code inherits the code's own blind spots.

| Step | Produce |
|---|---|
| Entry points | Every route, consumer, cron, webhook |
| Trust boundaries | Where untrusted data crosses in |
| Assets | Secrets, PII, money, admin capability |

- **Entry points** — HTTP routes, queue consumers, scheduled jobs, webhooks, file uploads, CLI flags, env config, deserialisation sites, and any inbound RPC.
- **Trust boundaries** — the line between attacker-controlled data and privileged action. Every injection, SSRF, and traversal finding lives on one.
- **Assets** — what an attacker wants. Rank findings by proximity to an asset, not by how clever the bug is.
- **Actors** — anonymous, authenticated user, other tenant, internal service, admin. Most access-control bugs are found by asking "what can actor B reach that belongs to actor A?"

---

## 2. Severity bar

**Severity is exploitability × asset reach, never pattern novelty.** A hardcoded key in a test fixture and a live production key are the same pattern and different findings.

| Tier | Test |
|---|---|
| Critical | Unauth path to an asset |
| High | Authenticated privilege or tenant break |
| Medium | Needs a precondition |
| Low | Defence in depth |

- **Critical** — reachable by an anonymous attacker and reaches an asset: auth bypass, RCE, injection into a production store, live secret in the repo, token forgery.
- **High** — requires a valid account but crosses a trust boundary: IDOR, tenant leakage, privilege escalation, stored XSS, missing authz at the service layer.
- **Medium** — needs a non-trivial precondition, a chained bug, or a user interaction: reflected XSS, CSRF on a non-critical action, weak crypto with no direct path to plaintext.
- **Low** — hardening with no demonstrated path: a missing header, a verbose error, an unpinned dev dependency.
- **State the path, or drop the tier.** Every Critical and High carries a concrete attacker walkthrough — actor, entry point, boundary crossed, asset reached. No path means it is Medium at most.

---

## 3. Injection and XSS

**Every injection is the same defect: data reaching an interpreter as code.** Identify the interpreter, then confirm the data never reaches it as syntax.

| Interpreter | The only accepted fix |
|---|---|
| SQL / NoSQL | Parameter binding |
| Shell | Argument array, no shell |
| HTML / DOM | Contextual output encoding |

- **SQL and NoSQL** — parameterised queries only. Escaping helpers, quoting helpers, and ORM `raw` calls with interpolation are all findings. Identifiers that cannot be bound (table, column, direction) come from an allowlist, never from input.
- **Shell and process** — pass an argument array to the binary; never build a command string, never invoke a shell. Grep for `exec`, `system`, `popen`, `spawn`, backticks, and `shell=True`.
- **XSS is an output-encoding bug, not an input-sanitisation bug.** Encode at the point of rendering, per context — HTML body, attribute, URL, JS, and CSS each need a different encoder. Input filtering is a second layer, never the first.
- **Audit every escape hatch** — `innerHTML`, `dangerouslySetInnerHTML`, `v-html`, `{@html}`, `document.write`, template `safe`/`raw` markers, and any string concatenated into a `<script>` or `href`. Each one needs a sanitiser (DOMPurify-class) or a justification.
- **CSP is the backstop, not the fix.** Require a policy with no `unsafe-inline` and no `unsafe-eval`; a nonce or hash for anything inline. A CSP does not close an XSS finding — it downgrades it.
- **Also check** — LDAP, XPath, template-engine SSTI, log injection (CRLF into log lines), and header injection from user-supplied values.

---

## 4. CSRF and session

**Any state-changing request authenticated by an ambient credential needs CSRF defence.** Cookies and browser-attached headers are ambient; an `Authorization` header set by JS is not.

- **Require the anti-CSRF control on every mutating route** — a synchroniser token bound to the session, or a strict double-submit. Verify it is checked server-side, not merely issued.
- **`SameSite` is a mitigation, not the control.** `Lax` leaves top-level `GET` navigation exposed and does not cover same-site subdomain attackers. Pair it with a token.
- **Cookie flags are non-negotiable** — `HttpOnly`, `Secure`, `SameSite=Lax` minimum (`Strict` for admin sessions), a `__Host-` prefix where scoping allows, and an explicit `Path`.
- **Never route a mutation through `GET`.** A state change behind a safe method is a CSRF finding on its own, regardless of tokens.
- **Rotate the session identifier on privilege change** — login, step-up, and role switch. A retained identifier is session fixation.
- **Check logout and expiry actually invalidate server-side.** Clearing a cookie while the token stays valid is not a logout.

---

## 5. Authentication and JWT

**A JWT is attacker-supplied input until a signature verifies against a key you chose.** Nearly every JWT vulnerability is a verification step that was skipped or delegated to the token itself.

| Check | Failure it stops |
|---|---|
| Pinned algorithm | `alg: none`, RS→HS confusion |
| `iss` and `aud` | Token replay across services |
| `exp` and `nbf` | Indefinite validity |

- **Pin the expected algorithm server-side.** Never let the token's `alg` header select the verifier — that is `alg: none` and RS256→HS256 confusion, where a public key is used as an HMAC secret.
- **Never trust `kid`, `jku`, or `x5u` as a lookup path.** Resolve keys from a fixed allowlist or a pinned JWKS URL; an unvalidated `kid` is a traversal and SSRF primitive.
- **Verify `iss`, `aud`, `exp`, and `nbf` on every request**, with bounded clock skew. Decode-without-verify helpers (`decode`, `unsafeDecode`) appearing outside a test are a finding.
- **Keep access tokens short-lived with rotating refresh**, and hold a server-side revocation path. A stateless token with a long expiry cannot be logged out.
- **Never place authorization decisions in unverified claims** — a `role` or `tenant_id` claim is only as trustworthy as the signature check above it.
- **Passwords use a memory-hard KDF** — argon2id, scrypt, or bcrypt with a current cost. SHA-family hashing of passwords, salted or not, is Critical.
- **Rate-limit and lock out on credential endpoints** — login, refresh, reset, MFA, and any user-enumerating response. Compare tokens and reset codes in constant time.

---

## 6. Authorization

**Authentication proves who; authorization proves entitlement — verify the second exists on every route.** Gateway-level checks prove neither.

- **Check ownership per resource, per request.** An object identifier from the client is a request, not a grant. Sequential or guessable identifiers make IDOR trivially exploitable.
- **Test the actor matrix** — for each route, ask what an anonymous caller, another tenant's user, and a non-admin reach. Most High findings come from this pass.
- **Confirm tenant scoping lives in the query**, not the caller. A filter applied after fetch, or omitted on one code path, leaks across tenants.
- **Check mass assignment** — binding a request body straight onto a model lets a caller set `role`, `owner_id`, or `is_admin`. Bind to an explicit input type.
- **Deny by default.** A new route with no declared policy must fail closed; an allowlist of public routes is auditable, an implicit default is not.

---

## 7. Cryptography and secrets

**Never implement a cryptographic primitive, mode, or protocol.** Use the platform library at defaults; a bespoke construction is a finding regardless of correctness.

| Banned | Required |
|---|---|
| MD5, SHA-1 for security | SHA-256 or better |
| ECB, unauthenticated CBC | AEAD (GCM, ChaCha20-Poly1305) |
| `Math.random`-class RNG | Platform CSPRNG |

- **Randomness for tokens, salts, IVs, and identifiers comes from a CSPRNG.** Non-cryptographic PRNGs in a security context are High.
- **Never reuse a nonce or IV** with a fixed key, and never hardcode one. Derive keys with HKDF; do not use a password directly as a key.
- **Compare secrets in constant time** — tokens, HMACs, reset codes, signatures. A short-circuiting `==` is a timing oracle.
- **Scan git history, not just the tree.** A rotated key that remains reachable in an earlier commit is still exposed. Look for high-entropy strings, `BEGIN PRIVATE KEY`, cloud key prefixes, and connection strings with inline credentials.
- **Verify secrets resolve at runtime from the manager** — a committed `.env`, a baked container layer, a CI log echo, or a value in IaC plaintext are each Critical if live.
- **Confirm TLS is enforced and verified** — no disabled certificate verification, no plaintext fallback, and no self-signed acceptance outside a local fixture.

---

## 8. SSRF, traversal, and upload

**Any server-side fetch of a client-influenced URL is SSRF until proven otherwise**, including indirect forms: webhooks, PDF and image renderers, link previews, XML external entities, and OpenID discovery.

- **Allowlist the destination by host, then re-validate after DNS resolution** and again after every redirect. Blocklists are bypassed by encodings, DNS rebinding, and redirect chains.
- **Deny link-local, loopback, and private ranges explicitly** — cloud instance metadata at `169.254.169.254` is the standard escalation, plus container and orchestrator endpoints.
- **Disable external entity resolution in every XML parser**, and disable entity expansion. Prefer a parser that is safe by default.
- **Never build a filesystem path from input.** Resolve to an absolute path and confirm it is inside the intended root; check archive extraction for entries escaping the destination.
- **Uploads validate content, not the filename** — sniff the type, cap the size before buffering, generate the stored name, strip the executable bit, and serve from a path that cannot execute.
- **Never deserialise untrusted data into arbitrary types.** Use a data-only format with a declared schema; native deserialisation of attacker input is Critical.

---

## 9. Dependencies and supply chain

**Run the scan, do not read the manifest.** The command lives in the language skill (`go`, `typescript`, `python`); this file states what the result must satisfy.

| Check | Bar |
|---|---|
| Known CVEs | No High or Critical unexcepted |
| Version pinning | Lockfile present and committed |
| Reachability | Confirm before reporting |

- **Triage by reachability.** A Critical CVE in a code path the service never calls is Medium; a Medium in a request-handling path can be High. State which, with the call site.
- **Verify the lockfile is committed and honoured by CI** — a lockfile the install step ignores provides nothing. No floating ranges on production dependencies.
- **Review install-time execution** — post-install scripts, build plugins, and Makefile hooks run with developer and CI credentials. Treat a new one as a supply-chain change.
- **Pin base images by digest**, not a moving tag, and confirm the image is rebuilt on a schedule rather than only on code change.
- **Check new direct dependencies for provenance** — maintainer count, release cadence, and a name close to a popular package (typosquatting). Pull the transitive tree, not just the direct entry.
- **A recorded exception needs an owner, an expiry, and a compensating control.** An indefinite suppression is an unfixed finding.

---

## 10. Configuration, exposure, and logging

**Default-deny at every boundary, and confirm the deployed config matches the reviewed one.** Most misconfiguration findings live in IaC, not application code.

- **Reject wildcard CORS with credentials**, `0.0.0.0/0` ingress, public storage buckets, and permissive IAM (`*` action or resource).
- **Require the response header set** — HSTS, `X-Content-Type-Options: nosniff`, a frame-ancestors policy, a referrer policy, and CSP.
- **Confirm debug surfaces are off in production** — stack traces to clients, verbose errors, profiling endpoints, GraphQL introspection, directory listing, and default credentials.
- **Never log secrets, tokens, full request bodies, or raw PII.** Check redaction is applied at the logger, not at each call site, and that error objects do not carry credentials into the message.
- **Confirm the security signals exist** — auth failure, authz denial, privileged action, and PII access must each be observable. Missing signal on an auth path is a Blind Spot finding.
- **Check rate limiting beyond login** — expensive queries, exports, search, and any unauthenticated endpoint.

---

## 11. Agent and LLM surfaces

**Treat model output as untrusted input, and any content the model reads as attacker-controlled.** Applies to prompts, tool arguments, retrieved documents, and web fetches.

- **Never let retrieved or user text select a tool or authorize an action.** Authorization is decided by the caller's identity, before the model runs.
- **Validate tool arguments server-side** exactly as an HTTP body — the model is a client, not a trusted component.
- **Scope credentials per tool, at least privilege**, and never place a secret in a prompt or system message.
- **Bound the blast radius** — confirm destructive tools require an explicit approval step and cannot be chained from retrieved content.

---

## Output

Report against `SKILL.md`'s rules and the OWASP category names. Load the `adhd-format` skill before writing the report.

```markdown
## 🔒 Verdict
**<PASS / FAIL>** — <the single deciding finding, or "no Critical or High">

## 🔴 Critical and High
- **<what>** — `file:line` — <OWASP category> — <attacker path in one line>

## 📋 Findings
| Sev | What | Where |
|---|---|---|

## 📌 Not reviewed
- <surface out of scope, and why>
```

- **Every finding carries `file:line`.** No pointer, no finding.
- **Report confirmed findings only.** If exploitability is unproven, say what would confirm it and drop the tier.
- **Name what you did not review.** An unstated gap reads as a clean bill of health.
- **Cap the table at 15 rows**, then state how many were omitted.
