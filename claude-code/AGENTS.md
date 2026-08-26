# Agent Rules

Universal directive for every model and every tool — Claude, GPT, Gemini, Qwen, Kimi, Llama, hosted or local. Plain markdown, no tool-specific syntax. Scoped to software and hardware engineering — the same rules apply whether the session is writing Go, wiring CDK infrastructure, or reviewing firmware.

**This file reaches the primary session only.** A subagent spawned via the Task/Agent tool never inherits it — it sees only its own agent-definition file. Any rule a forked agent must follow has to live in that agent's own file, restated, not assumed.

---

## 1. How to work — propose, don't impose

- **Recommend before rewriting.** Default to a patch or a snippet, not a wholesale redo.
- **Build for the SDD's target state, not the code's current shape.** `CHANGELOG.md` is the only signal of a real constraint — empty or absent means nothing has shipped, so there is no live behavior or consumer to preserve: write the target design directly, don't patch around scaffolding or hedge on architecture that isn't real yet. Once an entry exists, prior releases are current state and the patch-first default above applies.
- **Act on reversible, local work without asking.** Editing a file, running a search, reading a document, writing a scratch file — just do it. Confirmation is reserved for the irreversible and the shared (an action touching another person's system, sending something outward, deleting what can't be undone), never for a step you can undo yourself.
- **Assume by default; state it and move.** Ask only when genuinely blocked — a decision only the user can make, or an irreversible/shared action.
- **Never resolve a capability gap by memory.** Check the real source, document, or record before designing around a limit.
- **Skill existence is settled by the available-skills listing already in context** — it merges project-local `.claude/skills/` and global `~/.claude/skills/`. Never Glob/grep the filesystem to check whether a skill exists; that only sees the local half.
- **Never expand scope.** Out-of-task work goes to `BACKLOG.md` and gets one line to the user. Default answer is no.
- **Report honestly.** A failure, a skipped step, an unfinished part — say so plainly with the evidence.
- **This directory's own `AGENTS.md` is the fastest path to its facts** — read it before exploring.
- **In a code repo:** file-scoped skills load themselves via `paths:`; comment and git rules are hook-enforced and self-explain on the first attempt, not restated here. Run `/workflow-loop` for anything bigger than a one-line fix — the default engineering mode, with `/workflow-loop open` as its unscripted exception.
