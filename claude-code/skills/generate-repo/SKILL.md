---
name: generate-repo
description: Create a new repo's full skeleton, or scaffold/refresh its CLAUDE.md/AGENTS.md and the four standard documents.
argument-hint: [repo-type: go-api|vue-ui|svelte-ui|cdk-infra] [target path, defaults to cwd]
disable-model-invocation: true
---

# Generate Repo

Target: **$ARGUMENTS** — first token is the repo type (`go-api`, `vue-ui`, `svelte-ui`, `cdk-infra`), second is the target path (defaults to cwd). Repo type is only needed when the target has no code yet; an existing repo's language is detected from what's on disk, as before.

This skill owns both the contract and the writing. Anything that later checks a repo for drift reads this same file, so there is exactly one definition to keep current.

---

## The contract

### Why two files

Claude Code auto-loads `CLAUDE.md` only, on every session, never `AGENTS.md` directly (verified against `code.claude.com/docs/en/memory.md`). `AGENTS.md` is the portable, tool-agnostic file; `CLAUDE.md` is the trigger that makes Claude Code read it.

### Every file's shape is owned elsewhere, never restated here

| File | Shape owned by |
|---|---|
| `CLAUDE.md`, `AGENTS.md` | `templates/project/*.tmpl` |
| `BACKLOG.md`, `CHANGELOG.md` | `templates/project/*.tmpl` |
| `docs/prd.md`, `docs/sdd.md` | The `docs` skill's section skeletons |

**The split is not arbitrary.** A `.tmpl` exists where the file ships with *seeded content* — closeout stories, an initial version heading. The PRD and SDD ship as pure section skeletons with nothing filled in, so duplicating them into a `.tmpl` would give the format two owners and let them drift.

This skill fills placeholders and writes the result — it never re-describes a shape in prose. If a placeholder changes, edit the `.tmpl`; if a seed story changes, edit the language skill's `Seed backlog` section.

`AGENTS.md.tmpl`'s Quick-reference table takes its **field set** from the matching language skill's **Quick-reference fields** section (`go`, `typescript`, `python`, `svelte`, `vue`, `cdk`) — never invented per repo. Its Deviations section is **refresh-only** (see Step 1); a new repo has no drift yet, so Create and Scaffold leave it out entirely.

### Single-ownership rule

**This skill references a layout, a field set, or a seed story. It never restates one.** The repo layout tree, the Quick-reference field set, and the `Seed backlog` stories are owned by the language skill, which an agent already has loaded while writing code in that language — so there is exactly one copy and it cannot drift from what the agent is following. If a language skill lacks any of the three sections, add it there and reference it from here; never inline it into this file as a stopgap.

### No-duplication rule

A line only belongs in a generated `AGENTS.md` if it is a fact that `sdlc`, `cicd`, or the matching language skill cannot already produce by name. Concretely: this repo's actual port number belongs there; the meaning of "component test" does not. Restating global or language-skill content is not a convenience — it is the exact failure mode that let two live repos' `AGENTS.md` go stale after `claude-code/` was restructured.

---

## Step 1 — Determine the branch

| Branch | Condition | What it writes |
|---|---|---|
| Create | Target path missing or empty | Full skeleton, `AGENTS.md`, all four documents |
| Scaffold | Target has code but no `AGENTS.md` | `AGENTS.md`; appends seed stories to an existing `BACKLOG.md` |
| Refresh | `AGENTS.md` exists | All sections, Deviations included; appends seed stories |

**Only Create writes the four documents from scratch.** `BACKLOG.md` and `CHANGELOG.md` come from their `.tmpl`, with the repo type's `Seed backlog` stories spliced into `Cross-Cutting`. `docs/prd.md` and `docs/sdd.md` are written as empty section skeletons from the `docs` skill — every section present, every body `NEEDS DECISION`.

**Never fill a PRD or SDD section during generation.** A generated repo hands the user two skeletons to complete, not a spec invented from a repo type. That gate is the whole point of the document set.

**Scaffold and Refresh never create or restructure `BACKLOG.md`** — its domains and stories stay `backlog`'s territory. When one already exists, the only touch either branch makes is appending seed stories not already present under `Cross-Cutting`, matched by text. Nothing else is read, reordered, or rewritten.

**Neither Create nor Scaffold writes a Deviations section.** Nothing has had a chance to drift yet, and an empty header is still a cost.

**Refresh diffs the repo's actual layout and conventions against the language skill's `Repo layout — <token>` tree, and its actual practice against `sdlc` (coverage floors, test tiers), `cicd` (pipeline stages), and `security` wherever the repo visibly departs from one of them.** Only what differs becomes a Deviations line, each tagged with the skill it departs from. Matching it produces zero lines, not a line saying so.

## Step 2 — Load the facts

- **Create** — the repo type token names the language skill directly (`go-api` → `go`, `vue-ui` → `vue`, `svelte-ui` → `svelte`, `cdk-infra` → `cdk`). Load it and read its `Repo layout — <token>`, `Quick-reference fields`, and `Seed backlog — <token>` sections.
- **Scaffold / Refresh** — detect the language from the repo root (`go.mod`, `package.json`, `pyproject.toml`) and load the matching skill. For a `package.json` repo, narrow further: `svelte` or `vue` if a UI framework is present, `cdk` if `aws-cdk-lib` is, otherwise `typescript`. `typescript` and `python` carry no `Seed backlog` section — nothing to splice for those.

## Step 3 — Read the repo, don't ask for what's discoverable

**Scaffold / Refresh** — pull Quick-reference values from the repo itself: module/package name, declared port(s) (`docker-compose.yaml`, `main.go`/config), contract file (`openapi.yaml` if present), entrypoint path. Pull Commands the same way — `package.json` scripts, `Makefile`/`Taskfile` targets, `.claude/verify.sh` — and the Deploy row from whatever the repo's own deploy tooling is (`cdk deploy`, a `Makefile` target, a documented CLI). Only ask the user for a value that genuinely isn't in the repo.

**Create** — nothing to read yet. Ask for what Step 2's repo type can't supply: the repo name (if not derivable from the target path), its one-line purpose, and its port(s) — no cross-repo port convention exists to derive one from. Never ask for anything the repo type or language skill already fixes (layout, entrypoint path).

## Step 4 — Create: which repo types are supported

| Token | Language skill | Layout section | What gets written |
|---|---|---|---|
| `go-api` | `go` | `Repo layout — go-api` | Full skeleton — real entrypoint, build/deploy files |
| `vue-ui` | `vue` | `Repo layout — vue-ui` | Directory tree + manifest + empty starter page |
| `svelte-ui` | `svelte` | `Repo layout — svelte-ui` | Directory tree + manifest + empty starter page |
| `cdk-infra` | `cdk` | `Repo layout — cdk-infra` | Directory tree + manifest + structural `bin/app.ts`, zero stacks |

**No other token is supported.** `typescript` and `python` carry no `Repo layout` section in their skills — inventing one here would break the single-ownership rule, and inventing its *content* would mean guessing forge-SDK APIs from memory, which the global rules forbid outright. If the user asks for one of these, stop and name which skill needs a `## Repo layout — <token>` section added first (and, for a real entrypoint, a `reference.md` vertical slice like `go` has — see Step 5). Offer the doc-pair-only Scaffold path as a fallback if they want something written today.

## Step 5 — Create: materialize the tree

Walk every entry in the repo type's `Repo layout` tree:

- **A directory-only entry** (trailing `/`) → create it empty, no placeholder file inside.
- **A file with an established, language-agnostic shape** (`package.json`, `tsconfig.json`, `vite.config.ts`, `cdk.json`) → standard tool output for that file, filled from Step 3's values (package name, port). No Komodo-specific code involved, so no grounding risk.
- **`go-api` only** — `cmd/server/main.go` and `cmd/server/setup.go` follow the wiring shape in `go/reference.md`'s "The wiring" section, generalized to zero routes (an empty `http.ServeMux`, `/health` only) rather than the widget/thing example. `Dockerfile` and `docker-compose.yaml` follow standard container practice (multi-stage build, non-root user, a `/health` check, joining the shared external network). `.golangci.yaml` is copied verbatim from `templates/go/.golangci.yaml`. `openapi.yaml` gets a minimal stub — `/health` only; everything else grows with the API. `go.mod` gets the module path Step 3 established and the version floor `go` states.
- **`vue-ui` / `svelte-ui` / `cdk-infra`** — every file gets standard framework/tool defaults only, never an invented Komodo middleware, auth, or SDK call. `bin/app.ts` for `cdk-infra` follows the `cdk` skill's Stack layout structure (environment resolution, App construction, tagging) with zero stacks instantiated — the first stack is the user's next move, not this skill's.

## Step 6 — Write the documents

Load `docs` for the spec skeletons and `worklog` for the two record files.

- `CLAUDE.md` missing → write `templates/project/CLAUDE.md.tmpl` verbatim.
- `AGENTS.md` missing → fill `templates/project/AGENTS.md.tmpl` from Step 3's values. Its Commands table drops any row the repo has no equivalent for (e.g. no "Run locally" for a `cdk-infra` repo) rather than guessing. Its Documents table is built fresh each time from what's actually on disk — `docs/prd.md`, `docs/sdd.md`, `BACKLOG.md`, `CHANGELOG.md` each get a row only if that file exists; on Create all four already exist by the time this step runs, so all four appear.
- **Create only** → copy `templates/project/BACKLOG.md.tmpl` and `templates/project/CHANGELOG.md.tmpl`, then splice Step 2's `Seed backlog` stories into `Cross-Cutting`.
- **Create only** → write `docs/prd.md` and `docs/sdd.md` as full section skeletons from `docs`, every body `NEEDS DECISION`. Name them in the closing report as the user's next step; they gate everything downstream.
- **Any branch, when `BACKLOG.md` already exists** → append only the seed stories not already present, matched by text, under `Cross-Cutting`. Read the whole file first; touch nothing else.
- `AGENTS.md` exists → do not overwrite. Diff the proposed content against what's there, show the diff, and apply only on confirmation — "propose, don't impose", not a silent regeneration.

## Step 7 — Enforce the budget

Count the drafted `AGENTS.md`. **Over ~500 tokens is a stop condition**, not a warning: name which line pushes it over and whether that content belongs in `docs/` instead. Never ship an over-budget file to "fix later."

The ceiling is deliberately generous. This file is read on every turn in this repo and it replaces exploration that would otherwise cost tens of thousands of tokens — a complete Commands table earns its space many times over. What it may never hold is anything a skill already states by name.

## Out of scope

No cross-repo scanning — this skill only ever touches the one target repo in a single run. No `git init`, no first commit, no push — it writes files only; putting them under version control is the user's call, per the global git rule.
