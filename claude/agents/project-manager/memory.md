# MEMORY.md Conventions

Standards for the `MEMORY.md` session cache across Komodo projects.

`MEMORY.md` is a session-continuity cache: it exists so progress is not lost when
a session ends, compacts, or is interrupted. It is git-ignored, local, and
read/written by every agent. It is not a knowledge base and not a task tracker —
for work tracked across many sessions use `TODO.md` (see `todo.md`).

It is **per-project and created on demand** — it lives at the root of whatever
repo the work is happening in, not in the shared config repo. There is no
canonical `MEMORY.md`; each project gets its own, and each project's `.gitignore`
should exclude it.

---

## Purpose

A session can end abruptly — context fills, the user steps away, the process
dies. `MEMORY.md` is the checkpoint that lets the next session resume without
re-deriving where things stood. Treat it as working state, not history.

## Reading

At the start of a session (or after a compaction), read `MEMORY.md` at the
project root first. It tells you what was in flight, what was decided, and what
comes next. Reconcile it against the actual repo state before trusting it — it is
a cache, not ground truth.

## Writing

Update `MEMORY.md` whenever you reach a checkpoint worth not losing: a phase
boundary, a non-obvious decision, a partially-done change, or before a long or
risky operation. Keep it short and current — overwrite stale entries rather than
appending forever.

Use four sections:
- **Now** — what is in progress right now and how far it got.
- **Next** — the immediate next steps to resume cleanly.
- **Decisions** — choices made this session the next session must honor, and why (briefly).
- **Watch-outs** — anything fragile, half-finished, or easy to break.

## What does NOT go here

| Belongs in `MEMORY.md` | Goes elsewhere |
|------------------------|----------------|
| In-flight working state, resume notes | Long-lived task backlog → `TODO.md` |
| Decisions made this session | Durable project documentation → real docs |
| Watch-outs on half-done work | Anything that should survive in git → commit it |

Prune aggressively. A `MEMORY.md` full of resolved, stale state is worse than an
empty one — it misleads the next session.
