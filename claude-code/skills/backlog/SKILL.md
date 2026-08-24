---
name: backlog
description: Turn a goal into a domain-scoped Target State breakdown in BACKLOG.md, or normalize an existing messy file into that shape.
argument-hint: [what you want to build, or a messy file/path to normalize]
disable-model-invocation: true
---

# Backlog planning

**The file format is not here — `worklog` owns it.** Load `worklog` before writing anything; this skill is the planning run only.

Target: **$ARGUMENTS**

You are planning, not building. Write no implementation code during this skill.

**If `$ARGUMENTS` names a messy or unstructured file** rather than a new goal, skip to Part 2.

---

# Part 1 — The planning run

## Step 1 — Ask before assuming

**Ask your questions first, in one batch, before producing any plan.** A plan built on a guess wastes more time than a question costs.

Ask only what changes the plan. Skip anything you can determine by reading the repo — read it instead.

Typical unknowns worth asking:

- **Scope boundary** — what is explicitly *not* in V1?
- **Existing surface** — is this extending something already built, or greenfield?
- **Hard constraints** — a deadline, a dependency that must land first, a decision already made.
- **Done condition** — what has to be true for this to ship?

If the repo answers a question, do not ask it. If nothing is genuinely unclear, say so and move on.

## Step 2 — Read the ground truth

Before proposing anything:

- **Read `docs/sdd.md` if it exists** for architecture, data model, and interface context. It informs the plan; it does not hand you a ready-made decomposition — that is this skill's own job.
- **Read the existing `BACKLOG.md`.** Never duplicate a story already in it.
- **Read the code that this work touches.** The current state beats any ledger.
- **Read the project `AGENTS.md`** for stack and conventions.

State plainly if the repo contradicts what the user described. That contradiction is usually the most valuable output of the whole exercise.

## Step 3 — Name the target states

Two states, no more:

- **V1** — the smallest thing that is genuinely usable. Not a prototype, not feature-complete. Lands under `## Now — V1`.
- **V2** — the next coherent increment. Lands under `## Next`.

Anything beyond V2 is speculation and does not belong in a plan. **A planning run never writes `## Later`** — that section holds parked work the user put there.

## Step 4 — Decompose

`worklog` governs shape. Four constraints are the planning run's own:

- **Cap each run at 3 domains and 6 stories per domain.** If the work genuinely exceeds that, plan V1 only and say V2 needs its own pass. An unbounded dump is what made the old ledger useless.
- **The four closeout stories are structural, not planned content** — exempt from the cap, and never write them yourself; they already live in the template's `Cross-Cutting` domain.
- **Default every story to parallel** — mark `(after: "<story text>")` only where one story genuinely cannot start before another lands, never to impose an arbitrary order.
- **Every story carries a `Done when` command.** A story nobody else can check is not planned, it is hoped for. If no command can prove it, that is the finding — say so.

## Step 5 — Present, then stop

Show the plan and **wait for approval**. Do not write to `BACKLOG.md` yet.

```markdown
## 🎯 Target states
- **V1** — <one line>
- **V2** — <one line>

## 📋 Plan
### <domain>
| Story `[sev]` `size` | Req ID | Done when |
|---|---|---|
| <what> `[H]` `M` | `CP1` | `<command>` |

## ⚠️ Risks
- **<thing>** — why it could bite

## ❓ Open
- <anything still unresolved>
```

## Step 6 — Write on approval only

Merge into `BACKLOG.md` per `worklog`. Never write before approval.

- **Append under the right target state and domain.** Never create a second domain with the same name.
- **Never remove a story you did not add** unless the user says so.
- **Never rewrite another story's `(after: ...)` tag** — that dependency was true when someone else wrote it; if it is stale, ask instead of silently dropping it.

---

# Part 2 — Normalizing an existing file

Same command, different ask: **$ARGUMENTS** names a messy or unstructured file — a backlog with no hierarchy, checkboxes, dates, or already-done items; scattered `// TODO` comments; a plain notes file.

## Step 1 — Read everything, invent nothing

Read the source file(s) in full. Every line becomes exactly one of:

- **A genuinely open story** → keep, rewritten to the `worklog` shape, no `[WIP]` unless the source says work is active.
- **Something already done** (phrased as done, or contradicted by the current code) → drop, and add it to `CHANGELOG.md` if it is not already recorded. Verify against the code before dropping — never drop on the comment's word alone.
- **A date, a name, a status log entry, a checked box** → drop; git already carries that history.
- **Too vague to act on** (no `Done when` command derivable) → keep as a `[L]` story naming exactly what is unclear. Never invent detail the source did not state.

## Step 2 — Sort into the hierarchy

Assign each surviving story a target state (default `## Now — V1` unless the source clearly marks it future work) and a domain (the feature or route it belongs to — infer from the file's path or the story's own text, never a new taxonomy, never another service's name).

Where the story satisfies a PRD requirement, attach that requirement ID. A story with no traceable requirement keeps its line and is flagged — it may be undocumented scope.

If a target state's `Cross-Cutting` domain is missing any of the four closeout stories, add the missing ones.

## Step 3 — Present, then stop

Show the normalized file as a diff against the source and **wait for approval** before writing — this replaces the user's existing file, so it is never silent.
