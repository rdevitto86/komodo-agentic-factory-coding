---
name: standards-specs
description: Section maps and read contract for the SDD and the optional PRD — both are repo files under docs/spec/, scaffolded from templates/project and published by MkDocs Material. Section maps, the requirement-ID scheme, and how a decision gets appended in place.
user-invocable: false
---

# The SDD and PRD — repo files under docs/spec/

Both documents live as repo files: `docs/spec/SDD.md` and `docs/spec/PRD.md`, scaffolded from `templates/project/docs/spec/SDD.md` and `templates/project/docs/spec/PRD.md` by `git-repo-init`. `git-repo-init` only ever writes the empty stub — content-level authoring, editing, and audit against the section map below is the `sdd`/`prd` skills' job, done with stakeholders, never invented from nothing. This toolkit reads them like any other repo file, with the plain Read tool — no fetch step, no MCP tool, no distinction between the main session and a fork.

## Doc site — MkDocs Material

`templates/project/mkdocs.yml.tmpl` wires `docs/spec/` into a MkDocs Material site: `docs_dir: docs`, `theme.name: material`, and a `nav` entry for the PRD and the SDD (alongside the ADR and runbook trees). A repo that scaffolds `docs/spec/` gets `mkdocs.yml` copied alongside it with `{{NAME}}` filled in — that config is how the documents get built and served (`mkdocs build`/`mkdocs serve`), never a separate authoring surface.

## The SDD

**Required, one per repo.** The technical source of truth — architecture, data model, interfaces, decisions, recovery. Read it at `docs/spec/SDD.md` whenever a skill needs its content; it is the single copy, so there is no staleness question to weigh.

Section map an SDD carries: §0 Glossary, §1 Components, §3 app-type-specific detail, §5 Threats/Controls, §6 Testing tiers, §7 Observability + PRD requirement priorities (when a PRD backs the repo), §9 Infrastructure & Delivery, §10 Recovery + PRD phase split, §11 Decisions (each contentious or expensive-to-reverse call, appended in place as its own dated entry, never a separate file), §13 Open Items, §14 References.

**Decisions are appended, never overwritten.** A new entry in §11 gets a sequential number — write it as a dated sub-section when the decision is contentious, expensive to reverse, or too much sequencing for a table row — and never edit or renumber a prior entry.

## The PRD

**Optional.** Business scope, requirement priorities, and the requirement-ID scheme — read at `docs/spec/PRD.md`. Section map: §4 scope boundary, §7 requirement IDs + Must/Should/Could priority, §9 business risk, §10 V1/V2 phase split.

**IDs are minted only in the PRD's §7**, never invented by a skill reading it. A repo with no PRD simply has no requirement IDs to cite anywhere — that is not a gap to fill.

## No fallback tier

The SDD answers *how it works*; the PRD, when one exists, answers *scope, priority, and what counts as success*. Each question has exactly one owner. "Read the PRD when the SDD looks thin" is not a rule this toolkit implements — the agent judging *thin* is the one that would otherwise invent, so the trigger would fire never or always.
