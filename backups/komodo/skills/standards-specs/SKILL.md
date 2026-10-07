---
name: standards-specs
description: The spec files in docs/ (prd, hld, lld, decisions/), the sections each owns, and how a task cites them.
globs: ["**/prd.md", "**/hld.md", "**/lld.md", "**/decisions/*.md"]
roles: [planner]
---

# Specs: prd, hld, lld and decisions, one home per fact

A repo keeps its specs in `docs/`: `prd.md`, `hld.md` (high-level design), `lld.md` (low-level design), and `decisions/`, one file per decision. Files split by who reads them and how often they change, never by topic; a topic is a section. Nothing here is required, and a missing file is not a gap to fill. Plain headings, no numbers, so `docs/lld.md#interfaces` survives a reorder.

## `prd.md`, why and what must be true

Changes per release, and only the owner edits it; the planner reads it whole. It opens with a metadata table: product, version, milestone, owner, status, design links, change control. Owns: Executive summary and vision, Problem statement and objectives, User personas and environments, Product scope, Lifecycle workflow, Success criteria, Requirements, Constraints, Risks and mitigations, Open questions. The lifecycle is the stages and their limits as the product sees them; the component flow stays in `hld.md#data-flow`. Requirement IDs (`REQ-n`) are minted only under Requirements, grouped by area, each with a priority and the command that proves it.

## `hld.md`, the parts and how they connect

Changes rarely; the planner and the reviewer read it whole. It holds what would survive a rewrite in another language: the C4 context and container levels. Owns: Purpose, Context, Components, Boundaries, Data flow, Glossary. Names and reasons only: no flags, fields, versions, or numbers.

## `lld.md`, how each part works

Changes with the code; a builder reads one section through a task's `context`. It holds the C4 component level and below. Owns: Data model, Interfaces, User interface, Integrations, Cross-cutting concerns, Operations, Recovery, Testing.

## `decisions/`, why this way and not another

One file per decision, `NNNN-<slug>.md`, never rewritten, so parallel branches add decisions without conflict. Each opens `# NNNN. <the decision as a sentence>`, numbered in order, with a status (Proposed, Accepted, or Superseded by NNNN) and a date, then Context, Decision, Alternatives, and Consequences as bold labels. An open technical question is a Proposed entry. A superseded entry keeps its text and gains one status line. A `README.md` beside them holds the log's preamble, never an index. `komodo lint` refuses two files with one number; the branch that lands second renumbers.

## `diagrams/` and `media/`, what the prose shows

`docs/diagrams/` holds each diagram's editable source, such as a `.drawio` file, beside its exported `.png` or `.svg` under the same name; a spec links the export, and a source edit re-exports in the same change. `docs/media/` holds the static images, videos and GIFs a doc shows. Neither holds a fact the prose lacks: a diagram illustrates a section, it never replaces one.

## Where a topic goes

| Topic | Section | Linked, never restated |
|---|---|---|
| UI design | `lld.md#user-interface` | design files, tokens |
| API design | `lld.md#interfaces` | OpenAPI, AsyncAPI, proto, JSON Schema files |
| Integrations | names in `hld.md#context`; the rest in `lld.md#integrations` | contract files |
| Dependencies | policy in `prd.md#constraints` | manifests and lockfiles |
| Security | trust in `hld.md#boundaries`; threats in `lld.md#cross-cutting-concerns` | |
| Testing | `lld.md#testing` | test commands, CI config |
| Operations | `lld.md#operations` and `#recovery` | deploy and alert config |

## Rules

- **Every heading lives in exactly one file, and every fact in one place.** Another file cites it by anchor, never restates it.
- **A number a requirement sets lives only in the PRD.** Other files cite its `REQ-n`.
- **Machine-readable files are the source.** A contract, manifest, schema, or migration is linked from prose, never copied into it.
- **Company standards are never restated.** They live in the `standards-*` skills; a repo records a deviation as a decision.
- **A section that does not apply says "Not applicable."** It is never deleted, so the set stays the same in every repo.
- **A task's `context` names the section or decision file it traces to.** `komodo lint` fails an anchor that names no heading.
- **The spec is frozen during a run.** A change it needs is a finding filed to the backlog, never an edit a worker makes.
- **Older shapes migrate by renaming.** `architecture.md` becomes `hld.md`; `system-design.md` or an SDD becomes `lld.md`; a decisions file or table splits into `decisions/`.
