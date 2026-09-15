---
name: workflow-debug
description: Read a failure's reproduction and characterization in a fork, and return a ranked queue of testable hypotheses.
argument-hint: a pm brief — Task (repro steps + symptom) | Context | Out of scope (all three required)
context: fork
agent: pm
background: false
---

# Hypothesize

Brief: **$ARGUMENTS** — a `pm` brief whose `Task` slot carries the repro steps, the symptom, and any "started after" boundary P0 already established, alongside the required `Context` and `Out of scope`.

**You cannot see the calling conversation.** Everything you need is in that brief and on disk, and a missing or empty required slot is your standing stop.

## Order

1. **Read `AGENTS.md` and `CLAUDE.md`** — commands, layout, gotchas.
2. **Trace the failure signature back through the code path it names** — the function, endpoint, or component the symptom points at, and its immediate callers/callees.
3. **Read `CHANGELOG.md`** for what shipped around the "started after" boundary, if one exists. A regression's cause often sits in the diff between the last-good and first-bad entry.
4. **Every hypothesis carries a command with an exit code.** State what to check and the `Done when` that confirms or denies it — a diagnostic command, a log grep, a flag flip and re-run. A hypothesis with no such command cannot become a task.
5. **Rank hypotheses most-likely first** — best supported by what steps 2–3 turned up, not alphabetical, not by ease.

**Cap the queue at 6 hypotheses.** More means the characterization was too thin — say what P0 evidence is missing instead of padding the list.

Your standing rules on reading order, inventing nothing, and output shape already apply.
