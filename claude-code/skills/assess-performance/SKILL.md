---
name: assess-performance
description: Score the current diff's performance risk (Low → Critical) — latency, algorithmic complexity, build/runtime cost — with a cited rationale table, filed as a BACKLOG.md story at Med-High or above. Pass --report to skip the write.
argument-hint: [--report]
---

# Performance risk assessment

**Once per task, before `/workflow-complete` or a push — never per-edit.** Scores the performance risk the diff introduces, not whether it's already been profiled. Read the diff before scoring; every claim comes from what actually changed. Invoked directly by the user, or internally by `/assess-code-quality` as one input to its conventions pass (with `--report`, so a single diff never files two overlapping stories).

**Not a profiler.** `/assess-simplify`'s efficiency findings catch an actual bad pattern; this scores the odds one is hiding in the diff — a triage signal for whether a profiling pass is worth running now versus later.

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

**A driving factor needs a concrete cost change**, not a hypothetical. "Could theoretically" is not a finding; "the loop at `file:line` now issues one query per iteration" is.

## Output

```markdown
## Performance risk: <TIER>

| Where | Driving factor |
|---|---|
| `file:line` | <what raises the tier> |

**Next:** profile <hot path> — <one-line reason>
```

- **`file:line` for every driving factor.** No pointer, no score.
- **Omit `Next` below Med-High.** Nothing to recommend at Low through Med.
- **One row per driving factor.** If the table needs more than 3 rows, the diff needs splitting, not a longer table.
- **Diagram form — a Mermaid `graph LR` beneath the table, one node per hot-path hop.** Only a chain triggers it here (one factor sits on the path another feeds); the 3-row cap means the entity count never does. Unchained factors: table only.

## Findings → backlog

At Med-High or above, file one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] Performance risk: <TIER> — <driving factor> · S → \`/assess-performance\` scores Med or below`. Sev maps Med-High→`[M]`, High→`[H]`, Critical→`[C]`. Below Med-High, nothing is filed — there's no action to track. Append under the current target state's `Cross-Cutting` domain — full story-line rules live in `backlog-modify`. `--report` prints the score only; nothing is written.
