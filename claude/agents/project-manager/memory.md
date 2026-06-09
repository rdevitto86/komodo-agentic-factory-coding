# MEMORY.md Conventions

Standards for the `MEMORY.md` session cache across Komodo projects.

`MEMORY.md` is a session-continuity cache: it exists so progress is not lost when
a session ends, compacts, or is interrupted. It is git-ignored, local, and
per-project — at the root of the work repo, not the shared config repo. Not a
knowledge base and not a task tracker — for work tracked across many sessions use
`TODO.md` (see `todo.md`).

**Its existence is the on/off switch.** File present = memory enabled: agents read
it at session start and write it at checkpoints. File absent = memory disabled:
agents operate without it and **never create it**. Only the user enables memory,
by creating the file (and excluding it in `.gitignore`). A blank file counts as
enabled-but-empty.

---

## Purpose

A session can end abruptly — context fills, the user steps away, the process
dies. `MEMORY.md` is the checkpoint that lets the next session resume without
re-deriving where things stood. Treat it as working state, not history.

## Reading

If `MEMORY.md` is absent, memory is disabled — skip it and proceed; do not create
it. If it exists, read it at the start of a session (or after a compaction): it
tells you what was in flight, what was decided, and what comes next. Reconcile
against actual repo state before trusting it — a cache, not ground truth. A blank
file means enabled but not yet checkpointed; rebuild context from the repo and
write the first checkpoint.

## Writing

**Only when the file already exists** — never create `MEMORY.md`; its absence
means the user has memory off. When it exists, update it at any checkpoint worth
not losing: a phase boundary, a non-obvious decision, a partially-done change, or
before a long or risky operation. Keep it short and current — overwrite stale
entries rather than appending forever.

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
