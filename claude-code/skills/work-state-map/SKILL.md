---
name: work-state-map
description: Render a repo's whole work state — epics, task groups, and the release spine — as a published artifact, generated from BACKLOG.md and CHANGELOG.md.
argument-hint: [repo root]
---

# Work state map

Mapping: **$ARGUMENTS** (default: the current repo root).

One picture of what is planned, what is open or blocked, and what shipped — the two halves that otherwise live in two files read end to end.

**The map is a view, never a record.** `BACKLOG.md` is the record of open work and `CHANGELOG.md` is the record of shipped work; both remain the only sources of truth. The map is regenerated from them and is always disposable. Never edit either file from this skill, never reconcile a difference by changing the record to match the picture, and never treat a published artifact as state — a stale artifact is corrected by regenerating it, and `backlog-modify` / `changelog-write` own the files themselves.

## Process

1. **Generate the graph.** `python3 scripts/work_state.py` from the repo root, or `--repo-root <dir>` for another repo. Standard library only, no install step. It prints JSON to stdout and writes warnings to stderr.
2. **Do not re-parse the markdown.** The parser is the contract; this skill consumes its JSON and nothing else. That separation is what lets the renderer be replaced without touching the parser.
3. **Report the parser's warnings** in the turn summary. A missing source file, a task heading with no status marker, or an unresolvable `(after:)` target is reported, not crashed on — and not silently dropped from the picture either.
4. **Build the page and publish it** with the Artifact tool, per the renderer contract below.

## What the JSON carries

A node is `epic`, `task_group`, or `release`. There is no node per task or subtask — a task group's `counts` (`todo` / `in_progress` / `blocked` / `done` / `total`), `priorities`, and `acceptance` are how work at that level becomes visible. Every node carries its identity, `level`, `state`, and edges, and **no coordinate**: layout belongs to the renderer, which is what keeps this same output consumable by a future 3D renderer.

Edges are `contains` (epic to task group), `after` (a dependency between two task groups, lifted from a task's `(after:)` annotation; `intra_group` marks a pair inside one group), and `follows` (each release to the one before it).

**Releases are a separate spine, not linked per task group.** A closed-out band's tasks are deleted from `BACKLOG.md` and survive only as `CHANGELOG.md` prose, which carries no `TG-` identifiers. There is no linkage in the data, so the map does not draw one — a fuzzy title match would be invention. Say this when presenting the map if someone asks why shipped work does not connect back to a group.

## Renderer contract

**Mermaid, no library.** The artifact runtime renders a ```mermaid fence natively, so the map needs no CDN script, no build step, and no version pin — which is also the whole of AC-4 (zero setup). A graph library would only be warranted if the map needed force-directed layout or a 3D camera; it does not, and the JSON is coordinate-free, so that upgrade stays available without a parser change. If one is ever added it must come from a CSP-reachable host, pinned to an exact version, as a UMD build defining a global.

- **Inline the JSON.** The page cannot fetch its own data — every request to an outside host is blocked. Embed the parser's output in the page at publish time.
- **States are visually distinct** (AC-3): give `open`, `in_progress`, `blocked`, `done`, and `empty` each their own fill via `classDef`, and show the state's meaning in a legend rather than relying on color alone. The current released version is labeled as such and rendered distinctly from the `Unreleased` node beside it.
- **Group by epic** using a Mermaid `subgraph` per epic, with each task group's node label carrying its title and its open / blocked / done counts.
- **Always quote the label, and escape what quoting cannot carry.** Real titles contain `&` and backticked identifiers, and appending counts reads as a parenthetical — an unquoted `(` or `)` is a shape delimiter and breaks the whole diagram, so every node and subgraph label is written as `ID["<text>"]`, double quotes included, with no exceptions for a label that happens to look safe. Inside that text, apply all four rules before emitting:

  | In the title | Emit |
  |---|---|
  | `"` | `&quot;` |
  | `#` | `&#35;` |
  | `` ` `` | drop the backticks, keep the identifier |
  | `&` | `&amp;` |

  Escape `&` first, or the entity prefixes the other three rules produce get double-escaped. A `subgraph` needs the same treatment in its own quoted title, and Mermaid needs its id separate from its label: `subgraph EPIC01["…"]`, never a bare title.
- **Draw the release spine** as its own chain, visually separated from the epic subgraphs.
- **Responsive to about 400px** and **theme-aware** — the page around the diagram follows the artifact contract, and wide content (the diagram included) scrolls inside its own container rather than widening the page.

## Report

State the released version, the totals by state, and any parser warning. Give the artifact link. Say plainly that the map was generated from the two record files and is a view over them.
