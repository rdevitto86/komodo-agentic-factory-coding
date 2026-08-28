---
name: assess-change-risk
description: Score the current diff's blast radius (Low → Critical) with a cited rationale table, filed as a BACKLOG.md story at Med-High or above. Pass --report to skip the write.
argument-hint: [--report]
disable-model-invocation: true
---

# Risk assessment

**Once per task, before `/workflow-complete` or a push — never per-edit.** Scores what a change could break, not whether it already has a bug. Read the diff before scoring; every claim comes from what actually changed.

**Not a defect finder.** `/assess-bugs` and `/assess-security` find bugs; this scores the damage if one slipped through. High blast radius with zero known defects still scores High.

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

| Where | Driving factor |
|---|---|
| `file:line` | <what raises the tier> |

**Next:** run /assess-bugs — <one-line reason>
```

- **`file:line` for every driving factor.** No pointer, no score.
- **Omit `Next` below Med-High.** Nothing to recommend at Low through Med.
- **One row per driving factor.** If the table needs more than 3 rows, the diff needs splitting, not a longer table.

## Findings → backlog

At Med-High or above, file one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] Risk: <TIER> — <driving factor> · S → \`/assess-change-risk\` scores Med or below`. Sev maps Med-High→`[M]`, High→`[H]`, Critical→`[C]`. Below Med-High, nothing is filed — there's no action to track. Append under the current target state's `Cross-Cutting` domain — full story-line rules live in `backlog`. `--report` prints the score only; nothing is written.
