---
name: standards-worklog
description: Read/write directive shared by the two mutable work records — BACKLOG.md and CHANGELOG.md. Not their format; see backlog-modify and changelog-write for that.
user-invocable: false
paths: "**/BACKLOG.md, **/CHANGELOG.md"
---

# The work records

`BACKLOG.md` is what is still open. `CHANGELOG.md` is what shipped. Both live at the repo root, both are written during a build, and both are mutable — unlike the frozen SDD, which is a Drive doc this toolkit never writes at all.

**Structure is not here.** `backlog-modify` defines the `BACKLOG.md` format; `changelog-write` defines the `CHANGELOG.md` format. Load whichever one you're about to write before writing it — this skill is the behavior shared across both, not either one's shape.

## Directives

- **Read the current file before writing.** Never duplicate a line already there, and never write a `BACKLOG.md` story or `CHANGELOG.md` entry the file already carries in substance.
- **Never invent.** A record states what is true now — a story that is open, an entry that shipped — never a guess dressed as either.
- **Nothing writes back into the frozen spec.** The SDD is untouched by a build — it isn't even a repo file to touch.
- **Test stories are decomposed when a story is written, never invented later.** A domain with behavior stories and no `Tests:` story is an incomplete decomposition, not a gap to fill during implementation.
- **Never dump raw audit, review, or scan output straight into either file.** Report the findings; the user decides what becomes a line.
