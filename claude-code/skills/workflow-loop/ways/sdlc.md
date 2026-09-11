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

**Once the gate is green, commit here** — `/git-commit-message` against the task's diff alone, then `git add` + `git commit`. One commit per task; no review runs in this phase.

---

## P2.3 · Band review

Runs once per band, after every task in the band is committed — never per task. Dispatch `/assess-bugs`, `/assess-simplify`, and (only when a touched path is an auth/secret/boundary path or a rendered surface) `/assess-security` together in one parallel block, as plain forks with no isolation — they are read-only after TSK-01.4.2, so nothing here collides. **Invoke each with the band summary and which `standards-*` skills the touched files load** (the language skill at minimum; `standards-docker` for a touched `Dockerfile`/`docker-compose.yaml`, `standards-api-security` for a touched auth/secret/boundary path, `standards-ui-security` for a touched rendered surface) plus the round number — an unbriefed review picks its own lenses, which is not a repeatable gate. The lenses that matter:

- **Correctness** — does it do what the story said, including the edge the story named
- **Security** — new boundary, new query, new secret handling
- **Complexity** — a function that grew a fourth responsibility
- **Idiom** — does it read like the code around it
- **Performance** — algorithmic complexity, a new hot-path cost, unbounded growth
- **Comment discipline** — comments are in the diff now (the implementer writes them, see TSK-01.4.3); a narrative, name-echo, or change-narrating comment is a Low finding the orchestrator deletes itself with one Edit, never a fork

**Performance dispatch.** In the same pass, also run `/assess-performance` (no arguments needed — it reads the diff itself) whenever a touched path is performance-sensitive — a hot loop, a changed algorithm's complexity class, a new query or index. Unlike the three finder forks above, `/assess-performance` carries no `context: fork`/`agent: reviewer` frontmatter (it's the same self-filing scorer contract `/assess-code-quality` already uses), so it runs inline in this session rather than as a `reviewer` fork; it files its own `BACKLOG.md` story at Med-High or above directly, under `Cross-Cutting`, rather than returning a row for the orchestrator to append. Once filed, that story is an ordinary backlog row — the severity floor and round cap below apply to it exactly like a finder fork's finding.

**The orchestrator appends every finding row the three finder forks return to `BACKLOG.md` under the matching domain in one Edit, before triage** — story-line shape per `backlog-modify`. (`/assess-performance` already filed its own, as above.)

**Severity floor.** Critical and High findings, plus a Medium finding on correctness or a stated requirement, are fixed in this band via `/workflow-implement`; the fixed task re-enters P2.2. Every other finding stays filed and open — it waits for its own pick later, not folded into this pass.

**Round cap, same file, same band.** A new Critical/High finding on a file already reviewed in this band is round 2; a third straight new Critical/High finding on that same file is the stop signal — mirror the implement-side rule that the same check failing twice with the same error means stop, not retry. On round 3, do not fold the finding into another fix-and-reloop pass: file it, state plainly that the round budget for this file in this band is spent, and surface the choice to the user — fix now, risk-accept and ship, or defer to a follow-up task. **The budget drops to 2, not 3,** when the touched file is already flagged in `BACKLOG.md` as a hand-rolled parser or security boundary with an open adversarial-hardening story (for example `git_guard.py`) — adversarial review of hand-rolled shell/parser code is close to open-ended by construction, so budget for that going in rather than discovering it at round 3. The cap bounds the severity-floor fixed set only.

**Every review call's brief states its round number for this band.** Round 1 gets the standard brief. From round 2 onward, the brief also instructs the reviewer to report only clear, concrete, reproduced findings against the touched file — not a theoretical edge case in the underlying grammar or format the fix touches. A round-2+ call that surfaces only theoretical findings is not a new consecutive round for the cap above.

**Exception.** A band of more than three tasks, or any task on a security boundary, also gets a per-task `/assess-bugs` at P2.2, before that task's commit. **Same round cap and round-numbered brief as P2.3** governs that per-task call too — it counts toward the touched file's round total for the band, same budget, same rules.

---

## P2.4 · Closeout

**Once per band, not per-task** — after P2.3's band review has resolved its severity floor, before `/workflow-consolidate` runs.

**Only `/changelog-write` runs here**, covering every task the band shipped. No `assess-*` repeat — P2.3 already covered bugs, simplification, and (where warranted) security and performance for the whole band — and no `/backlog-audit` — no phase runs a band-scoped audit right now (TSK-01.4.4 moves that pass into P3).

**The four standing closeout tasks are a separate `Quality Assurance & Epic Hardening` task group, not this phase's job.** Their `Trigger` bullet fires only once every functional task group in the epic reaches `[DONE]` — an ordinary band's `/changelog-write` never satisfies them. When this band's own commit happens to be the one that clears the epic's last functional task group, pick up that task group in P1's next run like any other open work; nothing here special-cases it.

**Ends when:** `CHANGELOG.md`'s `[Unreleased]` section reflects the band.

---

## Adding a second way of working

A non-code lifecycle supplies its own `ways/<name>.md` **and its own forked phase skills**. The machine in `SKILL.md` does not change; the gates and the fork agents do.
