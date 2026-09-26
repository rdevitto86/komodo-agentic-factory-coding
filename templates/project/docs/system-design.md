# System design — <Repo Name>

How each part works. A builder reads one section of this file through a task's `context`, such as `docs/system-design.md#interfaces`; nobody reads it whole. Headings carry no numbers, so a citation survives a reorder. A requirement's number is cited by its `REQ-n`, never copied. A contract, manifest, schema, or migration is linked, never restated. A section that does not apply says "Not applicable."

## Data model

<Entities, fields, ownership, and the migrations that change them. Link the schema and migration files.>

## Interfaces

<Each API, event, or command: its version, its consumers, and a link to its contract file.>

## User interface

<Screens, flows, and states. Link the design files and tokens.>

## Integrations

<Each external system: what is called, how it authenticates, its limits, and what happens when it fails. Package dependencies live in the manifests.>

## Cross-cutting concerns

<Security threats and mitigations, error handling, and how each quality requirement is met.>

## Operations

<Deployment, configuration, and observability: what is emitted and where it is read.>

## Recovery

<Failure modes and the runbook for each.>

## Testing

<The tiers of tests, what each proves, and the command that runs it.>
