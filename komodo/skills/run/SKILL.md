---
name: run
description: Drive the assembly line. Ask the conveyor for the next action, do exactly that, repeat until it says done.
---

# Run

You drive one group through the line. The station order lives in the binary. You never guess it and never reorder it.

## The loop

1. Run `komodo step <group-or-task>`, or bare `komodo step` to continue the open run.
2. Do exactly what the JSON says, and nothing else.
3. Go back to 1. Stop when `action` is `done`.

## What the JSON means

- **`run`** — run `command` verbatim from the repo root, then loop.
- **`spawn`** — spawn the `role` agent on `brief`, in `worktree`. `machine` is `provider/model`; pass the model half as the spawn's model, so a task's `tier` is honoured. Its context is `skills`, `facets`, and `commands`; give it no more. When a `spawns` list is present, spawn only its entries, all in the same turn, and ignore the top-level `role` and `brief`. Wait for all of them, then loop.
- **`done`** — stop and report `why`.

## The binary

`komodo` is `bin/komodo-<os>-<arch>` at the repo root, built by `komodo gate --install`. In the toolkit repo `go run ./cmd/komodo` is the same binary.

## Rules

- **One step per turn.** Never run ahead of `step`. Never batch two stations; a `spawns` list is one step.
- **A headless driver never asks.** Every action `step` returns, `komodo close --group` included, is already approved by the human who launched the run.
- **A non-zero exit stops the loop.** Report the command and its output. Do not substitute another command.
- **A spawned agent works in `worktree` and nowhere else.** Every path it is given resolves from there, including its brief and its result.
- **Never pass an isolation option to a spawn.** The line already cut `worktree`; a second one strands the agent's diff.
- **A spawned agent returns the JSON its schema names.** Save it where the brief says, then loop. Never finish its work yourself.
- **Ad hoc work enters at any station.** `komodo brief <task>` on its own is legal, and so is a review with no group.
- **Report in the accessibility contract** when the loop ends.
