---
name: standards-specs
description: The SDD and the PRD: what each holds, who owns it, and where it lives.
globs: ["**/PRD.md", "**/SDD.md"]
roles: [planner]
---

# Specs: SDD and PRD

Both live as repo files under `docs/spec/`, read with the plain file tools. The SDD is required, one per repo; the PRD is optional.

## SDD sections
1. Purpose and scope
2. Architecture: components, boundaries, data flow
3. Data model
4. Interfaces: APIs, events, contracts
5. Non-functional requirements: performance, availability, security
6. Operations: deployment, configuration, observability
7. Recovery: failure modes and runbooks
8. Decisions: dated entries appended in place, never rewritten

## PRD sections
1. Problem and outcome
2. Users and scenarios
3. Scope in and out
4. Success metrics
5. Constraints and assumptions
6. Open questions
7. Requirements: the only place requirement IDs (`REQ-nn`) are minted

## Rules
- A task cites the SDD section or requirement ID it traces to in `context`. A repo with no PRD has no requirement IDs, and that is not a gap to fill.
- The SDD is frozen during a run. A change it needs is a finding filed to the backlog, never an edit a worker makes.
- A decision is appended to the SDD's decisions section with a date and the alternative rejected. Design rationale lives there, not in code comments.
