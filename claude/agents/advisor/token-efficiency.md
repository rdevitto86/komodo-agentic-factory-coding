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
- Marketing copy and content (`marketing`)
- Sales content and proposals (`sales`)

Only escalate to a Claude agent when the task requires deep reasoning, code-level work, or capabilities the MCP agents genuinely can't cover.

---

## 2. Compact aggressively

Long conversations accumulate context that agents must process on every turn. Compact frequently.

- Run `/compact` when a task phase is complete and the next phase is distinct
- Compact before spawning a new agent chain — don't carry stale context into fresh delegation
- After a long research or exploration phase, compact before switching to implementation
- The advisor should prompt compaction at natural breakpoints rather than letting context balloon

---

## 3. Keep agent context lean

Each agent should receive only what it needs — no more.

- Pass the specific file path, not the whole directory
- Summarize upstream agent output before passing it downstream — don't relay raw dumps
- Don't re-read files that haven't changed within a session
- Scope Glob and Grep patterns tightly; wide searches on large codebases burn tokens fast
- Prefer targeted `Read` with `offset`/`limit` over reading entire large files

---

## 4. Decompose tasks to minimize per-agent scope

A single agent handling a large, broad task accumulates a large context. Multiple focused agents with narrow scopes each run cheaper and in parallel.

- Break multi-file tasks into per-file agents (see `testing-ts.md` for the swarming pattern)
- Each agent's prompt should be self-contained and minimal — give it exactly what it needs to do its job
- Avoid passing full conversation history into agent prompts; summarize the relevant decision or constraint instead

---

## 5. Avoid redundant work

- Check if the codebase already has what you're about to generate before generating it
- Don't re-derive context that was already established earlier in the session
- If an agent produced output, use it — don't re-run the same agent with the same inputs
- Cache reasoning: if you've analyzed a file or decision, reference the conclusion rather than re-analyzing

---

## 6. Model selection matters

Use the cheapest model that can do the job well:

| Task type | Model |
|-----------|-------|
| Lookup, drafting, simple Q&A | `haiku` |
| Complex technical work, implementation | `sonnet` |
| High-stakes architecture, cross-domain reasoning | `opus` |

Don't default to opus for tasks that sonnet handles equally well. Don't use sonnet when haiku suffices.
