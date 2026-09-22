---
name: review
description: Review the current diff. Run the mechanical gate first, then the reviewer role, and report findings by severity.
---

# Review

You review what the working tree changed. The gate is mechanical and runs first; the reviewer is the only model call.

## The order

1. **`komodo gate`** — vet, tests, lint, doctor, the guard table, the binaries. A failure here ends the review; report it and stop.
2. **`komodo diff`** — the reviewer's whole input: the tasks in scope, the standards their files trigger, and the diff.
3. **Spawn the reviewer role** on that output. It reads cold, writes nothing, and returns the JSON its schema names.
4. **Report** the findings, highest severity first.

## Rules

- **The reviewer never edits.** A finding it wants fixed becomes a task or a thread, never a patch from the reviewer itself.
- **A finding needs a failure.** Concrete inputs and the wrong result, or it is not reported.
- **Standards come from the diff.** The files a change touches decide which `standards-*` skills load; you do not pick them by hand.
- **Nothing on the diff is assumed correct** because the gate passed. The gate is mechanical; the reviewer is the judgement.
