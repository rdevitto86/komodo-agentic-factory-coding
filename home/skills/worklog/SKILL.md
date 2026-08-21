---
name: worklog
description: Fixed formats for the two mutable work records — BACKLOG.md and CHANGELOG.md. Story shape, blocked notes, slice-to-story join, changelog groups, version bumps.
paths: "**/BACKLOG.md, **/CHANGELOG.md"
---

# The work records

`BACKLOG.md` is what is still open. `CHANGELOG.md` is what shipped. Both live at the repo root, both are written during a build.

**The frozen specs are `docs`.** `docs/prd.md` and `docs/sdd.md` are approved once and never edited by a build — load `docs` for those. The split matters at load time: a phase that only records work should not pay for the spec templates.

**Slice status never writes back into SDD §10.** The SDD defines slices, this file tracks which are open, the changelog records which are done. Three states, three files, no write-back.

---

## Slice → story — the join

**This is the step that turns a design into executable work.** A slice describes a buildable unit; a story is one agent's turn at it. Without this mapping the SDD and the backlog are two decompositions that never meet, and the plan dies in the doc.

| SDD §10 column | Becomes, in `BACKLOG.md` |
|---|---|
| `ID` | The slice ID on the story line |
| `Delivers` | The story text |
| `Depends on` | `(after: <slice-id>)` |
| `Done when` | The command after `→` |
| `Satisfies` | Not carried — trace it through the slice ID |

- **One slice yields one behavior story plus its test stories**, and every one of them carries the same slice ID. That ID is the only thread back to the PRD requirement.
- **Test stories are decomposed here, never invented later.** If a slice touches a file type `sdlc` defines a tier for and has no test story, the decomposition is incomplete — say so rather than filling the gap during implementation.
- **A slice with no `Done when` command cannot become a story.** Fix the SDD or flag it; never guess a command.

---

## `BACKLOG.md` — the open-work queue

**A dashboard, not a log.** Open work only. No dates, no completed items, no history, no checkboxes — `CHANGELOG.md` and git carry those.

Hierarchy is fixed: **target state (version) → domain → story.**

```markdown
# Backlog
Severity: [C] Critical · [H] High · [M] Medium · [L] Low · In progress: `[WIP]`

## Now — V1
### Create + fetch
- [C] Idempotent POST /orders · M · S2 → `go test ./orders/...`
- [M] Tests: unit + component coverage · S · S2 → `go test ./orders/...`

### Refunds
- [H] POST /orders/:id/refund · M · S4 → `go test ./refund/...`

### Order lifecycle
- [M] e2e: order create → refund · M · S7 → `make e2e` (after: S4)
```

Story line shape: `- [SEV][WIP] <text> · <size> · <slice-id> → \`<done when>\``

**A story with no slice omits the ID** — chores, closeout stories, and anything that predates the SDD. The `Done when` command is never optional.

### Rules

- **No checkboxes, ever.** `- [ ]`/`- [x]` are never written — a line's presence means it is open. Marking one "done" instead of deleting it is the exact drift this file prevents.
- **Target states** are `## Now — V1`, `## Next`, `## Later`. Nothing is scheduled by date.
- **Domains** are `Cross-Cutting` or a feature/route/screen/stack/queue name **inside this one service** — never another service's name.
- **Stories are flat under their domain** — no phase, no numbering. Default is parallel.
- **`(after: <slice-id>)` is the only sequencing the file encodes**, and it comes from the slice's `Depends on`. One agent choosing to work two domains back-to-back is a work-order choice, not a file property.
- **Every domain with behavior stories carries its own `Tests:` story.** That is the merge gate; integration, smoke, e2e, and perf get their own story. `sdlc` defines the tiers.
- **Every target state's `Cross-Cutting` domain carries four standing closeout stories** — `Security review`, `Bug sweep`, `Code smell`, `Performance`. They run last: delete the target state's header only once every other story is gone and these four are too.
- **Every story carries a severity tag and a relative size** (`S`/`M`/`L`). Break an XL down before writing it.
- **`[WIP]` sits right after the severity tag** — `- [C][WIP] Idempotent POST /orders · M · S2 → ...`. Remove it in the same change that deletes the line.
- **Delete the line in the same change that verifies it complete.** An absent line is the record.
- **Never dump audit or review findings straight in.** Report them; the user decides what becomes a line.

### A blocked story

**Stopping is a result, not a failure to report.** Keep the line, add `[BLOCKED]`, and indent the reason directly beneath it:

```markdown
- [H][BLOCKED] POST /orders/:id/refund · M · S4 → `go test ./refund/...`
  - Blocked: the SDK's `refund.Client` has no idempotency-key parameter at the
    pinned version, so a retry double-refunds. Confirmed at
    `vendor/forge/refund/client.go:88`. Needs an SDK change or a written
    decision to accept the risk.
```

**Four sentences maximum, and it must carry a `file:line`.** A blocked note without a citation is a guess.

---

## `CHANGELOG.md` — the shipped record

Repo root, not `docs/` — it is a published artifact, and release tooling and readers both expect it there.

```markdown
# Changelog

Notable changes to this project. Format follows Keep a Changelog; versions follow SemVer.

## [Unreleased]

## [0.2.0] — 2026-08-21

### Added
- Token refresh endpoint (`S2` → `AU2`, `AU3`)

### Fixed
- Rate limiter counted preflight requests against the caller's quota (`S3`)
```

- **Append-only.** Never rewrite a released section; a correction is a new entry.
- **Group under `Added` / `Changed` / `Fixed` / `Removed` / `Security`.** Omit any group with no entries.
- **Cite the slice ID**, and the PRD requirement IDs where the slice carried them. That keeps the trace from shipped code back to the reason it exists.
- **One line per entry**, written for someone who did not do the work.

### Versioning

**The `CHANGELOG.md` heading is the version source of truth.** The language manifest (`package.json`, and so on) is synced to match it, never the reverse. Go repos have no manifest, so the heading *is* the version and the user tags it.

| The change | Bump |
|---|---|
| Satisfies a PRD requirement ID | Minor |
| Fixes behavior, no new requirement ID | Patch |
| Breaks a published contract | Major |

---

