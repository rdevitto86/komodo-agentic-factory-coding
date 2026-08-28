---
name: git-commit-message
description: Write a copy-pastable commit message for the current diff — capped concise message, bulleted secondary description.
argument-hint: []
---

# Commit message

**Inside a git repository only** — if the working directory isn't one, say so and stop. Read the diff before writing this; every line comes from what actually changed, never from a plan.

**It is text, not the commit itself.** `workflow-loop`'s P2/P3 run `git commit` with this output directly — `git-pr-create` owns the branch/protected-ref rules governing when that's allowed. Invoked directly, or internally by the loop when a task or band closes out.

## Format

```
<commit message>

<secondary description>
```

- **The commit message is the concise explanation, not the detail dump.** It names *what* changed in one glance — a plain phrase (`added foundation for api`) and a typed one (`feat: api foundations`) are equally valid wording; either way it has to say something, never just a bare type tag. If you do use a type prefix, keep it to `feat`, `fix`, `chore`, `docs`, `test`, `refactor`, `perf`, `build`, `ci` — the same taxonomy `git-pr-create` uses for branch names.
- **Commit message caps at 72 characters.** Imperative mood, no trailing period. Anything that doesn't fit moves to the secondary description — never truncate mid-thought to squeeze it in.
- **The secondary description is where detail goes**, formatted as a `-`-prefixed bulleted list, one bullet per distinct concern. A bullet with multiple parts is delimited by `+` within itself — `added route scaffolding + auth middleware`.
- **Omit the secondary description entirely when the message already says it all** — a single-concern, single-file change gets nothing below it. Never pad with a bullet that just restates the message.
- **One bullet per distinct concern, not per file.** Ten files in one mechanical rename is one bullet (`+` for the pieces if it's worth naming them); a hand-written function is its own bullet even if it shares a file with something else.
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

- added route scaffolding + auth middleware
- wrote tests to validate all changes
```

**Typed message, several areas, multi-part bullets:**
```
refactor: bucket-prefixed skill rename

- renamed skills to bucket prefixes across generate + assess + standards + rules + config + workflow
- updated settings.json + repo-init + README + AGENTS.md to match
- added assess-performance and wired it into assess-code-quality
```

## Output

Return the message alone, in a fenced block, ready to paste — no surrounding commentary.
