---
name: runbook
description: Create, edit, and audit docs/runbook/ — one file per recurring failure mode or manual procedure, with the trigger, diagnosis steps, resolution, and escalation path. Authoring drafts only from an actual SDD §7 signal, §10 recovery scenario, or a procedure someone has actually run. Auditing checks the five template sections are present, every runbook is linked from the SDD's §14 References, and no runbook is orphaned against the SDD's current signals/recovery scenarios, filing findings to BACKLOG.md unless --report is passed.
argument-hint: [audit [--report]]
paths: "**/docs/runbook/**"
---

# Runbook — docs/runbook/

A runbook is the deep operational record for one recurring failure mode or manual procedure — what an on-call engineer follows at 3am without needing to reconstruct the reasoning first. `docs/spec/SDD.md` (a plain repo file — see `standards-specs`) §7 Observability names the signal that pages someone; §10 Recovery names the mechanism and RTO/RPO. A runbook exists only once a signal or a recovery path from those sections needs steps a table row can't hold — it is never a replacement for either.

`docs/runbook/*.md` is read, edited, and written with the ordinary Read/Edit/Write tools, no MCP tool and no Drive round-trip. Jargon-free, tables over prose, readable in one sitting, same bar every file under `docs/` holds.

**Two modes, one directory.** No `audit` token in `$ARGUMENTS` → Part 1, authoring. `audit` (optionally with `--report`) → Part 2, findings only.

---

# Part 1 — Authoring (create + edit)

## The fit test

Write one only when at least one is true:
- **An alert fires and someone has to act** — the SDD §7 signal has a runbook column, or should.
- **A recovery procedure in SDD §10 has more than one step**, needs judgment calls, or depends on tooling/credentials the operator won't have memorized.
- **A manual procedure repeats** — a migration, a key rotation, a failover — often enough that re-deriving it each time is itself the risk.

Never write ahead of the need: draft one only once an actual signal or recovery scenario exists to back it, never from a hypothetical.

## File location

`docs/runbook/<slug>.md` — never a top-level `/runbooks`, never the plural `docs/runbooks/`. `<slug>` is kebab-case, naming the failure mode or procedure, not the ticket (`db-failover`, not `incident-2026-08`).

Every runbook needs a citing link from `docs/spec/SDD.md`'s §14 References — this skill edits `docs/runbook/`, not the SDD itself, so add or update that link as a small courtesy edit, not a scope expansion into SDD authoring.

## Starting from the template

Copy `templates/project/docs/runbook/template.md` verbatim as the starting point for a new runbook — the same starter file `write-repo`'s Create step already scaffolds into a new repo. Never invent a different heading set. The five sections are:

- **Trigger** — the alert, symptom, or request that starts this runbook. Concrete: an alarm name, an error signature, a user-visible symptom — not "something is wrong."
- **Diagnosis** — numbered steps to confirm this is actually the failure mode this runbook covers, not a lookalike. Each step names a command or dashboard, not just an instruction to "check X."
- **Resolution** — numbered steps to fix it. Call out anything irreversible before the step that does it. State the expected outcome after each step so the operator knows whether to proceed or escalate.
- **Escalation** — who to page and when — after which failed step, or which severity — and what context to hand them.
- **Related** — links: the SDD §7/§10 section this backs, related §11 decisions, related runbooks.

## Keeping it current

Update `docs/runbook/` alongside the incident or drill that exercises it — a new file the moment a signal or recovery scenario earns one, a `Last verified` bump the moment someone actually runs it for real or in a drill. This is a repo file tree edited like any other tracked doc; there is no separate approval gate beyond the normal review the diff already gets.

---

# Part 2 — Audit

Findings only — this mode locates gaps and drift, it never fixes them. Run Part 1 to act on what it finds.

## Process

1. **List every file in `docs/runbook/`**, excluding `template.md`. No runbooks at all is not itself a finding — a repo may have no signal or recovery scenario that has yet earned one.
2. **Check the five template sections** — Trigger, Diagnosis, Resolution, Escalation, Related, all present in each file; a missing section is a finding even if the file has content otherwise.
3. **Check §14 References** — read `docs/spec/SDD.md`, then check every file present in `docs/runbook/` (excluding `template.md`) is linked from its §14 References; a file with zero hits there is orphaned.
4. **Check against the fit test** — read the SDD's §7 signals and §10 recovery scenarios; a runbook whose `Owns` line names a signal or scenario no longer present in either section is orphaned the other direction, drafted for something the SDD no longer describes.

## Report

| Kind | Where | Finding | Evidence |
|---|---|---|---|
| Missing section | `docs/runbook/db-failover.md` | no `## Escalation` heading | file body |
| Orphaned runbook (unlinked) | `docs/runbook/key-rotation.md` | not linked from SDD §14 References | SDD §14 (absent) |
| Orphaned runbook (stale) | `docs/runbook/legacy-cache-purge.md` | `Owns` names a §10 scenario no longer in the SDD | SDD §10 (absent) |

**Sev**: High (missing template section, runbook orphaned against the current SDD's signals/scenarios) · Medium (runbook not linked from §14 References) · Low (ordering, wording nits).

No findings (or no runbooks at all): state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <finding> · S → \`/runbook audit\` reports it clear`. Append under the current target state's `Cross-Cutting` domain — full story-line rules live in `write-backlog`. `--report` prints the table only; nothing is written.
