---
name: generate-repo
description: Create a new repo's full skeleton, or scaffold/refresh its CLAUDE.md/AGENTS.md/TODO.md set.
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

### The doc pair and TODO.md are template-owned, not restated here

`templates/project/CLAUDE.md.tmpl`, `templates/project/AGENTS.md.tmpl`, and `templates/project/TODO.md.tmpl` are the one copy of each file's fixed shape. This skill fills their placeholders and writes the result — it never re-describes the shape in prose. If a placeholder needs to change, edit the `.tmpl` file, not this one.

`AGENTS.md.tmpl`'s Quick-reference table takes its **field set** from the matching language skill's **Quick-reference fields** section (`go`, `typescript`, `python`, `svelte`, `vue`, `cdk`) — never invented per repo. Its Deviations section is **refresh-only** (see Step 1); a new repo has no drift yet, so Create and Scaffold leave it out entirely.

### Single-ownership rule

**This skill references a layout or a field set. It never restates one.** The repo layout tree and the Quick-reference field set are owned by the language skill, which an agent already has loaded while writing code in that language — so there is exactly one copy and it cannot drift from what the agent is following. If a language skill lacks either section, add it there and reference it from here; never inline it into this file as a stopgap.

### No-duplication rule

A line only belongs in a generated `AGENTS.md` if it is a fact that `tech-stack`, `sdlc`, `cicd`, `coding-principles`, or the matching language skill cannot already produce by name. Concretely: this repo's actual port number belongs there; the meaning of "component test" does not. Restating global or language-skill content is not a convenience — it is the exact failure mode that let two live repos' `AGENTS.md` go stale after `home/` was restructured.

---

## Step 1 — Determine the branch

| Branch | Condition | Sections written |
|---|---|---|
| Create | Target path doesn't exist, or exists and is empty | Full skeleton + Header, Quick reference, Docs pointer + `TODO.md` |
| Scaffold | Target has code but no `AGENTS.md` | Header, Quick reference, Docs pointer |
| Refresh | `AGENTS.md` exists | All four, Deviations included |

**Only Create writes `TODO.md`**, copied verbatim from `templates/project/TODO.md.tmpl`. Scaffold and Refresh never touch it — an existing repo's backlog belongs to `backlog`, not this skill.

**Neither Create nor Scaffold writes a Deviations section.** Nothing has had a chance to drift yet, and an empty header is still a cost.

**Refresh diffs the repo's actual layout and conventions against the `tech-stack` skill's standard shape.** Only what differs becomes a Deviations line. Matching the standard produces zero lines, not a line saying so.

## Step 2 — Load the facts

- Load `tech-stack` for the standard repo shape (folders, port convention, entrypoint convention) on every branch.
- **Create** — the repo type token names the language skill directly (`go-api` → `go`, `vue-ui` → `vue`, `svelte-ui` → `svelte`, `cdk-infra` → `cdk`). Load it and read its `Repo layout — <token>` and `Quick-reference fields` sections.
- **Scaffold / Refresh** — detect the language from the repo root (`go.mod`, `package.json`, `pyproject.toml`) and load the matching skill. For a `package.json` repo, narrow further: `svelte` or `vue` if a UI framework is present, `cdk` if `aws-cdk-lib` is, otherwise `typescript`.

## Step 3 — Read the repo, don't ask for what's discoverable

**Scaffold / Refresh** — pull Quick-reference values from the repo itself: module/package name, declared port(s) (`docker-compose.yaml`, `main.go`/config), contract file (`openapi.yaml` if present), entrypoint path. Only ask the user for a value that genuinely isn't in the repo.

**Create** — nothing to read yet. Ask only for what Step 2's repo type can't supply: the repo name (if not derivable from the target path) and its one-line purpose. Never ask for anything the repo type or `tech-stack` already fixes (layout, port convention, entrypoint path).

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
- **`go-api` only** — `cmd/server/main.go` and `cmd/server/setup.go` follow the wiring shape in `go/reference.md`'s "The wiring" section, generalized to zero routes (an empty `http.ServeMux`, `/health` only) rather than the widget/thing example. `Dockerfile` and `docker-compose.yaml` follow `tech-stack`'s stated container rules (multi-stage non-root, `/health` check, external shared network, dual-port only if the repo type needs a private port). `.golangci.yaml` is copied verbatim from `templates/go/.golangci.yaml`. `openapi.yaml` gets a minimal stub — `/health` only; everything else grows with the API. `go.mod` gets the module path Step 3 established and the version floor `tech-stack`/`go` state.
- **`vue-ui` / `svelte-ui` / `cdk-infra`** — every file gets standard framework/tool defaults only, never an invented Komodo middleware, auth, or SDK call. `bin/app.ts` for `cdk-infra` follows the `cdk` skill's Stack layout structure (environment resolution, App construction, tagging) with zero stacks instantiated — the first stack is the user's next move, not this skill's.

## Step 6 — Write the doc pair (+ TODO.md on Create)

- `CLAUDE.md` missing → write `templates/project/CLAUDE.md.tmpl` verbatim.
- `AGENTS.md` missing → fill `templates/project/AGENTS.md.tmpl` from Step 3's values.
- **Create only** → also copy `templates/project/TODO.md.tmpl` verbatim, untouched.
- `AGENTS.md` exists → do not overwrite. Diff the proposed content against what's there, show the diff, and apply only on confirmation — this is a "propose, don't impose" action per the global rules, not a silent regeneration.

## Step 7 — Enforce the budget

Count the drafted `AGENTS.md`. Over ~300 tokens is a stop condition, not a warning: name which line is pushing it over and whether that content belongs in `docs/` instead. Never ship an over-budget file to "fix later."

## Out of scope

No cross-repo scanning — this skill only ever touches the one target repo in a single run. No `git init`, no first commit, no push — it writes files only; putting them under version control is the user's call, per the global git rule.
