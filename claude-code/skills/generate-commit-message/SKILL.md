---
name: generate-commit-message
description: Write a copy-pastable commit message for the current diff — capped concise message, comma/plus-delimited secondary description.
argument-hint: []
---

# Commit message

**Inside a git repository only** — if the working directory isn't one, say so and stop. Read the diff before writing this; every line comes from what actually changed, never from a plan.

**It is output, not an action.** The user pastes it — `rules-source-control` owns why you never stage or commit it yourself. Invoked directly, or internally by `/workflow-complete` when it closes out a task or phase.

## Format

```
<commit message>

<secondary description>
```

- **The commit message is the concise explanation, not the detail dump.** It names *what* changed in one glance — a plain phrase (`added foundation for api`) and a typed one (`feat: api foundations`) are equally valid wording; either way it has to say something, never just a bare type tag. If you do use a type prefix, keep it to `feat`, `fix`, `chore`, `docs`, `test`, `refactor`, `perf`, `build`, `ci` — the same taxonomy `rules-source-control` uses for branch names.
- **Commit message caps at 72 characters.** Imperative mood, no trailing period. Anything that doesn't fit moves to the secondary description — never truncate mid-thought to squeeze it in.
- **The secondary description is where detail goes**, one flowing line, never bullets. Distinct changes are delimited by `,`. A change with multiple parts is delimited by `+` within its own comma-separated segment — `made a change here + here, wrote tests to validate all changes`.
- **Omit the secondary description entirely when the message already says it all** — a single-concern, single-file change gets nothing below it. Never pad with a segment that just restates the message.
- **One comma-segment per distinct concern, not per file.** Ten files in one mechanical rename is one segment (`+` for the pieces if it's worth naming them); a hand-written function is its own segment even if it shares a file with something else.
- **No trailer lines.** No co-author, no generated-by, no issue footer unless the user asked for one.

## Examples

Scaled to the diff — these are the three shapes that come up, not a menu to pick from:

**Single concern, no secondary description:**
```
fix: stop context_injector crashing on a missing BACKLOG.md
```

**Plain-phrase message, a couple of changes:**
```
added foundation for api

added route scaffolding + auth middleware, wrote tests to validate all changes
```

**Typed message, several areas, multi-part segments:**
```
refactor: bucket-prefixed skill rename

renamed skills to bucket prefixes across generate + assess + standards + rules + config + workflow, updated settings.json + generate-repo + README + AGENTS.md to match, added assess-performance and wired it into assess-code-quality
```

## Output

Return the message alone, in a fenced block, ready to paste — no surrounding commentary.
