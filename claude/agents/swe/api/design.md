# API Design Standards

Universal REST conventions for Komodo services. Standard REST (plural-noun URLs, method semantics, status-code meanings) is assumed; this fixes the Komodo-specific contracts and the non-obvious points.

---

## 1. URLs & methods

- Plural-noun resources, nested for ownership (`/users/{id}/orders`); no verbs in paths except non-CRUD actions via POST (`/orders/{id}/cancel`). Kebab-case multi-word segments. IDs in path, filters in query.
- Standard method semantics; no GET with side effects. Status codes carry their standard meanings plus the Komodo conventions: `422` for validation failure on a well-formed request, `409` for state conflict / optimistic-lock failure. Never 200 for errors, never 500 for client mistakes.

---

## 2. Request / response format

- JSON only (`application/json`); `camelCase` fields; ISO 8601 UTC timestamps; IDs as strings.
- Paginated lists:
```json
{ "data": [...], "pagination": { "cursor": "...", "hasMore": true } }
```

---

## 3. Error envelope

Identical across all services:
```json
{ "error": { "code": "MACHINE_READABLE_CODE", "message": "safe for display", "details": {} } }
```
- `code` SCREAMING_SNAKE, stable across versions. `message` carries no internal details. `details` optional (field-level validation). Never expose stack traces, internal service names, or query strings.

---

## 4. Versioning & docs

- Version in path (`/v1/users`). Breaking changes (removing/retyping a field, removing an endpoint, changing auth) need a new version; additive changes don't. Deprecated versions send `Deprecation` + `Sunset` headers with ≥90 days notice.
- Every endpoint has an OpenAPI 3.x entry before merge — all params, body, response fields, and error codes documented. Mark deprecated fields `deprecated: true`, never silently remove.
