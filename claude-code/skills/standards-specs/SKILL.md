---
name: standards-specs
description: Read-only MCP fetch contract for the SDD and the optional PRD — both are Google Docs in Drive, never repo files. Section maps, the fetch rule, the requirement-ID scheme, and why neither is ever cached locally.
user-invocable: false
---

# The SDD and PRD — Google Drive, read-only

Both documents live as Google Docs in Drive. Neither is a repo file, neither is scaffolded by `write-repo`, and no skill in this toolkit creates, edits, or resolves a `NEEDS DECISION` in either — authoring happens with stakeholders directly in Drive. This toolkit only fetches and reads.

## The SDD

**Required, one per repo.** The technical source of truth — architecture, data model, interfaces, decisions, recovery. Fetch it with the Google Drive MCP tools (`search_files`/`get_file_metadata` to locate it, `read_file_content`/`download_file_content` to read it) whenever a skill needs its content; never assume a cached copy is current.

Section map an SDD carries: §0 Glossary, §1 Components, §3 app-type-specific detail, §5 Threats/Controls, §6 Testing tiers, §7 Observability + PRD requirement priorities (when a PRD backs the repo), §9 Infrastructure & Delivery, §10 Recovery + PRD phase split, §11 Decisions (each contentious or expensive-to-reverse call, appended in place as its own dated entry, never a separate file), §13 Open Items, §14 References.

**Decisions are appended, never overwritten.** A new entry in §11 gets a sequential number — write it as a dated sub-section when the decision is contentious, expensive to reverse, or too much sequencing for a table row — and never edit or renumber a prior entry.

## The PRD

**Optional.** Business scope, requirement priorities, and the requirement-ID scheme — read via the same MCP tools. Section map: §4 scope boundary, §7 requirement IDs + Must/Should/Could priority, §9 business risk, §10 V1/V2 phase split.

**IDs are minted only in the PRD's §7**, never invented by a skill reading it. A repo with no PRD simply has no requirement IDs to cite anywhere — that is not a gap to fill.

## No fallback tier

The SDD answers *how it works*; the PRD, when one exists, answers *scope, priority, and what counts as success*. Each question has exactly one owner. "Fetch the PRD when the SDD looks thin" is not a rule this toolkit implements — the agent judging *thin* is the one that would otherwise invent, so the trigger would fire never or always.

## No fork ever fetches

`workflow-planner` and `workflow-implementer` declare no MCP tools — a forked phase cannot reach Drive even if it wanted to. Whatever a fork needs from either document arrives already resolved in `$ARGUMENTS`. Fetching happens in the main session only.

## Never cached locally

A fetched section is never written into a repo file — that would be exactly the kind of static reference that goes stale the moment Drive changes. Read it fresh each time a skill needs it.
