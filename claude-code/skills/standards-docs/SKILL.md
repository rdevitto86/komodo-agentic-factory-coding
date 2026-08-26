---
name: standards-docs
description: Read/write directive shared across docs/ — the frozen-spec vs. satellite-record split, jargon-free/tables-over-prose bar, and the SDD-as-hub cross-reference contract. Not any file's format; see generate-sdd, generate-adr, generate-runbook for that.
user-invocable: false
paths: "**/docs/**"
---

# The docs family

`docs/sdd.md` is the frozen spec — approved once, changed only by a human decision. `docs/adrs/` and `docs/runbooks/` are its satellite records — colocated under `docs/`, never a top-level `/adrs` or `/runbooks`, each existing only for what an SDD table row can't hold. A PRD, when one exists, is a Google Doc in Drive (`standards-prd`) — optional, never colocated under `docs/`, and out of scope for this skill.

**Structure is not here.** `generate-sdd`, `generate-adr`, and `generate-runbook` each define their own file's template. Load whichever one you're about to write before writing it — this skill is the behavior shared across all three, not any one's shape.

## Directives

- **The SDD is the hub.** Every ADR and runbook cites back to SDD §11 (Decisions table) or §14 (References) — owned by `generate-sdd`, never restated in the satellite file.
- **Jargon-free, tables over prose, readable in one sitting.** Applies to every file under `docs/` — a term with no plain-language gloss or §0 definition doesn't ship.
- **None of these are the entry point.** `README.md` (`generate-readme`) is — a few screens read first for what the repo is and how to run it. Every file under `docs/` holds depth README deliberately omits.
- **Load before touching.** Editing an already-open file under `docs/` mid-session counts the same as creating one.
- **Never write ahead of the need.** An ADR waits for a decision that's actually contentious or expensive to reverse; a runbook waits for a signal or recovery path already in the SDD. Neither is drafted from a hypothetical.
- **Nothing here writes back into the frozen spec.** An ADR or runbook may cite a PRD requirement ID (when a PRD exists) or SDD section for traceability; `docs/sdd.md` is otherwise untouched once approved.
