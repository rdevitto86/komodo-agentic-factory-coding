# Security Standards

Universal security baseline for all Komodo services and projects.

---

## 1. Secrets management

- No hardcoded secrets, tokens, or credentials anywhere in source code or config files
- Use a secrets manager (AWS Secrets Manager, HashiCorp Vault, etc.) — never environment variables for sensitive values in production
- Secrets must be rotated immediately on suspected exposure
- Never log secrets, tokens, PII, or full request/response bodies containing sensitive fields
- `.env` files are for local dev only — never committed; always in `.gitignore`

---

## 2. Input validation

- Validate and sanitize all user-supplied input at system boundaries (API edge, message queue consumer, file upload handlers)
- Server-side validation is mandatory — client-side validation is a UX concern only
- Validate: type, format, length, range, and allowed character set
- Reject unexpected fields rather than silently ignoring them
- Never pass unvalidated input to a database query, shell command, or template renderer

---

## 3. Authentication & authorization

- All endpoints require authentication unless explicitly documented as public
- Authorization checks live at the service layer — not only at the API gateway
- Principle of least privilege: every service account, IAM role, and user role gets only what it needs
- Session tokens must be short-lived with refresh rotation
- MFA required for admin-level access to production systems

---

## 4. Dependency security

- Dependency vulnerability scanning runs in CI on every PR
- No merging with known high or critical CVEs without a documented exception approved by a lead
- Lock dependency versions (`go.sum`, `package-lock.json`, etc.) — no floating ranges in production deps
- Review transitive dependencies when adding a new direct dependency

---

## 5. Data handling

- Classify data before storing or transmitting: public / internal / confidential / restricted
- PII requires encryption at rest and in transit (TLS 1.2+ minimum)
- Define a retention policy before storing any user data
- Data deletion requests must be fulfilled end-to-end — including backups and derived datasets
- Never copy production data to non-production environments without scrubbing PII first

---

## 6. OWASP Top 10 — applied guidance

| Risk | Komodo mitigation |
|------|------------------|
| Injection | Parameterized queries only; no string-concatenated SQL or shell commands |
| Broken auth | Short-lived tokens, refresh rotation, revocation on logout |
| XSS | Framework escaping by default; CSP headers on all web apps; no `innerHTML` with user content |
| IDOR | Validate ownership on every resource fetch — user ID from the auth token, not the request body |
| Security misconfiguration | IaC reviewed before deploy; no default credentials; disable unused features/ports |
| Sensitive data exposure | Encrypt in transit and at rest; mask in logs; scrub from error responses |
| Broken access control | Deny by default; explicit permission grants; test unauthorized paths in QA |
| Insecure deserialization | Validate schema on all deserialized input; reject unexpected types |
| Known vulnerabilities | CVE scanning in CI; quarterly dep audit |
| Logging & monitoring | Structured logs on all auth events, errors, and admin actions; alert on anomalies |

---

## 7. Incident response

If a security incident is suspected:

1. **Isolate** — disable the affected service/endpoint/account immediately
2. **Assess** — determine blast radius: what data, which users, how long
3. **Notify** — alert relevant team leads within 1 hour of discovery; do not attempt silent remediation
4. **Document** — maintain a running timeline from discovery through resolution
5. **Rotate** — rotate all potentially exposed credentials before re-enabling anything
6. **Post-mortem** — written within 5 business days: timeline, root cause, remediation, and prevention

No security issue is fixed "quietly." All incidents are documented even if no data was exposed.
