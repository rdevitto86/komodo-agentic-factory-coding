---
name: run
description: Drive the line: ask for the next action, do exactly that, repeat until done.
---

# Run

You drive one group through the line. The station order lives in the binary. You never guess it and never reorder it.

A drain, a plain `komodo run <group>`, `--no-ship`, and `--dry-run` all drive the conductor directly;
nothing headless ever starts this skill's loop. It runs only when a person types `/run <group>` themselves.

## The loop

1. Run `komodo step <group-or-task>`, or bare `komodo step` to continue the open run.
2. Do exactly what the JSON says, and nothing else.
3. Go back to 1. Stop when `action` is `done`.

## What the JSON means

- **`run`** — run `command` verbatim from the repo root, then loop.
- **`spawn`** — spawn the `role` agent on `brief`, in `worktree`. `machine` is `provider/model`; pass the model half as the spawn's model, so a task's `tier` is honoured. Its context is `skills`, `facets`, and `commands`; give it no more. When a `spawns` list is present, spawn only its entries, all in the same turn, and ignore the top-level `role` and `brief`. Wait for all of them, then loop.
- **`done`** — stop and report `why`.

## The binary

`komodo` on PATH wins whenever it is present, headless or in a session. In a session when nothing is on PATH, `komodo` is the absolute path the guard hook in the host's project settings names. In the toolkit repo when nothing is on PATH, `go run ./cmd/komodo` is the fallback.

## Rules

- **One step per turn.** Never run ahead of `step`. Never batch two stations; a `spawns` list is one step.
- **This driver never asks.** Every action `step` returns, `komodo close --group` included, is already approved by the human who typed `/run`.
- **A non-zero exit stops the loop.** Report the command and its output. Do not substitute another command.
- **A spawned agent works in `worktree` and nowhere else.** Every path it is given resolves from there, including its brief and its result.
- **Never pass an isolation option to a spawn.** The line already cut `worktree`; a second one strands the agent's diff.
- **A spawned agent returns the JSON its schema names.** Save it where the brief says, then loop. Never finish its work yourself.
- **After a pull request merges, or when `komodo doctor` reports drift, run `komodo sync` yourself.** Never hand the human `gate --install`, `install --host`, or a pull.
- **Report in the accessibility contract** when the loop ends.

## The line and ad hoc work

The line is captive: `/run`, which runs `komodo run`, is its one entry. No session relays this loop;
a drain, `komodo run <group>`, `--no-ship`, and `--dry-run` all drive the conductor directly, never a
model. Ad hoc work — one stage on a group, or with no group at all — is never this skill; it is the
orchestrator's own default agents, spawned outside the line and its line tier, with no skill of their
own (decision 0003). `komodo brief <task>` on its own is legal, and so is a review with no group, but
neither runs through `komodo step`.

An isolated spawn's worktree starts at the default branch, not yours: put `git switch --detach <sha>`
first in its prompt, naming your own branch tip.
