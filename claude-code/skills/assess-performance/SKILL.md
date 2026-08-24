---
name: assess-performance
description: Score the current diff's performance risk (Low → Critical) — latency, algorithmic complexity, build/runtime cost — with a one-paragraph cited rationale.
argument-hint: []
---

# Performance risk assessment

**Once per task, before `/workflow-complete` or a push — never per-edit.** Scores the performance risk the diff introduces, not whether it's already been profiled. Read the diff before scoring; every claim comes from what actually changed. Invoked directly by the user, or internally by `/assess-code-quality` as one input to its conventions pass.

**Not a profiler.** `/code-review`'s efficiency findings catch an actual bad pattern; this scores the odds one is hiding in the diff — a triage signal for whether a profiling pass is worth running now versus later.

## The scale

| Tier | Performance test |
|---|---|
| Low | No touch to a hot path, loop, query, or build step |
| Low-Med | Touches a hot path but with no complexity change — same algorithm, same I/O count |
| Med | Adds an O(n) cost inside an existing loop, an extra query per iteration, or a new synchronous call on a hot path |
| Med-High | Nested iteration over unbounded input, a blocking call where the surrounding code is otherwise async, or a build/dependency change that measurably slows CI — spanning multiple files |
| High | A pattern the next change will copy — an N+1 query, an unindexed lookup, a per-request allocation that scales with load |
| Critical | Actively contradicts a documented performance convention (`AGENTS.md`, a `standards-<lang>` skill's stated pattern) — or introduces unbounded growth (memory, connections, retries) with no backpressure |

Score the **highest tier any touched file reaches** — one Critical-tier file outweighs nine Low-tier ones.

## Output

```markdown
## Performance risk: <TIER>

<one paragraph, ≤3 sentences, citing file:line for the driving factor(s)>

**Next:** profile <hot path> — <one-line reason>
```

- **`file:line` for every driving factor.** No pointer, no score.
- **Omit `Next` below Med-High.** Nothing to recommend at Low through Med.
- **One paragraph only.** If the reasoning needs more than 3 sentences, the diff needs splitting, not a longer writeup.
