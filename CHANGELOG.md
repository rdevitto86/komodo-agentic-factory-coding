# Changelog

Notable changes to komodo-agentic-toolkit-coding. Format follows Keep a Changelog; versions follow SemVer.

## [0.25.0] — 2026-08-25

### Added
- `generate-pr` skill (renamed from `generate-pr-description`) + `.github/PULL_REQUEST_TEMPLATE.md` — opens a PR directly via `gh pr create`/`gh pr edit`, filling title, body, and a single label (`bug`/`documentation`/`duplicate`/`enhancement`/`do not merge`/`skill`) from the real diff against the repo's own template.

### Changed
- `git_guard.py` + `settings.json` — replaced the blanket git-mutation deny with protected-ref enforcement; the agent may now branch, commit, push its own branch, and open a pull request, still barred from `main`/`master`/`trunk`/`prod`/`production`/`release/*`/`hotfix/*` and from history rewrites and merges.
- `rules-source-control` — rewritten for the new capabilities, the protected-ref list, the `PUBLISH_ENABLED` kill switch, and `git merge --ff-only`/`git pull --ff-only` recovery.
- `generate-changelog` — tag creation is fully the user's now; dropped the agent tag-creation instruction.
- `config-accessibility-output` — added the fixed ✅/❌/⚠️ turn-end change summary schema.
- `workflow-loop` P3/P4 — P3 now commits the band here (`generate-commit-message`, then `git commit`) once `workflow-consolidate`'s read-only fork returns; P4 is renamed Publish and pushes + runs `generate-pr` instead of printing a commit message to paste.
- `workflow-complete` — rewritten to match: pushes the branch and runs `generate-pr`, no longer generates or prints a commit message.

## [0.24.2] — 2026-08-25

### Changed
- `git_guard.py` — `git tag` creation (lightweight and annotated) is now allowed for the agent; `-d`/`-D`/`--delete`/`-f`/`--force` stay denied. `settings.json` moved `Bash(git tag:*)` from deny to allow and added `Bash(bash setup.sh:*)`. `scripts/test-hooks.sh` grew G48–G51 covering the new split (125 → 129 passing). `generate-changelog` and `audit-changelog` updated to stop describing `git tag` as hard-denied.

## [0.24.1] — 2026-08-25

### Changed
- Repo renamed `komodo-agentic-tools-code` → `komodo-agentic-toolkit-coding` on GitHub; local refs updated in `README.md`, `AGENTS.md`, `CHANGELOG.md`, `bridges/komodo-bridge/agent-roster.md`, the working directory itself, and the git remote.
- `generate-commit-message` — secondary description switched from a comma/`+`-delimited flowing line to a `-`-prefixed bulleted list, one bullet per distinct concern.

## [0.24.0] — 2026-08-25

### Added
- `audit-changelog`, `audit-readme`, `audit-sdd`, `audit-testing` — four typed-only audit skills scoring `CHANGELOG.md`, `README.md`, `docs/sdd.md`, and the test suite against their respective generator/standards skills, findings filed to `BACKLOG.md` unless `--report` is passed.
- `generate-changelog` — a "Tag sync" section: before appending a version heading, check `.git/refs/tags/`/`.git/packed-refs` for a matching tag and hand the user a ready-to-run `git tag` line if one's missing (tagging stays hard-denied to the agent).

### Changed
- `standards-cicd`, `standards-docs`, `standards-observability`, `standards-security`, `standards-worklog` marked `user-invocable: false` — autoloaded knowledge, not meant to be typed directly.
- `generate-readme` switched from `disable-model-invocation: true` to `paths: "**/README.md"`, so it now loads automatically the instant README.md is touched instead of requiring the typed command.
- `claude-code/settings.json`'s `skillOverrides` gained `name-only` entries for the mid-loop phases, `generate-repo`, `generate-readme`, and the audit/commit-message command skills, matching `AGENTS.md`'s reachable-by-name list.
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
- `standards-python` — a "Repo layout" section noting `generate-repo` Create is unsupported for Python (no `templates/python/` needed, matching the other unsupported languages).

### Fixed
- `comment_guard.py` — `handle_pre` no longer recomputes `scan_comments` a second time in `check_echoes`'s caller; the result is cached as `scope_scanned` and reused for both the removed-comment check and echo suppression.

## [0.22.1] — 2026-08-24

### Fixed
- `git_guard.py` — `cp`/`mv` now scan their destination against `is_code_path` (plain and `-t`/`--target-directory` forms, including a not-yet-existing directory target), closing the bypass where copying a staged file over a code path skipped `comment_guard`. `scripts/test-hooks.sh` grew from 98 to 118 cases (G31–G47).
- `git_guard.py` — recurses into `eval`/`time`/`command`/`xargs`/`nohup` wrappers with flag-aware value stripping, so `time git commit` and similar wrapped invocations no longer dodge the guard.
- `comment_guard.py` — `check_echoes` suppression now scopes against the specific edit's own `old_string`/`old_text` context instead of a file-wide comment union, so a genuine new echo-comment violation is no longer masked by identical text existing elsewhere in the file.

## [0.22.0] — 2026-08-24

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
- `home/skills/readme/SKILL.md`; `generate-repo` gained a Create branch.

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
