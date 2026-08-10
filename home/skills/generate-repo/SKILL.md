---
name: generate-repo
description: Scaffold or refresh a repo's local CLAUDE.md/AGENTS.md pair.
argument-hint: [target repo path, defaults to cwd]
disable-model-invocation: true
---

# Generate Repo

Target: **$ARGUMENTS** (defaults to the current working directory).

This skill owns both the contract and the writing. Anything that later checks a repo for drift reads this same file, so there is exactly one definition to keep current.

---

## The contract

### Why two files

Claude Code auto-loads `CLAUDE.md` only, on every session, never `AGENTS.md` directly (verified against `code.claude.com/docs/en/memory.md`). `AGENTS.md` is the portable, tool-agnostic file; `CLAUDE.md` is the trigger that makes Claude Code read it.

### CLAUDE.md — fixed, one line

```
@AGENTS.md
```

Nothing else. If a genuinely Claude-Code-only instruction is ever needed, it goes below the import line and never restates anything the import already covers.

### AGENTS.md — fixed shape, ~300-token budget

1. **Header** — one line: repo name + one-line purpose.
2. **Quick reference** — a table. The field set is the **Quick-reference fields** section of the matching language skill (`go`, `typescript`, `python`, `svelte`, `vue`, `cdk`), never invented per repo.
3. **Docs pointer** — one line: `docs/` holds architecture, PRD, SDD, ADRs (see the `docs` skill for their structure) — read on demand before design work, never pre-loaded.
4. **Deviations** — **refresh runs only.** See below.

No fifth section. A repo that needs one is a signal the content belongs in `docs/`, not here.

### Single-ownership rule

**This skill references a layout or a field set. It never restates one.** The repo layout tree and the Quick-reference field set are owned by the language skill, which an agent already has loaded while writing code in that language — so there is exactly one copy and it cannot drift from what the agent is following. If a language skill lacks either section, add it there and reference it from here; never inline it into this file as a stopgap.

### No-duplication rule

A line only belongs in this file if it is a fact that `tech-stack`, `sdlc`, `cicd`, `coding-principles`, or the matching language skill cannot already produce by name. Concretely: this repo's actual port number belongs here; the meaning of "component test" does not. Restating global or language-skill content is not a convenience — it is the exact failure mode that let two live repos' `AGENTS.md` go stale after `home/` was restructured.

---

## Step 1 — Load the facts

- Load `tech-stack` for the standard repo shape (folders, port convention, entrypoint convention).
- Detect the language from the repo root (`go.mod`, `package.json`, `pyproject.toml`) and load the matching language skill. For a `package.json` repo, narrow further: `svelte` or `vue` if a UI framework is present, `cdk` if `aws-cdk-lib` is, otherwise `typescript`.
- Take the Quick-reference field set and the expected layout tree from that skill's **Quick-reference fields** and **Repo layout** sections.

## Step 2 — Read the repo, don't ask for what's discoverable

Pull Quick-reference values from the repo itself: module/package name, declared port(s) (`docker-compose.yaml`, `main.go`/config), contract file (`openapi.yaml` if present), entrypoint path. Only ask the user for a value that genuinely isn't in the repo.

## Step 3 — Branch on scaffold vs refresh

| Branch | Condition | Sections written |
|---|---|---|
| Scaffold | No `AGENTS.md` | Header, Quick reference, Docs pointer |
| Refresh | `AGENTS.md` exists | All four, Deviations included |

**Scaffold never writes a Deviations section.** A new repo has no drift to record, and an empty header is still a cost.

**Refresh diffs the repo's actual layout and conventions against the `tech-stack` skill's standard shape.** Only what differs becomes a Deviations line. Matching the standard produces zero lines, not a line saying so.

## Step 4 — Write or propose

- **`CLAUDE.md` missing** → write the fixed one-liner `@AGENTS.md`.
- **`AGENTS.md` missing** → write the scaffold shape, filled from Step 2.
- **`AGENTS.md` exists** → do not overwrite. Diff the proposed content against what's there, show the diff, and apply only on confirmation — this is a "propose, don't impose" action per the global rules, not a silent regeneration.

## Step 5 — Enforce the budget

Count the drafted `AGENTS.md`. Over ~300 tokens is a stop condition, not a warning: name which line is pushing it over and whether that content belongs in `docs/` instead. Never ship an over-budget file to "fix later."

## Out of scope

No cross-repo scanning. This skill only ever touches the one target repo in a single run — comparing many repos for drift is a separate, later capability, not this skill's job.
