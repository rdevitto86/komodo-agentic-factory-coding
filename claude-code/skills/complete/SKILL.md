---
name: complete
description: Close out finished work as a copy-pastable commit message plus what changed.
argument-hint: []
disable-model-invocation: true
---

# Complete

**At the end of a scoped task or a phase** — not after a single edit, not mid-plan. **Inside a git repository only** — if the working directory isn't one, say so and stop.

Read the diff before writing this. Every line comes from what actually changed, never from the plan you intended to execute.

## The commit message

**It is output, not an action.** The user pastes it. Never stage, commit, or offer to.

```
<type>(<scope>): <subject>

- <what changed, one line>
- <what changed, one line>
```

- **Types:** `feat`, `fix`, `chore`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`.
- **Subject caps at 72 characters** including the type and scope. Imperative mood, no trailing period.
- **The body is bullets, never prose.** One line each, same concision as the subject.
- **No trailer lines.** No co-author, no generated-by, no issue footer unless the user asked for one.

## Output

````markdown
## ✅ <task or phase name>

```
<the commit message, ready to paste>
```

### Changed
- **<file or area>** — <what changed, one sentence>

### Verified
- <command run> — <result>

### Follow-ups
- <deferred item> → BACKLOG.md
````

- **Changed lists areas, not every file.** Ten files in one package is one line.
- **Verified is what you actually ran** — the command and its real result. A failure goes here stated plainly, not softened and not omitted.
- **Omit any section that is empty.** No "N/A", no empty heading.
- **No test-tier breakdown.** `sdlc` owns tier definitions; repeating them here turns a summary into a report.
