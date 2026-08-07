---
name: api-design
description: REST conventions: URLs, status codes, error envelope, versioning, OpenAPI.
user-invocable: false
---

# API design

Standard REST is assumed. These are the fixed contracts.

## URLs and methods

- **Plural-noun resources**, nested for ownership: `/users/{id}/orders`.
- **No verbs in paths** except non-CRUD actions via POST: `/orders/{id}/cancel`.
- **Kebab-case** multi-word segments. IDs in the path, filters in the query.
- **No GET with side effects.**

Status codes carry standard meanings plus two conventions:

| Code | Meaning |
|---|---|
| `422` | Validation failure on a well-formed request |
| `409` | State conflict or optimistic-lock failure |

Never `200` for an error. Never `500` for a client mistake.

## Request and response

JSON only. `camelCase` fields. ISO 8601 UTC timestamps. IDs as strings.

Paginated lists:

```json
{ "data": [], "pagination": { "cursor": "...", "hasMore": true } }
```

## Error envelope

Identical across every service:

```json
{ "error": { "code": "MACHINE_READABLE_CODE", "message": "safe for display", "details": {} } }
```

- **`code`** is SCREAMING_SNAKE and stable across versions.
- **`message`** carries no internal detail.
- **`details`** is optional, for field-level validation.
- **Never expose** stack traces, internal service names, or query strings.

## Versioning and docs

- **Version in the path** — `/v1/users`.
- **Breaking changes need a new version**: removing or retyping a field, removing an endpoint, changing auth. Additive changes do not.
- **Deprecated versions send `Deprecation` and `Sunset` headers** with at least 90 days notice.
- **Every endpoint has an OpenAPI 3.x entry before merge** — all params, body, response fields, and error codes. Mark deprecated fields `deprecated: true` rather than removing them silently.
- **OpenAPI is the source of truth.** Generate clients per consumer; never hand-write one and never share a client package across services.
