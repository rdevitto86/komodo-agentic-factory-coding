# Changelog

Notable changes to komodo-agentic-toolkit-coding. Format follows Keep a Changelog; versions follow SemVer.

## [0.32.0] — 2026-08-27

### Changed
- `write-backlog`, `audit-backlog` merged into `backlog` (`plan`/`normalize`/`audit` modes over one `BACKLOG.md` format); frontmatter carries neither `disable-model-invocation` nor `user-invocable` so it stays callable by name from `workflow-loop`'s P1.
- 25 cross-referencing files (`AGENTS.md`, `README.md`, `claude-code/settings.json`, and 22 skills) repointed from `write-backlog`/`audit-backlog` to `backlog`/`backlog audit`.

### Removed
- `write-backlog`, `audit-backlog` — folded into `backlog`.

## [0.31.0] — 2026-08-26

### Added
- Local docs layout: `templates/project/docs/{spec,adr,runbook}/` starter files (`docs/spec/SDD.md`, `docs/spec/PRD.md`, `docs/adr/template.md`, `docs/runbook/template.md`) and `templates/project/mkdocs.yml.tmpl` (MkDocs Material theme); `write-repo`'s Create path now scaffolds all four into every new project repo.
- `sdd`, `prd` — merged authoring + audit skills for `docs/spec/SDD.md`/`docs/spec/PRD.md`, replacing `audit-sdd`/`audit-prd`.
- `adr` — authoring + audit skill for `docs/adr/`, greenfield (no prior generator existed).
- `runbook` — merged authoring + audit skill for `docs/runbook/`, replacing `write-runbook`.

### Changed
- `standards-specs` moved off its Drive-fetch contract to read `docs/spec/SDD.md` and `docs/spec/PRD.md` as local repo files, and documents MkDocs Material as the paired doc-site toolchain.
- Stale Drive/Google-Doc references repointed to the local `docs/spec/` contract across `workflow-implementer.md`, `write-repo`, `write-backlog`, `workflow-loop`, `audit-testing`, `workflow-consolidate`, `write-readme`, `AGENTS.md`, and `README.md`.
- `claude-code/settings.json`'s `skillOverrides` gained `sdd`/`prd`/`adr` as `name-only`, matching `runbook`/`write-readme`.
- `AGENTS.md` documents the new "authors and audits one local doc type" skill bucket.

### Removed
- `audit-sdd`, `audit-prd`, `write-runbook` — folded into `sdd`, `prd`, `runbook` respectively; all callers repointed.

## [0.30.0] — 2026-08-26

### Added
- `standards-security-api`, `standards-security-ui` — split out of `standards-security` for API and UI-surface security conventions respectively.
- `audit-prd`, `backlog-prioritize` — new typed-only skills; the former audits a fetched PRD, the latter reorders/re-prioritizes existing `BACKLOG.md` tasks without inventing new ones.
- `BACKLOG.md` and `templates/project/BACKLOG.md.tmpl` migrated to the `EPIC-XX` → `TG-XX.Y` → `TSK-XX.Y.Z` → `SUB-XX.Y.Z.N` hierarchy; `write-backlog`'s format section rewritten to match, replacing the old flat `T.D.S` numbering.

### Changed
- Skills renamed to bucket prefixes: `generate-backlog`→`write-backlog`, `generate-readme`→`write-readme`, `generate-runbook`→`write-runbook`, `git-create-issue`→`git-issue`, `git-create-pr`→`git-pr`; `standards-uiux`→`standards-design-ui`. Cross-references updated across `AGENTS.md`, `README.md`, `workflow-loop`, and the affected `standards-*`/`audit-*` skills.
- `standards-security` split into `standards-security-api` + `standards-security-ui`.

### Removed
- `generate-adr`, `generate-sdd` (+ its `authoring.md`), `generate-repo`, `standards-docs`, `standards-prd`, `workflow-debug-hypothesize` — superseded or unused.
- `standards-security` removed after its split into `standards-security-api`/`standards-security-ui`.

## [0.29.0] — 2026-08-26

### Added
- `standards-dotnet`, `standards-react`, `standards-java` — new domain skills following the standard `standards-<noun>` section order.
- `standards-uiux-security`, `standards-api-security` — security domain skills distinct from `standards-uiux`'s WCAG/Tailwind scope and `standards-security`'s general OWASP baseline.

### Changed
- `workflow-planner`'s queue cap raised from 12 to 20 tasks.

### Removed
- `standards-csharp`, `standards-gcp`, `standards-azure` parked as `SKILL.md.off` — not used yet, no listing cost, content preserved for when those domains land.

## [0.28.0] — 2026-08-26

### Added
- `rules-merge-conflicts` — autoloaded rule governing merge-conflict resolution: what to resolve unattended, what to escalate, what never gets silently dropped.
- `git-create-issue`, `git-create-pr` — split out of the prior combined git-issue-filing/git-PR-opening skills, each invocable by the agent or the human directly.
- `standards-git` — git branch/commit/push/merge/protected-ref domain knowledge, split out of `rules-source-control`.
- `standards-aws`, `standards-gcp`, `standards-azure`, `standards-csharp` — new domain skills following the standard `standards-<noun>` section order.

### Changed
- `rules-source-control` — trimmed to enforcement only; convention content moved to `standards-git`.
- Every reference to the retired `write-git-issue`/`write-pr` naming updated to `git-create-issue`/`git-create-pr` across `workflow-loop` and other skills that cite them.
- `claude-code/agents/implementer.md` → `workflow-implementer.md` and `claude-code/agents/planner.md` → `workflow-planner.md`, with all cross-references updated.
- `scripts/validate.sh`'s `LANGUAGE_SKILLS` and `claude-code/settings.json`'s `skillOverrides` extended to register the new skills.

## [0.27.0] — 2026-08-26

### Removed
- `.github/workflows/ci.yml` — CI ran `scripts/test-hooks.sh`/`scripts/validate.sh` on every push and PR; moved local instead.

### Added
- `scripts/hooks/git/pre-push-verify` — runs a repo's own `.claude/verify.sh` on push when one exists, no-op otherwise (same presence-gated pattern as `pre-push-golangci`). Picked up automatically by the existing `pre-push` dispatcher once `core.hooksPath` points at this directory.

## [0.26.0] — 2026-08-26

### Added
- `write-pr` skill (renamed from `generate-pr-description`) + `.github/PULL_REQUEST_TEMPLATE.md` — opens a PR directly via `gh pr create`/`gh pr edit`, filling title, body, and a single label (`bug`/`documentation`/`duplicate`/`enhancement`/`do not merge`/`skill`) from the real diff against the repo's own template.

### Changed
- `git_guard.py` + `settings.json` — replaced the blanket git-mutation deny with protected-ref enforcement; the agent may now branch, commit, push its own branch, sync its branch with its protected base via `git merge`, and open a pull request, still barred from `main`/`master`/`trunk`/`prod`/`production`/`release/*`/`hotfix/*` and from history rewrites and landing into a protected branch.
- `rules-source-control` — rewritten for the new capabilities, the protected-ref list, the `PUBLISH_ENABLED` kill switch, and `git merge`/`git pull --ff-only` recovery.
- `write-changelog` — tag creation is fully the user's now; dropped the agent tag-creation instruction.
- `config-accessibility-output` — added the fixed ✅/❌/⚠️ turn-end change summary schema.
- `workflow-loop` P3/P4 — P3 now commits the band here (`write-commit-message`, then `git commit`) once `workflow-consolidate`'s read-only fork returns; P4 is renamed Publish and pushes + runs `write-pr` instead of printing a commit message to paste.
- `workflow-complete` — rewritten to match: pushes the branch and runs `write-pr`, no longer generates or prints a commit message.

## [0.25.0] — 2026-08-26

### Added
- `write-git-issue` — files a finding as a GitHub issue via `gh issue create`, confirming with the user first; uses `--title-file`/`--body-file` to avoid shell injection.
- `audit-vulnerabilities` — CVE-focused sweep: Dependabot triage first, falling back to `govulncheck`/`npm audit`/`pip-audit` per the relevant `standards-*` skill.
- `audit-dependencies` — staleness/deprecation/EOL sweep, explicitly scoped clear of `audit-vulnerabilities`' CVE focus.
- `standards-shell` — shellcheck, `set -euo pipefail`, quoting; added to `scripts/validate.sh`'s `LANGUAGE_SKILLS` list.
- `standards-prd` — the PRD's read contract (section map, fetch rule, requirement-ID scheme). A PRD is now a Google Drive doc, read-only from this toolkit, never a repo file.
- `workflow-loop/ways/debugging.md` and `workflow-debug-hypothesize` (planner-backed fork) — a debugging way of working, dispatched from `workflow-loop/SKILL.md` for diagnostic-phrased tasks.
- `claude-code/settings.local.json.tmpl` and `claude-code/CLAUDE.local.md.tmpl` — personal-prefs overlay, copied to `~/.claude/` by `setup.sh` only if absent.
- `setup.sh` — resolves and reports the latest git tag as the installed version (falls back to short SHA, then "unknown").

### Changed
- **PRD moved out of the repo.** `generate-prd` and its `authoring.md` are removed; `docs/sdd.md` is now the sole frozen spec that lives in a repo and stands on its own (never blocked on a PRD existing). `write-sdd`/`authoring.md` rewritten so PRD requirement IDs are cited only when a PRD has been supplied as context, never invented. `AGENTS.md` gained a "PRD and Google Drive" section stating there is no fallback tier from SDD to PRD — three fetch points only (`workflow-loop` P0, `write-backlog`, `audit-sdd`), all in the main session, since no fork carries MCP tools to reach Drive.
- `claude-code/settings.json` split: personal prefs (`model`, `theme`, `effortLevel`, `modelSettings`, `tui`, `autoMemoryEnabled`, `autoCompactEnabled`, `remoteControlAtStartup`, `agentPushNotifEnabled`, `autoMode`, `env`) moved to `settings.local.json.tmpl`; `skillOverrides` gained the new skills above.
- `claude-code/AGENTS.md`'s single-user statement and full §2 ADHD-conversation section moved to `claude-code/CLAUDE.local.md.tmpl`, wired via a new `@CLAUDE.local.md` import in `claude-code/CLAUDE.md`.
- `bridges/komodo-bridge/.mcp.json.tmpl` — migrated from SSE (`type: sse`, `/sse`) to streamable HTTP (`type: http`, `/mcp`).
- `standards-typescript`, `standards-c` gained `## Repo layout` sections stating Create is unsupported, matching `standards-python`'s existing pattern.

### Fixed
- `git_guard.py` — dropped dead-code read-only allowances for `git branch`/`git stash`; `settings.json` already denies both at the permission layer before the hook runs. `scripts/test-hooks.sh` updated to match (127 passing).
- `scripts/validate.sh` — the hooks-check loop now includes `auto_format.py`, which was previously silently skipped.

## [0.24.2] — 2026-08-25

### Changed
- `git_guard.py` — `git tag` creation (lightweight and annotated) is now allowed for the agent; `-d`/`-D`/`--delete`/`-f`/`--force` stay denied. `settings.json` moved `Bash(git tag:*)` from deny to allow and added `Bash(bash setup.sh:*)`. `scripts/test-hooks.sh` grew G48–G51 covering the new split (125 → 129 passing). `write-changelog` and `audit-changelog` updated to stop describing `git tag` as hard-denied.

## [0.24.1] — 2026-08-25

### Changed
- Repo renamed `komodo-agentic-tools-code` → `komodo-agentic-toolkit-coding` on GitHub; local refs updated in `README.md`, `AGENTS.md`, `CHANGELOG.md`, `bridges/komodo-bridge/agent-roster.md`, the working directory itself, and the git remote.
- `write-commit-message` — secondary description switched from a comma/`+`-delimited flowing line to a `-`-prefixed bulleted list, one bullet per distinct concern.

## [0.24.0] — 2026-08-25

### Added
- `audit-changelog`, `audit-readme`, `audit-sdd`, `audit-testing` — four typed-only audit skills scoring `CHANGELOG.md`, `README.md`, `docs/sdd.md`, and the test suite against their respective generator/standards skills, findings filed to `BACKLOG.md` unless `--report` is passed.
- `write-changelog` — a "Tag sync" section: before appending a version heading, check `.git/refs/tags/`/`.git/packed-refs` for a matching tag and hand the user a ready-to-run `git tag` line if one's missing (tagging stays hard-denied to the agent).

### Changed
- `standards-cicd`, `standards-docs`, `standards-observability`, `standards-security`, `standards-worklog` marked `user-invocable: false` — autoloaded knowledge, not meant to be typed directly.
- `write-readme` switched from `disable-model-invocation: true` to `paths: "**/README.md"`, so it now loads automatically the instant README.md is touched instead of requiring the typed command.
- `claude-code/settings.json`'s `skillOverrides` gained `name-only` entries for the mid-loop phases, `write-repo`, `write-readme`, and the audit/commit-message command skills, matching `AGENTS.md`'s reachable-by-name list.
- `AGENTS.md`'s skill-contract section rewritten: bucket table and typed-only list gained the four new audits, and the context-budget section replaced its by-hand skill enumeration with the general "reached only by an explicit name is `name-only`" rule.

### Removed
- `CHANGELOG.md`'s stale 2026-08-24 backfill note and empty `## [Unreleased]` heading.

## [0.23.1] — 2026-08-24

### Added
- `audit-backlog` — a verdict rule for backlog lines that name no checkable file, command, or artifact (a `Done when` like "once X is scoped"): `git blame`/`git log -S '<line text>'` its introduction and check `CHANGELOG.md` for whether the thing it references was ever real. No commit ever built it → Stale, not Valid. Closes the gap where "nothing in the repo contradicts it" read as Valid for a dead placeholder indistinguishable from a live one.

## [0.23.0] — 2026-08-24

### Added
- `.github/workflows/ci.yml` — runs `scripts/test-hooks.sh` and `scripts/validate.sh` on push/PR, closing the gap where the hook and budget regressions only ran locally.
- `auto_format.py` — new `PostToolUse` hook on Edit/Write, running `gofmt`/`.go` and `prettier`/JS-TS-CSS-etc after every write; no-ops (exit 0) when the formatter isn't on `PATH`, matching the fail-open policy of `verify_gate.py`/`context_injector.py`. Registered in `settings.json`; `scripts/test-hooks.sh` grew F1–F7 (118 → 125 passing).
- `standards-python` — a "Repo layout" section noting `write-repo` Create is unsupported for Python (no `templates/python/` needed, matching the other unsupported languages).

### Fixed
- `comment_guard.py` — `handle_pre` no longer recomputes `scan_comments` a second time in `check_echoes`'s caller; the result is cached as `scope_scanned` and reused for both the removed-comment check and echo suppression.

## [0.22.1] — 2026-08-24

### Fixed
- `git_guard.py` — `cp`/`mv` now scan their destination against `is_code_path` (plain and `-t`/`--target-directory` forms, including a not-yet-existing directory target), closing the bypass where copying a staged file over a code path skipped `comment_guard`. `scripts/test-hooks.sh` grew from 98 to 118 cases (G31–G47).
- `git_guard.py` — recurses into `eval`/`time`/`command`/`xargs`/`nohup` wrappers with flag-aware value stripping, so `time git commit` and similar wrapped invocations no longer dodge the guard.
- `comment_guard.py` — `check_echoes` suppression now scopes against the specific edit's own `old_string`/`old_text` context instead of a file-wide comment union, so a genuine new echo-comment violation is no longer masked by identical text existing elsewhere in the file.

## [0.22.0] — 2026-08-24

### Added
- `standards-docker` skill — base image pinning, distroless healthcheck pattern (a binary `healthcheck` subcommand, since a distroless runtime has no shell), `stop_grace_period` above the app's own drain timeout, `.dockerignore`, non-root, multi-stage layering. `write-repo` Step 5's container-practice bullets now point at it.
- Canonical `standards-*` section order (root `AGENTS.md` §Skill contract), `templates/skills/standards.md.tmpl` to start a new one from, and a `scripts/validate.sh` check enforcing it across `standards-go`, `standards-typescript`, `standards-python`, `standards-c`, `standards-vue`, `standards-svelte`, `standards-cdk`.
- `Makefile` in every layout-bearing `standards-*` skill's `Repo layout` (go-api, go-mcp, vue-ui, svelte-ui, cdk-infra) — `verify` chains lint → typecheck/vet → test → build, and is what `context_injector.py` already looks for as a repo's merge gate. `templates/go/Makefile` and `templates/node/Makefile` ship the concrete targets.
- `Toolchain` sections for `standards-vue` and `standards-svelte` — previously absent, which left those repos' `standards-cicd`-mandated CI security scans with no command to run.
- `templates/go/.gitignore` and `templates/go/.dockerignore`, wired into `write-repo` Step 5, added to `standards-go`'s two `Repo layout` trees.
- A `Tests:` seed story in every layout-bearing `standards-*` skill (go-api, go-mcp, vue-ui, svelte-ui, cdk-infra), and `test/` now materializes as `standards-sdlc`'s tier subtree on Create instead of an empty directory — closes the gap where a generated repo violated `write-backlog`'s "every domain with behavior stories carries its own `Tests:` story" rule from the moment it was created.
- `write-repo` Step 8 — a Create run now runs its own `make verify` and confirms every Step 2 seed story actually landed in `BACKLOG.md`, reporting either failure plainly instead of a closing report that assumes success.
- `workflow-loop` guardrails: a phase is complete only when its named skill actually ran (inherited on-disk state is not a pass); P1 treats a backlog holding only `write-repo`'s seed stories as undecomposed; P2.1 runs its fork even when code already appears to exist; P2.0/`workflow-decompose` treat a transitively-`[BLOCKED]` task as one to route around, not a full loop halt; never poll a delegated phase with `ScheduleWakeup`; a P2.3 finding needing standards verification returns to a fork rather than being re-checked in the orchestrating window.
- `ways/sdlc.md` P2.3 now states `/code-review` is invoked with the task text and which `standards-*` skills apply, rather than left to choose its own review lenses.
- `workflow-loop` P1 trusts a language manifest already on disk (`go.mod`, `package.json`, `cdk.json`) as the zero-token signal that Create already ran, re-invoking `/write-repo` only for an open Foundation-edge story or an explicit Scaffold/Refresh ask.

### Deviations from the original plan
- The scaffold-freshness signal landed as the manifest-file check above, not the originally proposed `.claude/scaffold.json` marker with a version-bumped "contract" integer — `write-repo`'s existing Scaffold/Refresh drift-detection against the language skill's `Repo layout` tree already covers invalidation, so the extra state would have been unused machinery.
- `c` was documented as scaffold-only alongside `typescript`/`python` in `write-repo`, rather than given a new `Repo layout` tree.

## [0.21.2] — 2026-08-24

### Changed
- Skills renamed to bucket prefixes (`standards-go`, `standards-python`, `standards-sdlc`, `standards-security`, `standards-svelte`, `standards-typescript`, `standards-uiux`, `standards-vue`, `workflow-*`); `worklog` split into `standards-worklog` so records and specs stop sharing one skill.

## [0.21.1] — 2026-08-23

### Changed
- `home/` renamed to `claude-code/`; the business-domain agent (`home/agents/business.md`) and the `decompose` skill removed — final narrowing of scope to software/hardware engineering only.

## [0.21.0] — 2026-08-21

### Added
- `home/skills/lifecycle/ways/sdlc.md`, `home/skills/worklog/SKILL.md`, `home/skills/risk-assessment/SKILL.md`, `templates/project/{BACKLOG,CHANGELOG}.md.tmpl`.

### Changed
- `scripts/doctor.sh` renamed to `scripts/validate.sh` and expanded; `scripts/test-hooks.sh` grew substantially.

### Removed
- `home/skills/wrap-up/SKILL.md`, `home/skills/tech-stack/SKILL.md`, the standalone QA agent file.

## [0.20.0] — 2026-08-18

### Added
- `home/skills/readme/SKILL.md`; `write-repo` gained a Create branch.

### Changed
- `backlog` skill gained a normalize mode; `[WIP]` tags replaced checkbox-style TODO items.

## [0.19.0] — 2026-08-10

### Added
- `comment_guard.py` positional-slot model (step marker, banner, structured note, script manual, machine directive) replacing free-text heuristics; `paths:`-based skill auto-activation.

### Removed
- `scripts/hooks/git/pre-commit-comments` — superseded by `comment_guard.py`.

## [0.18.0] — 2026-08-10

### Added
- `scripts/hooks/git/{pre-commit,pre-commit-gofmt,pre-push,pre-push-golangci,install.sh}` — installable git hook scripts; `templates/go/.golangci.yaml`; `home/skills/tech-stack/SKILL.md`, `typescript/testing.md`, `uiux/SKILL.md`.

### Removed
- `home/skills/{sql,stack,tailwind,tax,terraform,todo,wcag}/SKILL.md` folded away or superseded.

## [0.17.0] — 2026-08-08

### Changed
- Config restructured into `home/` as a literal mirror of `~/.claude/`; `comment_guard` rewritten from shell (`no-comments-guard.sh`) to Python (`home/hooks/comment_guard.py`); `go`/`python`/`svelte`/`typescript`/`vue` unified into `home/skills/*/SKILL.md`.

### Added
- `templates/project/AGENTS.md.tmpl`, `templates/project/CLAUDE.md.tmpl`, `templates/project/TODO.md.tmpl`, `scripts/test-hooks.sh`.

## [0.16.0] — 2026-07-27

### Added
- Senior-engineering doctrine + decomposition rule in `standards/principles.md`.

### Fixed
- `no-comments-guard.sh` no longer blocked a restored (previously deleted) comment.

### Changed
- `standards/testing.md` tier model rewritten.

## [0.15.0] — 2026-07-20

### Added
- `standards/assumptions.md` — when to assume vs. ask.

### Changed
- `CLAUDE.md`/`advisor`/`software-engineer` agents trimmed for context bloat.

## [0.14.0] — 2026-07-20

### Added
- `standards/communication.md` — ADHD-calibrated output rules, pulled into every agent's context.

## [0.13.0] — 2026-07-17

### Added
- `docs/orchestration.md` design spec (tier/profile/mode taxonomy, duty classes, model tier map), `audit`/`story` skills, `komodo-bridge/registry.generated.json`, `scripts/gen-bridge-registry.sh`, `scripts/set-runtime.sh`, `scripts/doctor.sh`.

### Changed
- Tests moved out of colocation into a per-tier `test/` tree (component/integration/e2e/chaos/perf).

## [0.12.0] — 2026-07-06

### Fixed
- `git-guard.sh` hard-blocked all `git push`/`git merge`, not just force-push — closed a bypass.

### Added
- `standards/findings.md` — mandatory confidence/source/why fields on every reported finding, to cut audit noise.

## [0.11.0] — 2026-06-26

### Added
- `profile/AGENTS.md` / `profile/CLAUDE.md` — universal cross-model directive split out from Claude-specific config; `standards/testing.md` (environment/tier matrix), `templates/Justfile.tmpl`.

### Changed
- `advisor` agent role rewritten as orchestrator with a defined cross-review gate.

## [0.10.0] — 2026-06-12

### Changed
- Repo flattened: dropped the `claude/` prefix, top-level `agents/`/`standards/`/`templates/`; hooks ported to `platforms/claude/hooks/*.sh` so the config is no longer Claude-only in structure.

### Added
- Comment-rule regression fixtures + `scripts/test-comment-rules.sh`, `scripts/validate-bridge-roster.sh`.

## [0.9.0] — 2026-06-09

### Added
- `swe/changelog.md` — first CHANGELOG/version-bump standard in the repo's own history.

### Changed
- `swe/git-flow.md` commit format redefined as short `+`-joined types; agent directives tidied for token usage; worktree isolation banned in `principles.md`.

## [0.8.0] — 2026-06-06

### Added
- `cyber-security`, `data-analyst`, `lawyer`, `machinist`, `marketing`, `tax-advisor` agents (each with an `email.md` companion where applicable), `project-manager/memory.md`.
- `.gitignore`, `scripts/validate-refs.sh`.

### Fixed
- `MEMORY.md` accidentally committed to the repo — removed and gitignored.

## [0.7.0] — 2026-06-06

### Changed
- Standalone `claude/standards/*.md` files folded into per-agent `claude/agents/swe/<domain>/` subtrees (`go/coding.md`, `python/coding.md`, `svelte/coding.md`, `ts/coding.md`, `db/sql.md`, etc.) — replaced a flat standards library with domain-scoped agent modes.

### Added
- `mechatronics/cpp/coding.md`, `swe/api/audit.md`, `swe/api/blueprint.md`, `swe/design/design.md`, `project-manager` docs.
- `_testing-go.md`/`_testing-ts.md` retired in favor of expanded `go/coding.md`/`ts/coding.md`.

## [0.6.0] — 2026-05-28

### Added
- Go service scaffold templates (`Dockerfile.tmpl`, `client.go.tmpl`, `main-fargate.go.tmpl`, etc.) under `claude/skills/templates/service/`.
- `standards/docker.md`, `standards/observability.md`, `standards/svelte.md`, `standards/komodo-context.md`.

### Changed
- `standards/comments.md` and `standards/testing-go.md` substantially rewritten; `testing.md` renamed to `testing-ts.md` to pair with the new Go-specific standard.

## [0.5.0] — 2026-05-18

### Added
- `advisor` agent, `swe-test` agent, `standards/testing.md`, `standards/token-efficiency.md`, `standards/comments.md`, `standards/todo.md`, `standards/principles.md`, `standards/python.md`, `standards/testing-go.md`.

## [0.4.0] — 2026-04-08

### Removed
- `customer-servicing`, `lawyer`, `marketing`, `sales`, most of `project-manager`, `quality-assurance`, `robotics` agents and their skills — first pass at narrowing scope off the original multi-industry roster.

### Added
- `swe-embedded` agent.

### Changed
- `CLAUDE.md` reworked to drop MCP-agent references now that the roster is trimmed.

## [0.3.1] — 2026-03-31

### Added
- Brief TODO-tracking and SDK-usage notes appended to `project-manager`/`swe` agent files.

## [0.3.0] — 2026-03-28

### Added
- `claude/standards/` — `api-design.md`, `go.md`, `logging.md`, `pull-requests.md`, `security.md`, `sql.md`, `typescript.md`, plus a standards `README.md` index.
- `git-flow.md` skill, `project-management/trello.md` skill.

## [0.2.0] — 2026-03-24

### Added
- Skills spanning agriculture, customer service, marketing, sales, project workflows, and engineering subfields (electrical, hardware BOM, mechanical, robotics ROS, API middleware, DB migration, Terraform) — the repo's original scope was cross-industry, not software/hardware-only.

### Changed
- Software UI skills (`add-route`, `new-component`, `new-page`, `new-service`) relocated under `engineering/software/ui/`.

## [0.1.0] — 2026-03-24

### Added
- `setup.sh` symlink installer, `claude/settings.json`, first skill set (`add-route`, `new-component`, `new-page`, `new-service`) under `claude/skills/`.
