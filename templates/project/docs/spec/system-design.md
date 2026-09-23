# System design — <Repo Name>

The detail behind `architecture.md`. A builder reads one section of this file through a task's `context`, such as `docs/spec/system-design.md#interfaces`; nobody reads it whole. Headings carry no numbers, so a citation survives a reorder.

## Data model

<Entities, fields, ownership, and the migrations that change them.>

## Interfaces

<APIs, events, and contracts: shape, versioning, and who calls them.>

## Non-functional requirements

<Performance, availability, security, and the number each one is held to.>

## Operations

<Deployment, configuration, and observability: what is emitted and where it is read.>

## Recovery

<Failure modes and the runbook for each.>

## Testing

<The tiers of tests, what each proves, and the command that runs it.>

## Open items

<What is undecided, who decides it, and what it blocks.>
