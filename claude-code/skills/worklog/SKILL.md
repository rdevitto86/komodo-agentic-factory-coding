---
name: worklog
description: Fixed formats for the two mutable work records — BACKLOG.md and CHANGELOG.md. Story shape, blocked notes, changelog groups, version bumps.
paths: "**/BACKLOG.md, **/CHANGELOG.md"
---

# The work records

`BACKLOG.md` is what is still open. `CHANGELOG.md` is what shipped. Both live at the repo root, both are written during a build.

**The frozen specs are `docs`.** `docs/prd.md` and `docs/sdd.md` are approved once and never edited by a build — load `docs` for those. The split matters at load time: a phase that only records work should not pay for the spec templates.

**Nothing writes back into the frozen specs.** A story may cite a PRD requirement ID for traceability; the PRD and SDD are otherwise untouched by a build. Test stories are decomposed here, when a story is written, never invented later — a domain with behavior stories and no `Tests:` story is an incomplete decomposition, not a gap to fill during implementation.

---

## `BACKLOG.md` — the open-work queue

**A dashboard, not a log.** Open work only. No dates, no completed items, no history, no checkboxes — `CHANGELOG.md` and git carry those.

Hierarchy is fixed: **target state (version) → domain → story.**

```markdown
# Backlog
Severity: [C] Critical · [H] High · [M] Medium · [L] Low · In progress: `[WIP]`

## Now — V1
### Create + fetch
- [C] Idempotent POST /orders · M · CP1 → `go test ./orders/...`
- [M] Tests: unit + component coverage · S → `go test ./orders/...`

### Refunds
- [H] POST /orders/:id/refund · M · CP3 → `go test ./refund/...`

### Order lifecycle
- [M] e2e: order create → refund · M → `make e2e` (after: "POST /orders/:id/refund")
```

Story line shape: `- [SEV][WIP] <text> · <size> · <req-id, optional> → \`<done when>\``

**A story with no PRD requirement ID omits it** — chores, closeout stories, and anything with no PRD-traceable business ask. The `Done when` command is never optional.

### Rules

- **No checkboxes, ever.** `- [ ]`/`- [x]` are never written — a line's presence means it is open. Marking one "done" instead of deleting it is the exact drift this file prevents.
- **Target states** are `## Now — V1`, `## Next`, `## Later`. Nothing is scheduled by date.
- **Domains** are `Cross-Cutting` or a feature/route/screen/stack/queue name **inside this one service** — never another service's name.
- **Stories are flat under their domain** — no phase, no numbering. Default is parallel.
- **`(after: "<story text>")` is the only sequencing the file encodes**, naming the story it must follow by its own text — never an ID from another document. One agent choosing to work two domains back-to-back is a work-order choice, not a file property.
- **Every domain with behavior stories carries its own `Tests:` story.** That is the merge gate; integration, smoke, e2e, and perf get their own story. `sdlc` defines the tiers.
- **Every target state's `Cross-Cutting` domain carries four standing closeout stories** — `Security review`, `Bug sweep`, `Code smell`, `Performance`. They run last, after any Deploy stories (see below): delete the target state's header only once every other story is gone and these four are too.
- **Every story carries a severity tag and a relative size** (`S`/`M`/`L`). Break an XL down before writing it.
- **`[WIP]` sits right after the severity tag** — `- [C][WIP] Idempotent POST /orders · M · S2 → ...`. Remove it in the same change that deletes the line.
- **Delete the line in the same change that verifies it complete.** An absent line is the record.
- **Never dump audit or review findings straight in.** Report them; the user decides what becomes a line.

### Foundation and Deploy edges

`Cross-Cutting` has two fixed edges. Neither changes the rules above — same flat story list, same `(after:)` sequencing, same four closeout stories last.

- **Foundation, first.** Repo skeleton, toolchain floor, container build, health endpoint — what `generate-repo` Create already seeds. On Scaffold/Refresh of a pre-existing repo these surface as real open stories instead of pre-satisfied ones.
- **Deploy, last — before the four closeout stories.** CI deploy pipeline, STG rollout, PROD rollout. The domain rule still applies: a service repo's Deploy stories cover *becoming deployable* (build, push, wire the pipeline). The cloud infra itself is a story in the infra repo's own `Cross-Cutting`, never this one.
- **A Deploy story blocked on something outside this repo is still `[BLOCKED]`, same shape as any other** — the citation just points at the other repo's record instead of a code defect:

  ```markdown
  - [H][BLOCKED] STG rollout + smoke · S → `...`
    - Blocked: depends on the infra repo's stack for this service, which has
      its own open `[BLOCKED]` story. See that repo's `BACKLOG.md`.
  ```

### A blocked story

**Stopping is a result, not a failure to report.** Keep the line, add `[BLOCKED]`, and indent the reason directly beneath it:

```markdown
- [H][BLOCKED] POST /orders/:id/refund · M · CP3 → `go test ./refund/...`
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
- Token refresh endpoint (`AU2`, `AU3`)

### Fixed
- Rate limiter counted preflight requests against the caller's quota (`S3`)
```

- **Append-only.** Never rewrite a released section; a correction is a new entry.
- **Group under `Added` / `Changed` / `Fixed` / `Removed` / `Security`.** Omit any group with no entries.
- **Cite the PRD requirement ID where the shipped story carried one.** That keeps the trace from shipped code back to the reason it exists.
- **One line per entry**, written for someone who did not do the work.

### Versioning

**The `CHANGELOG.md` heading is the version source of truth.** The language manifest (`package.json`, and so on) is synced to match it, never the reverse. Go repos have no manifest, so the heading *is* the version and the user tags it.

| The change | Bump |
|---|---|
| Satisfies a PRD requirement ID | Minor |
| Fixes behavior, no new requirement ID | Patch |
| Breaks a published contract | Major |

---

