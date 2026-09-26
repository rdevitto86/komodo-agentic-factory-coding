# Product requirements — <Repo Name>

| Metadata | Specification |
|---|---|
| Product name | <Product name> (`<Repo Name>`) |
| Document version | <x.y.z> |
| Target milestone | <The release this document targets> |
| Document owner | <Role> |
| Status | Draft, {{DATE}} |
| Design | `docs/architecture.md`, `docs/system-design.md`, `docs/decisions.md` |
| Change control | Only the owner edits this file. |

Why the system exists and what must be true. Planner-facing. A repo with no PRD has no requirement IDs to cite, and that is not a gap to fill. IDs are minted only under Requirements below, never by anything reading this document.

## Executive summary and vision

<What the product does, for whom, and the outcome it guarantees, in one or two short paragraphs.>

## Problem statement and objectives

### Problem statement

<Each problem as a bold lead and one sentence of evidence.>

### Strategic objectives

<Each objective as a bold lead and what is true once it is met.>

## User personas and environments

| Persona | Environment | Primary execution context |
|---|---|---|
| <Persona> | <Platform> | <What they use it for> |

## Product scope

| Functional area | In scope (<version>) | Out of scope |
|---|---|---|
| <Area> | <What ships> | <What waits> |

## Lifecycle workflow

<The stages a unit of work passes through, as the product sees them, with each stage's limits. The component flow lives in architecture.md.>

## Success criteria

<The gates the release must pass, each one measurable.>

## Requirements

<Every number the system is held to lives here; other files cite its ID. Group requirements by area under their own headings.>

| Requirement ID | Requirement description | Priority | Verification proof |
|---|---|---|---|
| REQ-1 | <requirement text> | <Must / Should / Could> | <a command that exits zero when it holds> |

## Constraints

<What bounds the design and what is taken as given, including the dependency policy.>

## Risks and mitigations

| Risk | Severity | Root cause | Mitigation |
|---|---|---|---|
| <Risk> | <Critical / High / Medium / Low> | <Why it could happen> | <What prevents it, citing its REQ-n> |

## Open questions

<Product questions still unresolved, and who resolves each. A technical question is a Proposed entry in decisions.md.>
