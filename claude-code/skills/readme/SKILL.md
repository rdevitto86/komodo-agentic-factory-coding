---
name: readme
description: Generate or refresh a repo's README.md against a fixed, concise template.
argument-hint: [target repo path, defaults to cwd]
disable-model-invocation: true
---

# README

Target: **$ARGUMENTS** (defaults to the current working directory).

One fixed skeleton, applicable whether the repo is an API, a UI, a background service, a job runner, an SDK/library, or hardware firmware. Repo-kind detail lives in exactly one section (Usage); everything else is shared.

`# <repo-name>` as the H1, then five numbered `##` sections: **1. Overview · 2. Setup · 3. Usage · 4. Testing · 5. References.**

**Concise by default.** A README is an entry point, not the documentation. Depth belongs in `docs/prd.md` / `docs/sdd.md` (see the `docs` skill) — this file links to them rather than restating them. Target a few screens, not hundreds of lines; tables and links over prose.

---

## Step 1 — Load the facts

- Detect the language from the repo root (`go.mod`, `package.json`, `pyproject.toml`) and load the matching language skill for its layout and Quick-reference fields (port, entrypoint, build/run/test commands).
- If `docs/prd.md` / `docs/sdd.md` exist, load `docs` and pull from them — never invent what they don't state.
- If `BACKLOG.md` exists, check for open Blocker-tier items.

## Step 2 — Read the repo, don't invent

Every fact in the README must trace to something actually in the repo: a Makefile target, a `package.json` script, a config/env read, a route/handler file, a route file, an export list. A value with no source is a gap to flag to the user, never a filled-in guess.

## Step 3 — Fixed sections, in order

- **`# <repo-name>`** — H1 title, nothing else on that line.
- **1. Overview** — one paragraph on what it is. If there's a common misreading of its role (e.g. it looks like it does X but actually only does Y), one line stating what it deliberately is *not*. If `BACKLOG.md` has an open Blocker, one status line here too, pointing at `BACKLOG.md` — omit entirely on a clean repo.
- **2. Setup** — install and run, in copy-pasteable commands, sourced from the actual build tooling. Then env vars / config keys the repo actually reads, table form (name, required, description) — omit the table if the repo has none.
- **3. Usage** — how to actually use the thing once it's running, sourced from the code. Shape varies by repo kind, pick whichever apply:
  - API service → routes table (method, path, one-line description) + runnable `curl` examples for the primary flows
  - SDK / library → package or export table (one line per public package/module) + a code snippet per major package showing the call shape
  - UI → screens or top-level components + how to reach them locally
  - Job / worker → job or schedule table (trigger, cadence, what it does)
  - Hardware → interface/pinout table + how to drive it
  A repo can have more than one facet (e.g. a service that's also a library) — combine only when the repo genuinely has both, never speculatively.
- **4. Testing** — test tiers and the commands that run them, sourced from the Makefile/scripts, not invented tier names.
- **5. References** — pointer table to what exists: `docs/prd.md`, `docs/sdd.md`, `BACKLOG.md`, `CHANGELOG.md`, `openapi.yaml` or equivalent contract file. List only files present in this repo. **Never list individual `docs/adrs/` files** — the SDD §14 References section already links them; the README points at the SDD, not around it.

No section beyond these five. Deep design rationale, infra diagrams, and endpoint-by-endpoint request/response detail belong in `docs/sdd.md`, linked from §5 — not inlined here.

## Step 4 — Scaffold vs refresh

| Branch | Condition | Behavior |
|---|---|---|
| Scaffold | No `README.md` | Write all applicable sections fresh |
| Refresh | `README.md` exists | Diff proposed content against what's there section by section |

**Refresh never overwrites silently.** Show the diff, apply only on confirmation — same propose-don't-impose rule as `generate-repo`. A section already present and accurate is left untouched, not rewritten to match this template's wording.

## Step 5 — Flag, don't fabricate

Any section with no source data (no `docs/prd.md` for the purpose paragraph, no discoverable config for env vars) is flagged to the user as a gap, not filled with a plausible-sounding placeholder.
