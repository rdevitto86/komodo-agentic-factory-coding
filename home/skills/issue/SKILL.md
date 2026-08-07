---
name: issue
description: Standard Git issue format: title, severity, evidence, acceptance criteria.
user-invocable: false
---

# Issue

A template, not a workflow. Produce the markdown; the user files it. **Never run `gh issue create`, never open an issue, never edit one.**

## Title

`<area>: <verb phrase>` — no ticket prefixes, no severity in the title, no trailing period.

```
cart: 500 on empty promo code
ci: integration stage leaks testcontainers on failure
```

## Body

Only these sections, in this order. **Omit any section that has nothing real in it** — an empty heading is noise.

```markdown
**Severity:** [C] Critical · **Size:** M

## What
One or two lines. The observed behaviour, not the theory.

## Where
`internal/promo/apply.go:74` · `komodo-cart-api` · STG

## Repro
1. POST /cart/promo with `{"code": ""}`
2. Response is 500, body is empty

## Expected
400 with error code `PROMO_INVALID`.

## Evidence
Log line, trace id, failing test name, or the diff that introduced it.

## Acceptance criteria
- [ ] Empty and whitespace-only codes return 400 `PROMO_INVALID`
- [ ] Unit test covers both cases
```

## Rules

- **Severity uses the `TODO.md` tags** — `[C]` Critical · `[H]` High · `[M]` Medium · `[L]` Low. Size is S/M/L; anything XL is decomposed into separate issues first.
- **Acceptance criteria are checkable by someone else.** "Fix the bug" is not a criterion; "returns 400 `PROMO_INVALID` for an empty code" is.
- **Every behaviour issue names its test.** A criterion covering the new behaviour, at the tier the `sdlc` skill assigns it.
- **One issue, one change.** Two independent fixes are two issues, even when found in the same pass.
- **Evidence over narrative.** Paste the log line, the trace id, or the failing assertion. No speculation about root cause unless you traced it, and say which it is.
- **No ownership, no dates, no estimates in hours.**
