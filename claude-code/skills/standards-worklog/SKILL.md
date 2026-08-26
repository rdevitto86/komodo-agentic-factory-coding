---
name: standards-worklog
description: Read/write directive shared by the two mutable work records — BACKLOG.md and CHANGELOG.md. Not their format; see generate-backlog and generate-changelog for that.
user-invocable: false
paths: "**/BACKLOG.md, **/CHANGELOG.md"
---

# The work records

`BACKLOG.md` is what is still open. `CHANGELOG.md` is what shipped. Both live at the repo root, both are written during a build, and both are mutable — unlike the frozen `docs/sdd.md`.

**Structure is not here.** `generate-backlog` defines the `BACKLOG.md` format; `generate-changelog` defines the `CHANGELOG.md` format. Load whichever one you're about to write before writing it — this skill is the behavior shared across both, not either one's shape.

## Directives

- **Read the current file before writing.** Never duplicate a line already there, and never write a `BACKLOG.md` story or `CHANGELOG.md` entry the file already carries in substance.
- **Never invent.** A record states what is true now — a story that is open, an entry that shipped — never a guess dressed as either.
- **Nothing writes back into the frozen spec.** A `BACKLOG.md` story or `CHANGELOG.md` entry may cite a PRD requirement ID (when a PRD exists in Drive) for traceability; `docs/sdd.md` is otherwise untouched by a build.
- **Test stories are decomposed when a story is written, never invented later.** A domain with behavior stories and no `Tests:` story is an incomplete decomposition, not a gap to fill during implementation.
- **Never dump raw audit, review, or scan output straight into either file.** Report the findings; the user decides what becomes a line.
