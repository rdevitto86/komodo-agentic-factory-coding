# API Design Standards

Universal REST API design standards for all Komodo services.

---

## 1. URL conventions

- Use plural nouns for resources: `/users`, `/orders`, `/products`
- Nested resources for clear ownership: `/users/{id}/orders`, `/orders/{id}/items`
- No verbs in URLs — use HTTP methods to express the action
- Kebab-case for multi-word path segments: `/shipping-addresses`, `/line-items`
- IDs in path, filters in query string: `/orders/{id}` vs `/orders?status=pending`

---

## 2. HTTP methods

| Method | Use | Idempotent |
|--------|-----|-----------|
| GET | Read resource(s) | Yes |
| POST | Create resource or non-idempotent action | No |
| PUT | Full replace of a resource | Yes |
| PATCH | Partial update | No |
| DELETE | Remove resource | Yes |

- No GET requests with side effects
- Use POST for actions that don't map cleanly to CRUD: `/orders/{id}/cancel`

---

## 3. Status codes

| Code | When to use |
|------|-------------|
| 200 OK | Successful GET, PUT, PATCH |
| 201 Created | Successful POST that creates a resource |
| 204 No Content | Successful DELETE or action with no response body |
| 400 Bad Request | Client error — malformed request, missing required field |
| 401 Unauthorized | Missing or invalid authentication |
| 403 Forbidden | Authenticated but not authorized |
| 404 Not Found | Resource doesn't exist |
| 409 Conflict | State conflict (e.g., duplicate, optimistic lock failure) |
| 422 Unprocessable Entity | Validation failure on a well-formed request |
| 429 Too Many Requests | Rate limit exceeded |
| 500 Internal Server Error | Unexpected server failure |

Never use 200 for errors, and never use 500 for client mistakes.

---

## 4. Request / response format

- JSON everywhere — `Content-Type: application/json`
- camelCase field names: `createdAt`, `userId`, `lineItems`
- Timestamps in ISO 8601 UTC: `"2024-03-15T14:32:00Z"`
- IDs as strings, not integers (safe for distributed systems and JS clients)
- Paginated list responses:

```json
{
  "data": [...],
  "pagination": {
    "cursor": "eyJpZCI6MTIzfQ==",
    "hasMore": true
  }
}
```

---

## 5. Error responses

Consistent structure across all services:

```json
{
  "error": {
    "code": "MACHINE_READABLE_CODE",
    "message": "Human-readable description of what went wrong",
    "details": {}
  }
}
```

- `code` — SCREAMING_SNAKE, stable across versions, used for programmatic handling
- `message` — safe for display, no internal implementation details
- `details` — optional, field-level validation errors or additional context
- Never expose stack traces, internal service names, or query strings to clients

---

## 6. Versioning

- Version in the URL path: `/v1/users`, `/v2/orders`
- Breaking changes require a new version — additive changes do not
- Deprecated versions: respond with `Deprecation` and `Sunset` headers
- Minimum 90 days notice before removing a version
- What counts as breaking: removing a field, changing a field type, removing an endpoint, changing auth requirements

---

## 7. Documentation

- Every endpoint must have an OpenAPI 3.x entry before the PR merges
- Document all path parameters, query parameters, request body fields, and response fields
- Include all possible error codes in the responses section
- Mark deprecated fields with `deprecated: true` — don't silently remove them
