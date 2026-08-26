# Way of working — Debugging

The gates for the phases that run in **this** session, when the task is "find why it's broken," not "build something." Same five-phase machine as `sdlc.md` — reproduce stands in for spec, hypothesize for decompose, test-one-hypothesis for implement, and closeout writes a runbook instead of a changelog entry.

`standards-sdlc` and `standards-cicd` are not loaded here — a debugging session reads the failure, it doesn't design a feature.

---

## P0 · Reproduce

**The human gate is a reliable repro, not an approved spec.** Establish, in dialogue:

| Fact | Why it blocks |
|---|---|
| A repro command or steps | Nothing downstream has a `Done when` without one |
| The symptom, stated concretely | "500 on POST /orders" not "it's broken" |
| A "started after" boundary, if known | A version, a deploy, a commit — narrows P1's search |

**If the failure can't be reproduced on demand, this is the one phase allowed to block with nothing delivered** — hypothesizing against a failure nobody can trigger is the guessing the whole machine exists to prevent.

**Ends when:** the repro command exists and running it reliably shows the symptom.

---

## P1 · Hypothesize

**Runs as `/workflow-debug-hypothesize <repro + characterization from P0>` instead of `/workflow-decompose`** — the fork target is `planner`, same as decompose, but the contract is a ranked hypothesis queue, not a backlog-derived task queue. See that skill for its read order.

**Ends when:** every hypothesis carries a `Done when` command and a rank.

---

## P2.1 · Test one hypothesis

**Runs as `/workflow-implement`, unchanged** — its contract ("execute one task, run its `Done when` command") already fits: the task is the top-ranked untested hypothesis, the `Done when` is the diagnostic command P1 attached to it.

**A hypothesis that confirms becomes the fix.** Re-run the same fork with a follow-up task: patch the root cause, `Done when` is the original P0 repro command no longer showing the symptom.

**A hypothesis that denies is dropped, not retried.** Pick the next-ranked one and continue.

---

## P2.2 · Verify

**The gate is the P0 repro command, not the repo's own `verify.sh`/`Makefile` target.** Run it. If it still shows the symptom, the hypothesis was wrong even if its own `Done when` passed — return to P1's queue, not to a second attempt at the same fix.

**A code fix that touches shared logic still owes the repo's normal gate too** — run it once the repro clears, same as any other change.

**Ends when:** the P0 repro command no longer shows the symptom, and its output is in the transcript.

---

## P2.4-equivalent · Closeout

**Runs as `/workflow-implement` writing a runbook entry, instead of `/workflow-consolidate`.** The task: draft `docs/runbooks/<slug>.md` from `generate-runbook`'s template, `<slug>` naming the failure mode. `Done when`: the file exists and its Trigger/Diagnosis/Resolution sections cite the confirmed root cause and the fix from P2.1/P2.2 — no changelog entry, no version bump, this phase never touches either.

**A code fix that shipped still needs its own changelog entry** — that's `/workflow-consolidate`, run once through the normal `sdlc.md` gates for the fix itself; this closeout only produces the operational record of the incident.

**Ends when:** the runbook file exists and cites the SDD §7 signal or §10 recovery scenario it backs, if one exists.
