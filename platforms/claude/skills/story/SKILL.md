---
name: story
description: Generate a work item/story ready to paste into the tracker in play. Argument = what the work is about (plain description of the feature/bug), optionally naming a tracker.
argument-hint: <what the story is about — optionally "for TODO.md" / "JIRA story for..." / tracker name>
---

# Story

What the story is about: $ARGUMENTS

Parse the tracker from the argument if named (e.g. "for TODO.md", "JIRA story for..."); default to Trello if none is named.

## Runtime

1. Check whether the local MCP `pm` agent is reachable (komodo bridge at `http://localhost:8000/sse`). If reachable, call `analyze_specs` with the parsed subject and tracker and use its output — zero token cost, primary runtime.
2. If unreachable, fall back to `agents/business-architect/agent.md`'s `stories` mode directly: one shippable outcome, mandatory context + acceptance criteria + out-of-scope, relative sizing (S/M/L, never hours), flag anything XL as needing a breakdown first, link dependencies explicitly.

## Output

Format ready to paste into the named tracker (Trello card, TODO.md bullet, JIRA ticket, etc.) per that agent's output contract. Lead with the title; use lists, not prose. Surface any out-of-scope work discovered while drafting as a separate item rather than folding it in.
