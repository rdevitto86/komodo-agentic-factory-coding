# Way of working — SDLC

The gates for the phases that run in **this** session. The forked phases carry their own rules, so nothing here is restated inside them.

`standards-sdlc` owns test tiers and coverage floors. `standards-cicd` owns pipeline stages. Neither is repeated here.

---

## P0 · Spec

The SDD sections a code build actually depends on:

| Section | Blocks |
|---|---|
| §1 Architecture | Any task adding a component |
| §2 Data Model | Any task that persists |
| §5 Security | Any task on an external boundary |

**A `NEEDS DECISION` in a section a task depends on is a P0 block**, not a P2 assumption. Elsewhere it is fine; the doc may be incomplete about work nobody is doing.

---

## P2.2 · Verify

**The repo's own gate**, discovered in order: `.claude/verify.sh`, then a `verify` target in `Makefile`, `Taskfile`, or `justfile`. `verify_gate.py` runs it at turn's end and blocks while red.

**A repo with none of those has no gate** — one line of finding, because it means the user is the verification loop here.

**Coverage floors are `standards-sdlc`'s.** A task that drops coverage below its floor has failed even with green tests.

---

## P2.3 · Review

`/assess-bugs` against the task, always. Add `/assess-security` when the touched surface includes an auth/secret/boundary path. **Invoke each with the task text and which `standards-*` skills the touched files load** (the language skill at minimum; `standards-docker` for a touched `Dockerfile`/`docker-compose.yaml`, `standards-api-security` for a touched auth/secret/boundary path, `standards-ui-security` for a touched rendered surface) — an unbriefed review picks its own lenses, which is not a repeatable gate. The lenses that matter:

- **Correctness** — does it do what the story said, including the edge the story named
- **Security** — new boundary, new query, new secret handling
- **Complexity** — a function that grew a fourth responsibility
- **Idiom** — does it read like the code around it
- **Comment discipline** — the guard blocks additions at write time; a review still catches one smuggled through an allowed slot

**Neither call takes `--report` here** — each files its findings to `BACKLOG.md` like any standalone run. Findings on correctness, security, or a stated requirement are folded straight into P2.0's pick and become the next P2.1 task, in this same pass. Everything else is optional and stays filed for later.

**Round cap, same file, same band.** A new Critical/High finding on a file already reviewed in this band is round 2; a third straight new Critical/High finding on that same file is the stop signal — mirror the implement-side rule that the same check failing twice with the same error means stop, not retry. On round 3, do not fold the finding into another P2.1 fix-and-reloop pass: file it, state plainly that the round budget for this file in this band is spent, and surface the choice to the user — fix now, risk-accept and ship, or defer to a follow-up task. **The budget drops to 2, not 3,** when the touched file is already flagged in `BACKLOG.md` as a hand-rolled parser or security boundary with an open adversarial-hardening story (for example `git_guard.py`) — adversarial review of hand-rolled shell/parser code is close to open-ended by construction, so budget for that going in rather than discovering it at round 3.

**Every review call's brief states its round number for this band.** Round 1 gets the standard brief. From round 2 onward, the brief also instructs the reviewer to report only clear, concrete, reproduced findings against the touched file — not a theoretical edge case in the underlying grammar or format the fix touches. A round-2+ call that surfaces only theoretical findings is not a new consecutive round for the cap above.

---

## P2.4 · Closeout

**Once per band, not per-task** — after every task in the current pick is green, before `/workflow-consolidate` runs. Clears the target state's four standing closeout stories.

`/assess-bugs`, `/assess-security`, `/assess-simplify` against the whole band's diff, then confirm the perf suite ran. Each runs as a `reviewer` fork, same as P2.3. **Dispatch the three `assess-*` calls in parallel, `isolation: worktree`**, not the serial list this used to be — all three write to `BACKLOG.md`, so this is a writer fan-out, not a read-only one; merge each worktree's diff back one at a time before continuing. **No `--report` here either** — each files to `BACKLOG.md`. Every story a call just filed is folded into P2.0's pick and resolved in this same pass: fixed via `/workflow-implement`, or explicitly declined and removed with the reason noted. None of it waits for the next `/workflow-loop` run.

**Same round cap and round-numbered brief as P2.3, per touched file, for the whole band.** A closeout fix-and-reloop pass counts toward that file's round total; hitting the cap here stops the same way — file, state the budget is spent, surface fix-now/risk-accept/defer to the user instead of resolving it in this pass.

**Same round cap and round-numbered brief as P2.3, per touched file, for the whole band.** A closeout fix-and-reloop pass counts toward that file's round total; hitting the cap here stops the same way — file, state the budget is spent, surface fix-now/risk-accept/defer to the user instead of resolving it in this pass.

**Ends when:** the four closeout stories' findings are fixed or explicitly declined — that satisfies their `Done when: findings triaged`, so `/workflow-consolidate` deletes them like any other finished story.

---

## Adding a second way of working

A non-code lifecycle supplies its own `ways/<name>.md` **and its own forked phase skills**. The machine in `SKILL.md` does not change; the gates and the fork agents do.
