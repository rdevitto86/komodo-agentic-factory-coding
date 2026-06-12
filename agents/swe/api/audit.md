# Skill: /api-audit

Audit an existing API against the API Blueprint and report conformance. **Report-only — never modifies the API source.**

## Usage

```
/api-audit <path-to-api> [--rows S,R,X,...] [--no-todo]
```

- `<path-to-api>` — directory of the service to audit (e.g. `apis/komodo-cart-api`). Defaults to the current directory if omitted.
- `--rows` — limit the audit to specific checklist groups or rows (e.g. `--rows R,X1` audits the Runtime group plus row X1). Audits everything if omitted.
- `--no-todo` — print the report only; skip appending findings to `TODO.md`.

---

## Before auditing anything

1. Read `claude/standards/api-blueprint.md` — §2 is the checklist you evaluate, §1 maps the target's language to its governing standards.
2. Detect the language from the target (manifest file: `go.mod`, `package.json`, `pyproject.toml`). Load the mapped language and test standards from the §1 table. For structural rows, the language standard defines pass/fail.
3. Read each standard a row cites **before** judging that row — do not audit from memory. The blueprint deliberately does not restate the rules; the cited standard is the authority.
4. Read the target's `TODO.md` (root and relevant subdirs) so you don't re-report work already tracked there.

---

## Evaluating

Walk §2 row by row, in order, for every row whose **Applies to** scope matches the target. For each row assign one verdict:

- **PASS** — the property holds. Cite the `file:line` that demonstrates it.
- **FAIL** — the property is violated or absent. Cite the `file:line` of the violation (or the directory where the artifact should exist), name the governing standard, and state the one concrete action that fixes it.
- **N/A** — the row's scope doesn't apply (e.g. P3 on a service with no database). State why in one clause.

Rules:

- Judge only against the cited standard. If a row's standard is silent on something, it passes — do not invent stricter criteria.
- One verdict per row. If a row fails in several places, cite the clearest instance and note the count (`+3 more`).
- Do not fix anything. Do not add inline TODOs to the API source. Do not touch the API's files at all.
- Stay in scope: audit the rows requested. Do not expand into adjacent refactors — surface those as separate `TODO.md` items per `~/.claude/standards/principles.md` and `CLAUDE.md`.

---

## Report format

Print a grouped table, one section per §2 group, then a summary line.

```
API Blueprint audit — <api name> (<language>)

Structure & surface
  S1  PASS  internal/ + pkg/v1/ present                     komodo-context §4
  S2  PASS  pkg/v1/exports.go                                komodo-context §4
  S3  FAIL  routes mounted at /orders, not /v1/orders        api-design §6
            → version the path: /v1/orders        main.go:42
  S4  PASS

Runtime contract
  R1  PASS  internal/handlers/health.go:12                   komodo-context §4
  R3  FAIL  handler returns bare string, not error envelope  api-design §5
            → wrap in { error: { code, message } }  internal/handlers/order.go:88
  R4  FAIL  "GetOrder: not found" — name prefix              principles §1
            → "order not found"                     internal/handlers/order.go:91 (+2 more)
  ...

Summary: 18 PASS · 4 FAIL · 2 N/A   →   4 gaps
```

Keep verdict lines terse. The fix arrow (`→`) carries the action and the `file:line`; never pad it into prose.

---

## After auditing

1. Unless `--no-todo` is set, append every FAIL to the target's `TODO.md` under `## API conformance — gaps from audit`, one item per finding in the `**Problem:** / **Action:**` format from `~/.claude/agents/project-manager/todo.md`. **No date in the section header.** If the section already exists, merge — don't duplicate items already present.
2. Print the summary line last so the count is the final thing on screen.
3. Do not commit, branch, or modify API source. The report and the `TODO.md` entries are the only outputs.
