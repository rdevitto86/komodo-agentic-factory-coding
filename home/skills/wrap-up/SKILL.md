---
name: wrap-up
description: Report finished phased or scoped task work as a PR-style summary — commit message, changed files, test scenarios by tier, follow-ups.
argument-hint: []
disable-model-invocation: true
---

# Wrap-up

**Only at the end of phased work or a scoped task** — not after every file edit, not mid-plan. **Only inside a git repository** — if the working directory isn't one, say so and stop.

## Output

```markdown
## ✅ <task/phase/plan name>
**Commit message**: <type>(<scope>): <summary>

### Changed
- <file/area> — <what changed, ≤5 sentences>

### Tests
#### Unit
- <scenario covered>
#### Component
- <scenario covered>
#### Integration
- <scenario covered>

### Follow-ups
- <deferred item, if any> → added to TODO.md
```

- **Commit message** follows conventional-commit style even if nothing is actually committed.
- **Only include tier subheadings that have scenarios.** No empty `### Component` when nothing discretionary was added.
- **Tests lists scenarios, not tier names** — a reviewer should know what broke if one of these regresses.
- **Follow-ups is omitted entirely when there are none** — no "N/A" filler.
