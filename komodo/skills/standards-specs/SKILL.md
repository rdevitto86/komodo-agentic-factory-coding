---
name: standards-specs
description: The three spec files, which section each one owns, and how a task cites them.
globs: ["**/architecture.md", "**/system-design.md", "**/prd.md"]
roles: [planner]
---

# Specs: architecture, system design, and an optional PRD

A repo keeps its design in `docs/spec/`, or in its `README.md` when that is where it lives; a task cites whichever holds the section. Nothing here is required, and a missing file is not a gap to fill. Plain headings, no numbers, so `docs/spec/system-design.md#interfaces` survives a reorder.

## `architecture.md`, the stable shape

Small enough to read whole. Owns: Purpose, Components, Boundaries, Data flow, Decisions. A decision is appended in place with a date and the alternative rejected; design rationale lives there, not in code comments.

## `system-design.md`, the detail

Read one section at a time through a task's `context`. Owns: Data model, Interfaces, Non-functional requirements, Operations, Recovery, Testing, Open items.

## `prd.md`, optional

Planner-facing. Owns: Problem and outcome, Users and scenarios, Scope, Success metrics, Constraints and assumptions, Open questions, Requirements. Requirement IDs (`REQ-n`) are minted only under Requirements, nowhere else.

## Rules

- **Every heading lives in exactly one file.** A task cites one place, and the files never drift.
- **A task's `context` names the section it traces to.** `komodo lint` fails an anchor that names no heading.
- **The spec is frozen during a run.** A change it needs is a finding filed to the backlog, never an edit a worker makes.
