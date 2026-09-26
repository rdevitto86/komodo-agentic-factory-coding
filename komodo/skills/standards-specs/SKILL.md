---
name: standards-specs
description: The four spec files in docs/, the sections each one owns, and how a task cites them.
globs: ["**/prd.md", "**/architecture.md", "**/system-design.md", "**/decisions.md"]
roles: [planner]
---

# Specs: four files, one home per fact

A repo keeps its specs in `docs/`: `prd.md`, `architecture.md`, `system-design.md`, and `decisions.md`. Files split by who reads them and how often they change, never by topic; a topic is a section. Nothing here is required, and a missing file is not a gap to fill. Plain headings, no numbers, so `docs/system-design.md#interfaces` survives a reorder.

## `prd.md`, why and what must be true

Changes per release, and only the owner edits it; the planner reads it whole. It opens with a metadata table: product, version, milestone, owner, status, design links, change control. Owns: Executive summary and vision, Problem statement and objectives, User personas and environments, Product scope, Lifecycle workflow, Success criteria, Requirements, Constraints, Risks and mitigations, Open questions. The lifecycle is the stages and their limits as the product sees them; the component flow stays in `architecture.md#data-flow`. Requirement IDs (`REQ-n`) are minted only under Requirements, grouped by area, each with a priority and the command that proves it.

## `architecture.md`, the parts and how they connect

Changes rarely; the planner and the reviewer read it whole. Owns: Purpose, Context, Components, Boundaries, Data flow, Glossary. Names and reasons only: no flags, fields, versions, or numbers.

## `system-design.md`, how each part works

Changes with the code; a builder reads one section through a task's `context`. Owns: Data model, Interfaces, User interface, Integrations, Cross-cutting concerns, Operations, Recovery, Testing.

## `decisions.md`, why this way and not another

Append-only. Each entry is `## NNNN. <the decision as a sentence>`, numbered in order, with a status (Proposed, Accepted, or Superseded by NNNN) and a date, then Context, Decision, Alternatives, and Consequences as bold labels. An open technical question is a Proposed entry. A superseded entry keeps its text and gains one status line.

## Where a topic goes

| Topic | Section | Linked, never restated |
|---|---|---|
| UI design | `system-design.md#user-interface` | design files, tokens |
| API design | `system-design.md#interfaces` | OpenAPI, AsyncAPI, proto, JSON Schema files |
| Integrations | names in `architecture.md#context`; the rest in `system-design.md#integrations` | contract files |
| Dependencies | policy in `prd.md#constraints` | manifests and lockfiles |
| Security | trust in `architecture.md#boundaries`; threats in `system-design.md#cross-cutting-concerns` | |
| Testing | `system-design.md#testing` | test commands, CI config |
| Operations | `system-design.md#operations` and `#recovery` | deploy and alert config |

## Rules

- **Every heading lives in exactly one file, and every fact in one place.** Another file cites it by anchor, never restates it.
- **A number a requirement sets lives only in the PRD.** Other files cite its `REQ-n`.
- **Machine-readable files are the source.** A contract, manifest, schema, or migration is linked from prose, never copied into it.
- **Company standards are never restated.** They live in the `standards-*` skills; a repo records a deviation as a decision.
- **A section that does not apply says "Not applicable."** It is never deleted, so the set stays the same in every repo.
- **A task's `context` names the section it traces to.** `komodo lint` fails an anchor that names no heading.
- **The spec is frozen during a run.** A change it needs is a finding filed to the backlog, never an edit a worker makes.
