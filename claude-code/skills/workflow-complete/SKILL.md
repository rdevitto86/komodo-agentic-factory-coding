---
name: workflow-complete
description: Close out finished work as a copy-pastable commit message plus what changed.
argument-hint: []
---

# Workflow Complete

**At the end of a scoped task or a phase** — not after a single edit, not mid-plan. **Inside a git repository only** — if the working directory isn't one, say so and stop.

Read the diff before writing this. Every line comes from what actually changed, never from the plan you intended to execute.

## The commit message

Invoke `/generate-commit-message` for the message block — this skill owns the format, not this one. Splice its result into the fenced block below.

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
- **No test-tier breakdown.** `standards-sdlc` owns tier definitions; repeating them here turns a summary into a report.
