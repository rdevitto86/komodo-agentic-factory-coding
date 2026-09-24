# Scorecard

How this repo is rated, so two reviews of one commit give one score. A rating that skips this file is not comparable to the history below.

## How to score

1. **Score `origin/main`, never a local branch.** Run `git fetch origin`, then score that commit in a worktree outside `/tmp`. The guard allows writes to the temp dir, so `guard check` run from there reports false failures.
2. **Start from the last row of the history.** Every change from it cites a PR or a command output since that commit.
3. **An area's score is the highest level it fully meets,** plus 4 times the share of the next level's criteria it meets, rounded down. A criterion without evidence is not met.
4. **The total is the plain average of the six areas,** rounded to the nearest whole number. No other weighting.
5. **Record the result** as a new history row with the commit, the six area scores, and the total.

## Evidence commands

```bash
git fetch origin && git worktree add --detach ~/komodo/ai/.score origin/main
go run ./cmd/komodo gate                 # vet, race tests, lint, doctor, guard table, comments
go test -cover ./...                     # per-package coverage
go run ./cmd/komodo doctor --remote      # the forge ruleset on main
go run ./cmd/komodo metrics              # ledger: seconds per station, tokens, repairs
gh pr list --state merged --limit 20     # what landed since the last row
```

The probe set for Safety is every row of `komodo guard check`. A bypass found in a review becomes a denied row there before it counts as fixed.

## Levels

### Safety
- **70:** one guard hook, its table in the gate, denies critical refs, trailers, and paths outside the worktree.
- **80:** a lexer-based guard under fuzz, at least 300 table rows with 0 wrong, safety modes, spawn isolation refused.
- **90:** forge writes through `gh api` refused, gate bypass refused, interpreter bodies and scripts written then run are read, and a builder's worktree cannot push.
- **95:** a local reviewer reviews only above its recorded recall, and no review has found a new bypass for three consecutive rows.

### Throughput
- **60:** a group runs end to end headless.
- **70:** at least 10 consecutive headless ships with no hand edit, and a ledger that times every station.
- **80:** every ready spawn in a wave launches together, and waves split by file.
- **90:** groups with disjoint files run at once, small tasks build on the light tier, and `komodo metrics` reports tasks per hour.
- **95:** a sustained tasks-per-hour figure recorded from a repo other than this one.

### Architecture
- **80:** host names only in `internal/mount/` with doctor enforcing it, the station order only in `step`, the standard library only, and an ADR per core decision.
- **90:** `step` decides through a pure, fuzzed `Next`, live status stays out of `BACKLOG.md` until ship, one git adapter, the guard links neither `profile` nor `mount/ollama`, and the planner has its own package.
- **95:** the devices and stations have their own packages, and `line` is under 4,000 lines.

### Code quality
- **80:** the gate is green, with vet and race tests, and every package has tests.
- **90:** every package is at or above 70 percent coverage, there are at least 5 fuzz targets, and no non-test file is over 500 lines.
- **95:** every package is at or above 80 percent coverage.

### Proven on real work
- **70:** at least 10 consecutive headless ships with no hand edit, each with a run id in the ledger.
- **80:** a multi-task, multi-wave group has shipped headless.
- **90:** three groups have shipped headless on a repo other than this one.
- **95:** the exit test has run on a second host with no change outside the mounts.

### Docs and honesty
- **80:** the README describes what exists, and doctor fails on drift.
- **90:** every proof in the changelog names its run id, the failed runs are recorded next to the passing ones, and every core decision has an ADR.
- **95:** the history below has no row whose total moved without a cited change.

## History

| Date | Commit | Safety | Throughput | Architecture | Code | Proven | Docs | Total | Note |
|---|---|---|---|---|---|---|---|---|---|
| 2026-09-24 | before `#154` | — | — | — | — | — | — | 76 | Recorded before this rubric |
| 2026-09-24 | `1eff696` | 80 | 70 | 80 | 82 | 80 | 90 | 80 | First rubric score. Code meets 2 of 3 at 90; `gate` is at 67.4% and `pr` at 68.4% coverage |
