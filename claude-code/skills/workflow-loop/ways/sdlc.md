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

`/code-review` against the task. **Invoke it with the task text and which `standards-*` skills the touched files load** (the language skill at minimum; `standards-docker` for a touched `Dockerfile`/`docker-compose.yaml`, `standards-security` for a touched auth/secret/boundary path) — an unbriefed review picks its own lenses, which is not a repeatable gate. The lenses that matter:

- **Correctness** — does it do what the story said, including the edge the story named
- **Security** — new boundary, new query, new secret handling
- **Complexity** — a function that grew a fourth responsibility
- **Idiom** — does it read like the code around it
- **Comment discipline** — the guard blocks additions at write time; a review still catches one smuggled through an allowed slot

**Findings on correctness, security, or a stated requirement return to P2.1. Everything else is optional.**

---

## Adding a second way of working

A non-code lifecycle supplies its own `ways/<name>.md` **and its own forked phase skills**. The machine in `SKILL.md` does not change; the gates and the fork agents do.
