# Changelog

Notable changes to komodo-agentic-config. Format follows Keep a Changelog; versions follow SemVer.

Append-only — a released section is never rewritten. Format rules live in the `standards-worklog` skill.

## [Unreleased]

## [0.1.0] — 2026-08-24

### Added
- `standards-docker` skill — base image pinning, distroless healthcheck pattern (a binary `healthcheck` subcommand, since a distroless runtime has no shell), `stop_grace_period` above the app's own drain timeout, `.dockerignore`, non-root, multi-stage layering. `generate-repo` Step 5's container-practice bullets now point at it.
- Canonical `standards-*` section order (root `AGENTS.md` §Skill contract), `templates/skills/standards.md.tmpl` to start a new one from, and a `scripts/validate.sh` check enforcing it across `standards-go`, `standards-typescript`, `standards-python`, `standards-c`, `standards-vue`, `standards-svelte`, `standards-cdk`.
- `Makefile` in every layout-bearing `standards-*` skill's `Repo layout` (go-api, go-mcp, vue-ui, svelte-ui, cdk-infra) — `verify` chains lint → typecheck/vet → test → build, and is what `context_injector.py` already looks for as a repo's merge gate. `templates/go/Makefile` and `templates/node/Makefile` ship the concrete targets.
- `Toolchain` sections for `standards-vue` and `standards-svelte` — previously absent, which left those repos' `standards-cicd`-mandated CI security scans with no command to run.
- `templates/go/.gitignore` and `templates/go/.dockerignore`, wired into `generate-repo` Step 5, added to `standards-go`'s two `Repo layout` trees.
- A `Tests:` seed story in every layout-bearing `standards-*` skill (go-api, go-mcp, vue-ui, svelte-ui, cdk-infra), and `test/` now materializes as `standards-sdlc`'s tier subtree on Create instead of an empty directory — closes the gap where a generated repo violated `generate-backlog`'s "every domain with behavior stories carries its own `Tests:` story" rule from the moment it was created.
- `generate-repo` Step 8 — a Create run now runs its own `make verify` and confirms every Step 2 seed story actually landed in `BACKLOG.md`, reporting either failure plainly instead of a closing report that assumes success.
- `workflow-loop` guardrails: a phase is complete only when its named skill actually ran (inherited on-disk state is not a pass); P1 treats a backlog holding only `generate-repo`'s seed stories as undecomposed; P2.1 runs its fork even when code already appears to exist; P2.0/`workflow-decompose` treat a transitively-`[BLOCKED]` task as one to route around, not a full loop halt; never poll a delegated phase with `ScheduleWakeup`; a P2.3 finding needing standards verification returns to a fork rather than being re-checked in the orchestrating window.
- `ways/sdlc.md` P2.3 now states `/code-review` is invoked with the task text and which `standards-*` skills apply, rather than left to choose its own review lenses.
- `workflow-loop` P1 trusts a language manifest already on disk (`go.mod`, `package.json`, `cdk.json`) as the zero-token signal that Create already ran, re-invoking `/generate-repo` only for an open Foundation-edge story or an explicit Scaffold/Refresh ask.

### Deviations from the original plan
- The scaffold-freshness signal landed as the manifest-file check above, not the originally proposed `.claude/scaffold.json` marker with a version-bumped "contract" integer — `generate-repo`'s existing Scaffold/Refresh drift-detection against the language skill's `Repo layout` tree already covers invalidation, so the extra state would have been unused machinery.
- `c` was documented as scaffold-only alongside `typescript`/`python` in `generate-repo`, rather than given a new `Repo layout` tree.
