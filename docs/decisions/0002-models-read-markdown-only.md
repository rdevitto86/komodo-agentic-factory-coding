# 0002. Models read markdown, never Go

**Status:** Accepted, 2026-09-21.

## Context

A model that reads the binary's source can reason about the station order and route around it. Every token it spends on Go is a token not spent on the task.

## Decision

Rules, roles, skills, standards, facets, and policy are markdown or JSON under `komodo/`. The binary fills a brief from them and a model reads only the brief. `komodo step` prints the one next action, so the station order lives only in the binary.

## Alternatives

- **Prompt code inside Go.** It is faster to change but invisible to review, and a host swap would touch it.
- **A model-driven orchestrator.** It is flexible, but it spends tokens on control flow, and one bad turn skips a station.

## Consequences

- **A reviewer reads one brief.** It is the whole input a builder gets.
- **Swapping a model is a profile row.** No text a model reads changes.
- **Go stays out of every slot.** `komodo diff` skips `bin/`, and no role asks for source it does not edit.
