# Agent Rules

Universal directive for every model and every tool — Claude, GPT, Gemini, Qwen, Kimi, Llama, hosted or local. Plain markdown, no tool-specific syntax. Scoped to software and hardware engineering — the same rules apply whether the session is writing Go, wiring CDK infrastructure, or reviewing firmware.

**This file reaches every custom agent and every forked skill, not just the primary session.** A non-fork subagent and a `context: fork` skill both load the full CLAUDE.md hierarchy at startup, this file included — so a rule stated here never needs restating in an agent body.

---

## 1. How to work — propose, don't impose

- **Recommend before rewriting.** Default to a patch or a snippet, not a wholesale redo.
- **Build for the SDD's target state, not the code's current shape.** `CHANGELOG.md` is the only signal of a real constraint — empty or absent means nothing has shipped, so there is no live behavior or consumer to preserve: write the target design directly, don't patch around scaffolding or hedge on architecture that isn't real yet. Once an entry exists, prior releases are current state and the patch-first default above applies.
- **Act on reversible, local reads without asking** — search, reading, exploring. A write needs a directive verb (implement, change, edit, fix, add, remove, update, apply, delete, build) on a single, unambiguous instruction; gray-area verbs (review, assess, consider, propose, "why don't we") or a message discussing multiple options authorize analysis only — no file touched — until a directive verb or explicit go-ahead follows. Confirmation is otherwise reserved for the irreversible and the shared (an action touching another person's system, sending something outward, deleting what can't be undone), never for a directive step you can undo yourself.
- **Assume by default; state it and move.** Ask only when genuinely blocked — a decision only the user can make, or an irreversible/shared action.
- **Never resolve a capability gap by memory.** Check the real source, document, or record before designing around a limit.
- **Skill existence is settled by the available-skills listing already in context** — it merges project-local `.claude/skills/` and global `~/.claude/skills/`. Never Glob/grep the filesystem to check whether a skill exists; that only sees the local half.
- **Never expand scope.** Out-of-task work goes to `BACKLOG.md` and gets one line to the user. Default answer is no. This includes formatting and lint fixes: touch only the lines a task requires, never reflow or restyle a pre-existing line just because the file is already open — a shared file may carry another engineer's in-flight edit to that line.
- **Report honestly.** A failure, a skipped step, an unfinished part — say so plainly with the evidence.
- **This directory's own `AGENTS.md` is the fastest path to its facts** — read it before exploring.
- **A file under this toolkit's own `claude-code/hooks/` is live via symlink the instant it's saved.** `git_guard.py` blocks every write verb it recognizes (`mv`, `cp`, `tee`, a redirect, `sed -i`, `perl -i`, a `python -c` file write) onto any target in every extension the comment lint knows, plus its own document-extension set, for every agent, so the only sanctioned path for a hook-directory edit is the Edit/Write tool itself — an in-place write, not a crash-safe atomic swap. Every session, this one included, is exposed to that write's window regardless.
- **In a code repo:** file-scoped skills load themselves via `paths:`; comment and git rules are hook-enforced and self-explain on the first attempt, not restated here. Run `/workflow-loop` for anything bigger than a one-line fix — the default engineering mode, with `/workflow-loop open` as its unscripted exception.
