---
name: risk-assessment
description: Score the current diff's blast radius (Low → Critical) with a one-paragraph cited rationale.
argument-hint: []
disable-model-invocation: true
---

# Risk assessment

**Once per task, before `/complete` or a push — never per-edit.** Scores what a change could break, not whether it already has a bug. Read the diff before scoring; every claim comes from what actually changed.

**Not a defect finder.** `/code-review` and `/security-review` find bugs; this scores the damage if one slipped through. High blast radius with zero known defects still scores High.

## The scale

| Tier | Blast radius test |
|---|---|
| Low | No behavior change, or fully isolated — docs, comments, test-only, formatting |
| Low-Med | One file or function, reversible, no fan-out |
| Med | Contained to one package/service, reversible, covered by tests |
| Med-High | Multi-file, touches a critical path, partial test coverage |
| High | Crosses a trust boundary or touches shared/prod-adjacent surface, thin coverage |
| Critical | Irreversible and on a prod, security, or data-loss path — no rollback |

Score the **highest tier any touched file reaches** — one Critical-tier file outweighs nine Low-tier ones.

## Output

```markdown
## Risk: <TIER>

<one paragraph, ≤3 sentences, citing file:line for the driving factor(s)>

**Next:** run /code-review — <one-line reason>
```

- **`file:line` for every driving factor.** No pointer, no score.
- **Omit `Next` below Med-High.** Nothing to recommend at Low through Med.
- **One paragraph only.** If the reasoning needs more than 3 sentences, the diff needs splitting, not a longer writeup.
