# Token Efficiency

Claude agent usage runs against a shared subscription. Every agent spawned, every file read unnecessarily, every bloated context passed between agents is a real cost. Treat tokens like money — spend them where they produce value, cut waste everywhere else.

---

## 1. MCP agents over Claude agents

MCP agents (local Qwen3 via the komodo bridge) run **outside Claude's context window entirely** — zero token cost. Claude agents (sonnet, opus, haiku) consume subscription tokens.

Default to MCP agents for:
- Test planning and QA (`qa`)
- Task breakdown and sprint planning (`pm`)
- Document and contract review (`lawyer`)
- Customer response drafting (`customer-servicing`)
- Marketing copy, content, and proposal drafting (`marketing`)

Only escalate to a Claude agent when the task requires deep reasoning, code-level work, or capabilities the MCP agents genuinely can't cover.

---

## 2. Compact aggressively

Long conversations accumulate context that agents must process on every turn. Compact frequently.

- Run `/compact` when a task phase is complete and the next phase is distinct
- Compact before spawning a new agent chain — don't carry stale context into fresh delegation
- After a long research or exploration phase, compact before switching to implementation
- The advisor should prompt compaction at natural breakpoints rather than letting context balloon

---

## 3. Keep context lean, avoid redundant work

Each agent receives only what it needs — no more.

- Pass the specific file path, not the whole directory; scope Glob/Grep tightly; prefer targeted `Read` with `offset`/`limit` over whole large files.
- Summarize upstream agent output before passing it downstream — never relay raw dumps.
- Don't re-read unchanged files, re-derive context already established this session, or re-run an agent on inputs it already processed — reference the prior conclusion instead.
- Check whether the codebase already has what you're about to generate.

---

## 4. Decompose tasks to minimize per-agent scope

A single agent handling a large, broad task accumulates a large context. Multiple focused agents with narrow scopes each run cheaper and in parallel.

- Break multi-file tasks into per-file agents (see `~/.claude/modes/ts/coding.md` for the swarming pattern)
- Each agent's prompt should be self-contained and minimal — give it exactly what it needs to do its job
- Avoid passing full conversation history into agent prompts; summarize the relevant decision or constraint instead

---

## 5. Model selection matters

Model tier definitions (haiku / sonnet / opus and when to use each) live in `CLAUDE.md` § Claude Code agents. Use the cheapest model that can do the job well — don't default to opus when sonnet suffices, or sonnet when haiku suffices.
