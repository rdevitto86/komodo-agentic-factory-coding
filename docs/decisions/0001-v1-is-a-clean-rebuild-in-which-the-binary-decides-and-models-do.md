# 0001. V1 is a clean rebuild in which the binary decides and models do

**Status:** Accepted, 2026-09-25.

**Context.** The first 1.0.0 line worked end to end on one Mac, but readiness stalled between 72 and 88 for two weeks. The cause was the design, not the models. Every row below was measured in this repo on 2026-09-25; later entries cite a row as "evidence n".

| # | Problem | Evidence | Consequence |
|---|---|---|---|
| 1 | A model drove the loop | `komodo run` launched `claude -p "/run <group>" --permission-mode bypassPermissions --model sonnet`, and the run skill relayed `komodo step` JSON | TG-03.11 and TG-03.15 failed when the driver added an isolation option; in TG-03.22 the driver saved the reviewer's result itself; the driver's tokens were never metered |
| 2 | The guard was a bash denylist scored as a wall | 4,597 non-test lines in `internal/guard` re-implemented bash: quoting, brace lists, globs, variables | Denying commands by parsing bash can never be complete; every guard diff invited new "bypass" findings |
| 3 | Review never converged | TG-03.22 ran 11 review rounds: 17, 8, 10, 6, 6, 6, 6, 5, 5, 4, 2 findings; TG-03.31 rose from 1 to 7 across fixes | Each round re-reviewed the whole diff from scratch, with a floor that included weaknesses needing a specific precondition |
| 4 | "Done" was a model's score | A scorecard was edited 3 times by the session that then graded itself 88; blind reviews landed near 72 | Agents polished what the rubric counted, and none of its proofs ran the line on a real repo |
| 5 | Builders were spawned inside the driver | 3,289 of 3,894 builder shell calls started with `cd` into the worktree | The guard had to follow `cd` chains; builders drew 187 refusals, 133 of them "outside the worktree" |
| 6 | Builders explored by shell | 987 `grep`, 356 `sed`, 125 `cat`, 116 `ls` and 63 `find` calls | Turns and tokens went to finding context the binary could have packed |
| 7 | The light tier built | A rubric proof required one-file tasks to build on the light tier; the 3 Haiku builds averaged 75 turns | A cheaper model needing many more turns cost more, not less |
| 8 | The token meter was wrong | TSK-03.7.1 logged 45,520,132 input tokens in 63 turns | No budget can be enforced on a meter nobody trusts |
| 9 | No per-session budget | The only cap was 90 minutes per group; one build took 122 turns | A stuck session burned until the group budget ran out |
| 10 | Sessions were not hermetic | Every session loaded personal instructions, settings, plugins and MCP servers; models were floating aliases; the host CLI auto-updated | Three machines ran three different agents |
| 11 | The line rebuilt itself mid-run | TSK-03.31.7 and TSK-03.32.4 were stale-binary bugs | The thing under test changed while the test ran |
| 12 | `BACKLOG.md` was the database | 159 KB, 166 tasks, 153 done; 77 of 149 commits since Sep 11 touched it; ship rewrote it and lost a task's body | Merge conflicts, lost work, and a large token cost on every read |
| 13 | Groups stacked on side branches | TG-03.31 and TG-03.32 cut from a branch 39 commits ahead of `main` | Bases drifted, and hand commits rode along unreviewed |
| 14 | Windows was untested and partly unsafe | No CI, no `.gitattributes`, `sh -c` everywhere; on Windows a timeout killed only the parent; the hook matcher missed PowerShell | On native Windows the guard was bypassed by tool choice, and timeouts left orphans |
| 15 | Silent passes | The gate skipped build checks when detection found none; a local 3B reviewer counted as a proof and missed a real bug | "Green" could mean nothing ran |

The Python prototype died the same way: "every open backlog item was about keeping the orchestrator safe from itself" (`CHANGELOG.md`, 1.0.0-alpha.1).

**Decision.** Rebuild V1 against the PRD, following the principles in `hld.md#purpose`. Port only the parts that already work, with their tests:

| Part | Where it lives today | Why it holds |
|---|---|---|
| A pure decision function over disk state | `Next` in `internal/line/snapshot.go`, fuzzed | Deterministic, restartable, testable without a model |
| A worktree per unit of work, waves by file overlap | `internal/line/worktree.go`, `internal/plan` | Parallel builds with no shared state |
| Schema-checked results | `komodo/roles/*.schema.json`, `internal/line/schema.go` | Output is data, rejected when wrong |
| Credential scrub, then the binary pushes | `Scrub` and `finishShip` in `internal/run/run.go` | The model never pushes |
| Brief slots with caps and clip markers | `internal/line/brief_slots.go`, `clip.go` | Bounded input, the same bytes on any machine |
| The ledger | `internal/ledger` | Metrics cost zero tokens |
| The plan probe | `internal/mount/claude/limits.go` | Reads the plan and usage window without a model |
| The forge audit | `komodo doctor --remote` | Checks the one boundary a shell cannot reach |

**Alternatives.**

- **Patch the first line in place.** Its structure produced the loops.
- **Rewrite everything, the working parts included.** It would reopen problems already solved and tested.

**Consequences.**

- **Each ported part keeps its tests,** so a regression shows at once.
- **The evidence table is the standing record of why.**
