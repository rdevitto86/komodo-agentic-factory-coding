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

Levels 70 and 80 are prose, checked by hand once. Every criterion from 90 up has a proof: a command that exits zero on a tree that meets it. A proof that fails means the criterion is not met, whatever a review believes. When a fix merges, its proof passes and the score moves with it; nothing else moves it.

Run every proof from the repo root:

```bash
while IFS=$'\t' read -r id cmd; do bash -c "$cmd" >/dev/null 2>&1 && echo "$id pass" || echo "$id fail"; done < <(sed -n 's/^| `\([A-Z][0-9]*[a-z]\)` | [^|]* | `\(.*\)` | [^|]* |$/\1\t\2/p' docs/scorecard.md | sed 's/\\|/|/g')
```

### Safety
- **70:** one guard hook, its table in the gate, denies critical refs, trailers, and paths outside the worktree.
- **80:** a lexer-based guard under fuzz, at least 300 table rows with 0 wrong, safety modes, spawn isolation refused.

| Id | Criterion | Proof | Delivered by |
|---|---|---|---|
| `S90a` | Forge writes through `gh api` are refused | `grep -q 'branches/main/protection' internal/guard/table.go` | TSK-03.22.1 |
| `S90b` | Gate bypass is refused | `grep -q -- '--no-verify' internal/guard/table.go` | TSK-03.22.2 |
| `S90c` | Interpreter bodies and scripts written then run are read | `grep -q 'python3 -c' internal/guard/table.go` | TSK-03.22.3, TSK-03.22.4 |
| `S90d` | A builder's worktree cannot push | `grep -q 'refused://' internal/line/worktree.go` | TSK-03.23.1 |
| `S95a` | A local reviewer reviews only above its recorded recall | `grep -q 'func ReviewerRecall' internal/mount/registry.go` | TSK-03.24.2 |

`S95b`, no new bypass found for three consecutive history rows, is read from the history.

### Throughput
- **60:** a group runs end to end headless.
- **70:** at least 10 consecutive headless ships with no hand edit, and a ledger that times every station.

| Id | Criterion | Proof | Delivered by |
|---|---|---|---|
| `T80a` | Every ready spawn in a wave launches together | `grep -q 'json:"spawns' internal/line/step.go` | TSK-03.21.1 |
| `T80b` | Waves split by file | `grep -rqE 'func (claimsOverlap\|Overlap)\(' internal/line internal/plan` | TSK-03.26.1 |
| `T90a` | Groups with disjoint files run at once | `grep -rq '"runs"' internal/line` | TSK-03.26.5 |
| `T90b` | Small tasks build on the light tier | `grep -q 'light_builder' internal/mount/registry.go` | TSK-03.26.3 |
| `T90c` | `komodo metrics` reports tasks per hour | `go run ./cmd/komodo metrics \| grep -qi 'per hour'` | TSK-03.26.4 |

`T95a`, a sustained tasks-per-hour figure from another repo, is read from the changelog.

### Architecture
- **80:** host names only in `internal/mount/` with doctor enforcing it, the station order only in `step`, the standard library only, and an ADR per core decision.

| Id | Criterion | Proof | Delivered by |
|---|---|---|---|
| `A90a` | `step` decides through a pure, fuzzed `Next` | `grep -q 'func Next(' internal/line/snapshot.go && grep -q 'func FuzzNext' internal/line/snapshot_test.go` | TSK-03.21.3 |
| `A90b` | Live status stays out of `BACKLOG.md` until ship | `! grep -q 'writeStatus(' internal/line/close.go` | TSK-03.25.1 |
| `A90c` | One git adapter | `test -f internal/git/git.go` | TSK-03.27.1 |
| `A90d` | The guard links neither `profile` nor `mount/ollama` | `! go list -deps ./internal/guard \| grep -qE '^komodo/internal/(profile\|mount/ollama)$'` | TSK-03.27.2 |
| `A90e` | The planner has its own package | `test -f internal/plan/plan.go` | TSK-03.27.3 |
| `A95a` | `line` is under 2,500 non-test lines | `test "$(cat $(ls internal/line/*.go \| grep -v _test) \| wc -l)" -lt 2500` | TSK-03.27.4, TSK-03.27.5 |

### Code quality
- **80:** the gate is green, with vet and race tests, and every package has tests.

| Id | Criterion | Proof | Delivered by |
|---|---|---|---|
| `C90a` | Every package at or above 70 percent coverage | `go test -cover ./... 2>/dev/null \| awk '/coverage:/{for(i=1;i<=NF;i++) if($i ~ /%$/){sub(/%/,"",$i); if($i+0<70) bad=1}} END{exit bad}'` | TG-03.28 |
| `C90b` | At least 5 fuzz targets | `test "$(grep -rh '^func Fuzz' internal cmd \| wc -l)" -ge 5` | `#154` |
| `C90c` | No non-test file over 500 lines | `test -z "$(find internal cmd -name '*.go' ! -name '*_test.go' \| xargs wc -l \| awk '$2!="total" && $1>500')"` | `#176` |
| `C95a` | Every package at or above 80 percent coverage | `go test -cover ./... 2>/dev/null \| awk '/coverage:/{for(i=1;i<=NF;i++) if($i ~ /%$/){sub(/%/,"",$i); if($i+0<80) bad=1}} END{exit bad}'` | not queued |

### Proven on real work
- **70:** at least 10 consecutive headless ships with no hand edit, each with a run id in the ledger.
- **80:** a multi-task, multi-wave group has shipped headless.
- **90:** three groups have shipped headless on a repo other than this one, recorded in the changelog with run ids.
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
