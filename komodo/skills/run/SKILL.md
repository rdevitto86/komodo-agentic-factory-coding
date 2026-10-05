---
name: run
description: Run one group through the line: start komodo run in the background, watch it, report how it ended.
---

# Run

You start one group's run and watch it. The conductor in the `komodo` binary drives every stage, exactly as a headless `komodo run` does. You never drive a stage, brief a line role, or spawn one yourself.

## Steps

1. Start `komodo run <group>` in the background from the repo root. Add `--no-ship` only when the person asked to stop before the push. Never run it in the foreground; a run lasts up to its budget.
2. While it runs, take one `komodo status` snapshot when the person asks how it is going. Never pass `--watch`; it redraws until interrupted and never returns.
3. When the background command exits, read its exit code and the end of its output.

## Reporting

- **Exit 0:** the group shipped. Report its pull request link from the output and how long the run took.
- **Non-zero:** report the stage that stopped and its reason, quoted from the output, and the command that continues it: `komodo resume <group>`.
- **Blocked:** a blocked group's note is in its `docs/backlog` file and on its draft pull request; name both.
- **Report in the accessibility contract** when the run ends.

## Rules

- **One driver.** Only the binary moves a group between stages. This skill never calls `brief`, `close` or `step`.
- **A stopped run is resumed, never restarted.** `komodo resume <group>` keeps its work; a second `komodo run` would cut it again.
- **The binary.** `komodo` on PATH wins. In the toolkit repo with nothing on PATH, use `go run ./cmd/komodo`.
- **After a pull request merges, or when `komodo doctor` reports drift, run `komodo sync` yourself.** Never hand the person `gate --install`, `install --host`, or a pull.

## The line and ad hoc work

The line is captive: `komodo run`, started by this skill or headless, is its one entry. Ad hoc work, meaning one stage on a group or no group at all, is never this skill. It is the orchestrator's own default agents, spawned outside the line, with no skill of their own (decision 0003).
