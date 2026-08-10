---
name: wrap-up
description: Report finished work as a commit message plus what changed.
argument-hint: []
disable-model-invocation: true
---

# Wrap-up

**At the end of a scoped task or a phase** — not after a single edit, not mid-plan. **Inside a git repository only** — if the working directory isn't one, say so and stop.

Read the diff before writing this. Every line comes from what actually changed, never from the plan you intended to execute.

## Output

```markdown
## ✅ <task or phase name>

**<type>(<scope>): <summary>**

### Changed
- **<file or area>** — <what changed, one sentence>

### Verified
- <command run> — <result>

### Follow-ups
- <deferred item> → TODO.md
```

- **Conventional-commit style**, even though nothing is being committed. Subject line only; no body, no footer.
- **Changed lists areas, not every file.** Ten files in one package is one line.
- **Verified is what you actually ran** — the test command and its real result. A failure goes here stated plainly, not softened and not omitted.
- **Omit any section that is empty.** No "N/A", no empty heading.
- **No test-tier breakdown.** `sdlc` owns tier definitions; repeating them here turns a summary into a report.
