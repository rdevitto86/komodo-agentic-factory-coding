---
name: standards-prd
description: How to read a PRD from Google Drive — what each section is authoritative for, when to fetch one, and the requirement-ID contract with the SDD. Optional per repo; docs/sdd.md is the source of truth.
user-invocable: false
---

# The PRD

A PRD answers *what are we building and why*. It is **optional per code repo**, lives as a Google Doc in Drive, and is **never authored from a coding session** — that happens with stakeholders elsewhere. `docs/sdd.md` (`generate-sdd`) is the source of truth for a repo; this skill is how you read the business context behind it when a question genuinely needs it.

**Read-only, always.** Never create, edit, or resolve a `NEEDS DECISION` in a PRD from here — including appending `(decided <date>)`. A PRD is frozen at approval and amended by the people who own it. A change it needs is a finding you report, exactly like one the SDD needs.

## When to fetch — and when never

The SDD and the PRD do not overlap, so there is no "fall back to the PRD when the SDD is thin." Each question has one owner. When that owner is the PRD and no PRD exists, the answer comes from the user — never from the SDD, never invented.

| The question | Owner |
|---|---|
| How does it work · what's the design | `docs/sdd.md` — never the PRD |
| What's open · what shipped | `BACKLOG.md` · `CHANGELOG.md` |
| Is this in scope · which comes first · Must or Could | PRD §4, §7, §10 |
| Why does this exist · what counts as success · who for | PRD §1, §8, §6 |

**Three fetch points, all in the main session:**

| Trigger | Fetch |
|---|---|
| Drafting or amending `docs/sdd.md` (`workflow-loop` P0) | §4, §7, §10 |
| Writing or normalizing stories (`generate-backlog`) | §7 priorities, §10 phase split |
| The cross-reference audit (`audit-sdd`) | §7 requirement IDs |

**Never fetch from a forked phase.** `planner` and `implementer` carry no MCP tools and cannot reach Drive at all — anything a fork needs is passed in its `$ARGUMENTS` by the caller, the same rule that makes `workflow-implement` require its `Done when` explicit.

**Never at session start.** A session must not wait on a network round-trip to begin — the same rule `context_injector.py` follows for the bridge.

**Never during implement, verify, review, or consolidate.** A task carries its own `Done when`, and business context does not change how the code is written.

**Fetch the section, not the document.** A whole PRD runs ~2,700 words; §7's table alone is a fraction of that. Pull only what the trigger names.

## Finding one

`mcp__claude_ai_Google_Drive__search_files` for `PRD — <repo-name>`, then `read_file_content` on the hit.

**No hit is not a finding.** Most repos have no PRD — that is the expected state, not a gap. Say so in one line and carry on with the SDD.

**Never guess which document is the PRD.** Two plausible matches, or a title that doesn't follow the convention, is a question for the user, not a coin flip.

## The sections

| § | Holds | Authoritative for |
|---|---|---|
| 1 | The Problem | why this exists at all |
| 2 | The Solution | what this owns vs. deliberately doesn't |
| 3 | Goals | the G-numbered outcomes |
| 4 | Not in Scope | **the scope boundary** — the SDD has no equivalent |
| 5 | Assumptions & Dependencies | what the plan takes as given |
| 6 | Who This Serves | stakeholder roles and their needs |
| 7 | Requirements | **requirement IDs and Must/Should/Could priority** |
| 8 | Success Metrics | what "it worked" means, numerically |
| 9 | Risks | business risk — distinct from SDD §12's technical risk |
| 10 | Timeline / Phases | **the V1/V2 split** |
| 11 | Glossary | business and legal terms |

The three bolded rows are what the loop actually consumes. The rest are read only on a direct question.

## Requirement IDs

A 2–3 letter prefix per capability group plus a number — `CP1`, `CP2`, `DC1`. A prefix is assigned once per group and never reused for a different one.

**The IDs are already in the repo.** `generate-sdd`'s cross-reference contract puts every §7 ID into SDD §1 or §11 alongside a one-line meaning, so citing an ID in a story or a changelog entry needs no fetch — read the SDD. What is *not* local, and the whole reason to fetch §7, is each ID's **priority**.

**Never mint an ID.** They are minted in the PRD by the people who own it. A capability with no ID either has no PRD behind it — normal, and the story simply omits the field — or is undocumented scope, which is a finding.
