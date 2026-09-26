---
name: run
description: Launch the assembly line. Start komodo run in the background, report progress, and manage group execution.
---

# Run

You launch the assembly line and manage its execution. The station order lives in the binary. You never guess it and never reorder it.

## The commands

- **`komodo run [groups]`** — Start the conductor in the background and report progress.
- **`komodo status`** — Show groups by state, time used, and blocker notes.
- **`komodo stop [groups]`** — Pause one or more groups.
- **`komodo resume [groups]`** — Resume one or more groups.

## The binary

`komodo` on PATH wins whenever it is present, headless or in a session. In a session when nothing is on PATH, `komodo` is the absolute path the guard hook in the host's project settings names. In the toolkit repo when nothing is on PATH, `go run ./cmd/komodo` is the fallback.

## Rules

- **Start the run once.** Launch `komodo run [groups]` once per request. Never repeat it within a single invocation.
- **A headless driver never asks.** Every action you take is already approved by the human who launched you.
- **A non-zero exit stops execution.** Report the command and its output. Do not substitute another command.
- **Report in the accessibility contract** when your work ends.
