---
name: assess-bugs
description: Score the current diff's latent-defect risk (Low → Critical) from complexity, coverage, and change shape, with a one-paragraph cited rationale.
argument-hint: []
disable-model-invocation: true
---

# Defect risk assessment

**Once per task, before `/workflow-complete` or a push — never per-edit.** Scores how likely the diff is to hide an uncaught bug, not whether one has been found. Read the diff before scoring; every claim comes from what actually changed.

**Not a finder.** `/code-review` reads the logic and reports actual bugs; this scores the odds one is still in there — a triage signal for whether that pass is worth running now versus later.

## The scale

| Tier | Risk test |
|---|---|
| Low | No behavior change, or trivial and fully covered by an existing test |
| Low-Med | New logic, but simple (single branch, no state) and tested |
| Med | Branching or stateful logic, partially tested |
| Med-High | Concurrency, retries, or multi-step state changes; thin or no test coverage |
| High | Touches a shared/critical path with untested edge cases |
| Critical | Untested logic on an irreversible or money/data-loss path |

Score the **highest tier any touched file reaches** — one Critical-tier file outweighs nine Low-tier ones.

## Output

```markdown
## Defect risk: <TIER>

<one paragraph, ≤3 sentences, citing file:line for the driving factor(s)>

**Next:** run /code-review — <one-line reason>
```

- **`file:line` for every driving factor.** No pointer, no score.
- **Omit `Next` below Med-High.** Nothing to recommend at Low through Med.
- **One paragraph only.** If the reasoning needs more than 3 sentences, the diff needs splitting, not a longer writeup.
