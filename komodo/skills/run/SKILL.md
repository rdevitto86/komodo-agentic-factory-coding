---
name: run
description: Launch the assembly line in the background, report progress, and control execution.
---

# Run

You launch and watch the conductor drive groups through the line. The station order lives in the binary. You never guess it and never reorder it.

## Launch and watch

1. Run `komodo run [groups]` to start the conductor in the background.
2. Run `komodo status` to report groups by state, time used, and blocker notes.
3. Run `komodo stop [groups]` or `komodo resume [groups]` to pause or continue execution.
4. Report the output. Stop when the human asks or the conductor reports all groups done.

## The binary

`komodo` on PATH wins whenever it is present, headless or in a session. In a session when nothing is on PATH, `komodo` is the absolute path the guard hook in the host's project settings names. In the toolkit repo when nothing is on PATH, `go run ./cmd/komodo` is the fallback.

## Rules

- **A headless driver never asks.** The human who launched the run already approved it; report what you see.
- **A non-zero exit stops the loop.** Report the command and its output. Do not substitute another command.
- **After a pull request merges, or when `komodo doctor` reports drift, run `komodo sync` yourself.** Never hand the human `gate --install`, `install --host`, or a pull.
- **Report in the accessibility contract** when the loop ends.
