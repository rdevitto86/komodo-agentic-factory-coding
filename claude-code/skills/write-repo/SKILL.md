---
name: write-repo
description: Create a new repo's full skeleton, or scaffold/refresh its CLAUDE.md/AGENTS.md and the three standard local documents.
argument-hint: [repo-type: go-api|go-mcp|vue-ui|svelte-ui|cdk-infra] [target path, defaults to cwd]
context: fork
agent: workflow-implementer
background: false
---

# Generate Repo

Target: **$ARGUMENTS** — first token is the repo type (`go-api`, `go-mcp`, `vue-ui`, `svelte-ui`, `cdk-infra`), second is the target path (defaults to cwd). Repo type is only needed when the target has no code yet; an existing repo's language is detected from what's on disk, as before.

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
| `README.md` | The `write-readme` skill's fixed template |

The SDD (and, when one exists, the PRD) is a Google Doc in Drive — never a repo file, never scaffolded here. `standards-specs` owns the read-only fetch contract.

**The split is not arbitrary.** A `.tmpl` exists where the file ships with *seeded content* — closeout stories, an initial version heading.

This skill fills placeholders and writes the result — it never re-describes a shape in prose. If a placeholder changes, edit the `.tmpl`; if a seed story changes, edit the language skill's `Seed backlog` section.

`AGENTS.md.tmpl`'s Quick-reference table takes its **field set** from the matching language skill's **Quick-reference fields** section (`standards-go`, `standards-typescript`, `standards-python`, `standards-svelte`, `standards-vue`, `standards-cdk`) — never invented per repo. Its Deviations section is **refresh-only** (see Step 1); a new repo has no drift yet, so Create and Scaffold leave it out entirely.

### Single-ownership rule

**This skill references a layout, a field set, or a seed story. It never restates one.** The repo layout tree, the Quick-reference field set, and the `Seed backlog` stories are owned by the language skill, which an agent already has loaded while writing code in that language — so there is exactly one copy and it cannot drift from what the agent is following. If a language skill lacks any of the three sections, add it there and reference it from here; never inline it into this file as a stopgap.

### No-duplication rule

A line only belongs in a generated `AGENTS.md` if it is a fact that `standards-sdlc`, `standards-cicd`, or the matching language skill cannot already produce by name. Concretely: this repo's actual port number belongs there; the meaning of "component test" does not. Restating global or language-skill content is not a convenience — it is the exact failure mode that let two live repos' `AGENTS.md` go stale after `claude-code/` was restructured.

---

## Step 1 — Determine the branch

| Branch | Condition | What it writes |
|---|---|---|
| Create | Target path missing or empty | Full skeleton, `AGENTS.md`, all three local documents |
| Scaffold | Target has code but no `AGENTS.md` | `AGENTS.md`; `README.md` if missing; appends seed stories to an existing `BACKLOG.md` |
| Refresh | `AGENTS.md` exists | All sections, Deviations included; `README.md` scaffolded or refreshed; appends seed stories |

**Only Create writes the three documents from scratch.** `BACKLOG.md` and `CHANGELOG.md` come from their `.tmpl`, with the repo type's `Seed backlog` stories spliced into `Cross-Cutting`. `README.md` is written by the `write-readme` skill from Step 3's facts — the one document of the three with real content on day one, since it describes what already exists rather than what's planned.

**Never write the SDD.** It's a Drive doc, out of scope for this skill entirely — name it in the closing report as something the user maintains outside this toolkit, not something this skill scaffolds.

**Scaffold and Refresh never create or restructure `BACKLOG.md`** — its domains and stories stay `write-backlog`'s territory. When one already exists, the only touch either branch makes is appending seed stories not already present under `Cross-Cutting`, matched by text. Nothing else is read, reordered, or rewritten.

**Neither Create nor Scaffold writes a Deviations section.** Nothing has had a chance to drift yet, and an empty header is still a cost.

**Refresh diffs the repo's actual layout and conventions against the language skill's `Repo layout — <token>` tree, and its actual practice against `standards-sdlc` (coverage floors, test tiers), `standards-cicd` (pipeline stages), and `standards-security-api`/`standards-security-ui` wherever the repo visibly departs from one of them.** Only what differs becomes a Deviations line, each tagged with the skill it departs from. Matching it produces zero lines, not a line saying so.

## Step 2 — Load the facts

- **Create** — the repo type token names the language skill directly (`go-api` → `standards-go`, `go-mcp` → `standards-go`, `vue-ui` → `standards-vue`, `svelte-ui` → `standards-svelte`, `cdk-infra` → `standards-cdk`). Load it and read its `Repo layout — <token>`, `Quick-reference fields`, and `Seed backlog — <token>` sections.
- **Scaffold / Refresh** — detect the language from the repo root (`go.mod`, `package.json`, `pyproject.toml`) and load the matching skill. For a `package.json` repo, narrow further: `standards-svelte` or `standards-vue` if a UI framework is present, `standards-cdk` if `aws-cdk-lib` is, otherwise `standards-typescript`. `standards-typescript` and `standards-python` carry no `Seed backlog` section — nothing to splice for those.

## Step 3 — Read the repo, don't ask for what's discoverable

**Scaffold / Refresh** — pull Quick-reference values from the repo itself: module/package name, declared port(s) (`docker-compose.yaml`, `main.go`/config), contract file (`openapi.yaml` if present), entrypoint path. Pull Commands the same way — `package.json` scripts, `Makefile`/`Taskfile` targets, `.claude/verify.sh` — and the Deploy row from whatever the repo's own deploy tooling is (`cdk deploy`, a `Makefile` target, a documented CLI). Only ask the user for a value that genuinely isn't in the repo.

**Create** — nothing to read yet. Ask for what Step 2's repo type can't supply: the repo name (if not derivable from the target path), its one-line purpose, and its port(s) — no cross-repo port convention exists to derive one from. Never ask for anything the repo type or language skill already fixes (layout, entrypoint path).

**When invoked by name from another skill or a `BACKLOG.md` story rather than typed directly** — this runs forked (`workflow-implementer`, per this file's frontmatter), and a fork cannot ask. The invocation must supply repo name, purpose, and port(s) up front, the same way `workflow-implement` requires its `Done when` command explicit because "the fork cannot see the queue."

## Step 4 — Create: which repo types are supported

| Token | Language skill | Layout section | What gets written |
|---|---|---|---|
| `go-api` | `standards-go` | `Repo layout — go-api` | Full skeleton — real entrypoint, build/deploy files |
| `go-mcp` | `standards-go` | `Repo layout — go-mcp` | Full skeleton — MCP/Streamable HTTP entrypoint, build/deploy files, zero tools registered |
| `vue-ui` | `standards-vue` | `Repo layout — vue-ui` | Directory tree + manifest + empty starter page |
| `svelte-ui` | `standards-svelte` | `Repo layout — svelte-ui` | Directory tree + manifest + empty starter page |
| `cdk-infra` | `standards-cdk` | `Repo layout — cdk-infra` | Directory tree + manifest + structural `bin/app.ts`, zero stacks |

**No other token is supported.** `standards-typescript`, `standards-python`, and `standards-c` carry no `Repo layout` section in their skills — inventing one here would break the single-ownership rule, and inventing its *content* would mean guessing forge-SDK APIs from memory, which the global rules forbid outright. If the user asks for one of these, stop and name which skill needs a `## Repo layout — <token>` section added first (and, for a real entrypoint, a `reference.md` vertical slice like `standards-go` has — see Step 5). Offer the doc-pair-only Scaffold path as a fallback if they want something written today.

## Step 5 — Create: materialize the tree

Walk every entry in the repo type's `Repo layout` tree:

- **A directory-only entry** (trailing `/`) → create it empty, no placeholder file inside — **except `test/`**, which materializes as `standards-sdlc`'s tier subtree (`component/ contract/ integration/ e2e/ smoke/ perf/ chaos/`), each created empty. Unit tests colocate beside source per the language skill and never live under `test/`.
- **A file with an established, language-agnostic shape** (`package.json`, `tsconfig.json`, `vite.config.ts`, `cdk.json`) → standard tool output for that file, filled from Step 3's values (package name, port). No Komodo-specific code involved, so no grounding risk. For `vue-ui`/`svelte-ui`/`cdk-infra`, `package.json`'s `scripts` carries `lint`, `typecheck`, `test`, `build` — the targets `templates/node/Makefile` delegates to.
- **`Makefile`** (`go-api`/`go-mcp`/`vue-ui`/`svelte-ui`/`cdk-infra`, all) → copied verbatim from `templates/go/Makefile` (Go repo types) or `templates/node/Makefile` (Node repo types). This is the `verify` target `context_injector.py` discovers — never invent a different shape per repo.
- **`.gitignore`/`.dockerignore`** (Go repo types) → copied verbatim from `templates/go/.gitignore` / `templates/go/.dockerignore`, with `{{MODULE_BINARY_NAME}}` filled from the module path Step 3 established.
- **`go-api` only** — `cmd/server/main.go` and `cmd/server/setup.go` follow the wiring shape in `standards-go/reference.md`'s "The wiring" section, generalized to zero routes (an empty `http.ServeMux`, `/health` only) rather than the widget/thing example. `Dockerfile` and `docker-compose.yaml` follow `standards-docker` (multi-stage, pinned exact base tag, distroless runtime with a binary `healthcheck` subcommand, `stop_grace_period` above the app's own drain timeout, joining the shared external network). `.golangci.yaml` is copied verbatim from `templates/go/.golangci.yaml`. `openapi.yaml` gets a minimal stub — `/health` only; everything else grows with the API. `go.mod` gets the module path Step 3 established and the version floor `standards-go` states.
- **`go-mcp` only** — same `Dockerfile`/`docker-compose.yaml`/`.golangci.yaml`/`go.mod` treatment as `go-api`. `cmd/server/main.go` wires an MCP endpoint over Streamable HTTP (the current MCP transport spec; stdio only if the target repo's own SDD says otherwise) with zero tools registered, `/health` only — **no vetted Go MCP SDK is recorded anywhere in this org yet**, so stop and ask which library/import path to use rather than guessing one from memory; this is exactly the capability-gap rule in `AGENTS.md` §1, and the run stops with the tree unwritten rather than emitting a plain-`net/http` server that merely looks like an MCP skeleton. Once one is confirmed, record it in `standards-go/reference.md` so this is a one-time question, not a recurring one. `tools.md` (the contract file, `go-mcp`'s analog to `openapi.yaml`) gets a minimal stub — `/health` only; every real tool grows the file as it's registered.
- **`vue-ui` / `svelte-ui` / `cdk-infra`** — every file gets standard framework/tool defaults only, never an invented Komodo middleware, auth, or SDK call. `bin/app.ts` for `cdk-infra` follows the `standards-cdk` skill's Stack layout structure (environment resolution, App construction, tagging) with zero stacks instantiated — the first stack is the user's next move, not this skill's.

## Step 6 — Write the documents

Load `write-changelog` for `CHANGELOG.md` and `write-readme` for `README.md`. Splicing seed stories into `BACKLOG.md` needs only the story line shape — `- T.D.S | SEV | [WIP] <text> · <size> → \`<done when>\`` — the full ruleset lives in `write-backlog`, not needed for a splice. After splicing, renumber `Cross-Cutting`'s `T.D`/`T.D.S` tags so the spliced stories stay contiguous with what was already there — same rule `write-backlog` states for its own merge step.

- `CLAUDE.md` missing → write `templates/project/CLAUDE.md.tmpl` verbatim.
- `AGENTS.md` missing → fill `templates/project/AGENTS.md.tmpl` from Step 3's values. Its Commands table drops any row the repo has no equivalent for (e.g. no "Run locally" for a `cdk-infra` repo) rather than guessing. Its Documents table is built fresh each time from what's actually on disk — `README.md`, `BACKLOG.md`, `CHANGELOG.md` each get a row only if that file exists; on Create all three already exist by the time this step runs, so all three appear.
- **Create only** → copy `templates/project/BACKLOG.md.tmpl` and `templates/project/CHANGELOG.md.tmpl`, then splice Step 2's `Seed backlog` stories into `Cross-Cutting`.
- **Create only** → copy `templates/project/docs/spec/`, `templates/project/docs/adr/`, and `templates/project/docs/runbook/` verbatim into the target's `docs/`, and copy `templates/project/mkdocs.yml.tmpl` to `mkdocs.yml` with `{{NAME}}` filled from Step 3's repo name.
- **Create only** → name drafting the SDD in Drive as the user's next step in the closing report; it gates everything downstream, but this skill never writes it.
- **`README.md` missing (any branch)** → hand off to `write-readme`'s own Scaffold path using Step 3's facts. On Create, this runs after the tree is materialized (Step 5) so there's an entrypoint and commands to describe, not an empty skeleton.
- **`README.md` exists (Scaffold / Refresh)** → hand off to `write-readme`'s own Refresh path — it diffs and confirms itself; this skill doesn't duplicate that logic.
- **Any branch, when `BACKLOG.md` already exists** → append only the seed stories not already present, matched by text, under `Cross-Cutting`. Read the whole file first; touch nothing else.
- `AGENTS.md` exists → do not overwrite. Diff the proposed content against what's there, show the diff, and apply only on confirmation — "propose, don't impose", not a silent regeneration.

## Step 7 — Enforce the budget

Count the drafted `AGENTS.md`. **Over ~500 tokens is a stop condition**, not a warning: name which line pushes it over and whether that content belongs in `docs/` instead. Never ship an over-budget file to "fix later."

The ceiling is deliberately generous. This file is read on every turn in this repo and it replaces exploration that would otherwise cost tens of thousands of tokens — a complete Commands table earns its space many times over. What it may never hold is anything a skill already states by name.

## Step 8 — Verify the output, Create only

**A Create run that cannot build reports as incomplete, not done.** Once the tree is materialized and `Makefile` exists (Step 5), run its `verify` target and paste what it returned — the same standard `workflow-loop`'s P2.2 holds everything else to. A failure here is the run's own result, stated plainly, never silently dropped from the closing report.

**Confirm every Step 2 seed story actually landed** in `BACKLOG.md`'s `Cross-Cutting` domain — read the file back and diff against what the language skill's `Seed backlog` section named. A story that silently failed to splice is a defect in this run, not a future surprise for whoever reads the backlog next.

## Out of scope

No cross-repo scanning — this skill only ever touches the one target repo in a single run. No `git init`, no first commit, no push — it writes files only; putting them under version control is the user's call, per `rules-source-control`.
