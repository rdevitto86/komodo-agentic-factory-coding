# Changelog

Notable changes to komodo-agentic-toolkit-coding. Format follows Keep a Changelog; versions follow SemVer.

## [Unreleased]

## [0.37.2] — 2026-08-31

### Changed
- Risk-accepted (not closed) the gap where allowlisted `Bash(grep:*)`/`Bash(sed:*)`/`Bash(awk:*)` and unrestricted `Bash(curl:*)` can read/exfiltrate the same secret-path files (`.env`, `*.pem`, `id_rsa*`, `credentials`) that `Read`'s deny list protects: Claude Code's `Bash(...)` permission matching is a fixed-prefix, wildcard-only-at-the-end language with no syntax for "this verb, wherever a secret path appears in its arguments" — a workable deny pattern for grep/sed/awk/curl would need real argument parsing (the same gap `git_guard.py` exists to close for git) which is new hook engineering, not a `settings.json` change. Mitigated today by Claude Code's own auto-mode classifier, this repo's single-operator (non-hosted, non-multi-tenant) threat model, and the absence of any live `.env`/`*.pem`/`id_rsa*`/`credentials` file in this repo's own tree.

### Removed
- `.github/workflows/windows-hooks.yml` — dropped GitHub Actions CI from this repo entirely; the local `scripts/hooks/git/pre-push-verify` dispatcher already runs this repo's own `.claude/verify.sh` (`test-hooks.sh` + `validate.sh`), the same coverage the workflow ran, so nothing regresses.

### Fixed
- `README.md`'s "Git hooks for other repos" section told a reader to run `git config core.hooksPath .githooks` — no `.githooks` directory has ever existed in this toolkit, silently disabling hooks rather than installing them. Replaced with the real `scripts/hooks/git/install.sh` usage.
- `README.md`'s "Diagram exception" note claimed only the embedded mermaid diagram was a deliberate carve-out from the `readme` skill's 6-section template, when the whole document's structure diverges. Rewrote it into a "Template exception" note naming every diverging section and why.
- `README.md`'s base-context token figure ("~894 tokens") and skill counts ("50 active, 3 parked") were stale against `bash scripts/validate.sh`'s current output and the real skill-directory count; updated to 1069 tokens and 60 active / 6 parked.

## [0.37.1] — 2026-08-29

### Added
- `scripts/validate.sh`: a `hooksPath` check that fails when `core.hooksPath` is set but doesn't resolve to a real directory, so a dangling path can't silently disable every git hook again.
- `scripts/validate.sh`'s budget pass now also reports the repo-root `AGENTS.md`'s token cost as a separate, ungated `root AGENTS.md` row (previously unmeasured entirely) — the BUDGET-gated total stays scoped to `claude-code/AGENTS.md`, the file actually loaded on every turn everywhere.

### Changed
- Corrected six stale facts in root `AGENTS.md`: the hook-regression-case count, a hooks table missing `auto_format.py`, a typed-only-skill list missing `backlog-prioritize`, a parked-skill list naming three of six `SKILL.md.off` skills, and a "stays full-description" claim naming only `workflow-loop` instead of it plus `standards-aws`/`git-merge-conflict`. Dropped a dead `write-*` skill grant from `claude-code/agents/workflow-implementer.md` — no `write-*` skill exists.

### Fixed
- `context_injector.py`'s `STORY` regex matched a pipe-delimited format nothing in this repo produces, so the `SessionStart` hook always reported "Backlog: 0 open" regardless of actual backlog state. Now matches the real `#### [TSK-E.T.S] <text> [P: SEV] [STATUS]` heading, uses `[IN_PROGRESS]` instead of the nonexistent `[WIP]` token, and excludes `[DONE]` stories from the open tally.
- `scripts/test-hooks.sh` ran ~11.5s serially with 2-3 subprocess calls per case; collapsed payload encode/decode into fewer subprocess calls and parallelized case execution behind a bounded worker pool (`TEST_HOOKS_PARALLEL`, default 8) with per-case isolated session ids and per-worker result files, cutting runtime to ~2s. The pool's initial `wait -n` throttle silently disabled itself on bash 3.2 (macOS's default `/bin/bash` doesn't support `wait -n`), letting every case run fully unthrottled; replaced with a portable busy-poll.
- This repo's `core.hooksPath` pointed at a pre-rename path that no longer exists, silently disabling `pre-commit-gofmt`, `pre-commit-hooks-syntax`, and `pre-push-verify`; repointed to the real in-repo `scripts/hooks/git`.

## [0.37.0] — 2026-08-28

### Added
- `scripts/install.py`: cross-platform installer for Windows, resolving the working `python3`/`python`/`py -3` interpreter and generating `settings.json` with an absolute, tilde-free hook `"command"` string instead of the static `"python3 ~/.claude/hooks/x.py"` that only worked on macOS/Linux. Symlinks `claude-code/` into the target the same way `setup.sh` does, falling back to a copy (with Developer Mode + re-sync guidance printed) when symlink creation fails.
- `docs/windows-install.md`: non-technical Windows install guide (Python detection, Git for Windows, Developer Mode, copy-fallback re-syncing, verifying the install).
- `.github/workflows/windows-hooks.yml`: CI verification of hook dispatch on `windows-latest`, covering both the Git Bash and PowerShell-fallback paths Claude Code actually spawns hook commands through.

### Changed
- `README.md`'s Setup section now documents macOS/Linux and Windows as separate install paths, the latter linking to the new install guide.

### Fixed
- `scripts/install.py`'s generated hook `"command"` string only double-quoted the hook path when it contained a space, with no other shell-metacharacter escaping — now quoted unconditionally with `shlex.quote()`, closing a command-injection-adjacent gap where a metacharacter-bearing path could reach `settings.json` unescaped and execute on every future hook invocation.
- `scripts/install.py`'s symlink calls never passed `target_is_directory`, which on Windows creates a broken file-type reparse point for a directory entry instead of a functional directory symlink.
- `comment_guard.py`'s Windows CI check asserted the wrong decision (`deny` instead of the real, tested `ask`) for an unallowed added comment; `context_injector.py`'s `docs/BACKLOG.md` display string baked in `os.path.join`'s native separator, rendering as `docs\BACKLOG.md` on Windows; the `test-hooks.sh` formatter no-op cases relied on an `ln -s` shim that needs the same symlink privilege this whole effort works around — now invokes the resolved interpreter by its full path instead.

## [0.36.0] — 2026-08-28

### Added
- `claude-code/skills/backlog-plan/SKILL.md` — new skill, the planning-run mode extracted from `backlog-modify`.

### Changed
- `backlog-modify` is now normalize-only; `workflow-loop`'s P0 invokes `backlog-plan` instead of `backlog-modify plan`.
- `claude-code/settings.json`'s `skillOverrides` adds `backlog-plan` as `name-only`; `AGENTS.md` documents the exception.

## [0.35.0] — 2026-08-28

### Added
- `scripts/hooks/git/pre-commit-hooks-syntax`, auto-discovered by the existing dispatcher, blocks `git commit` when the staged `git_guard.py`/`comment_guard.py` has a Python syntax error.

### Changed
- Extracted the tag-sync check duplicated across `changelog`'s "Tag sync" section and `workflow-complete`'s P4 step into a standalone `git-commit-tag` skill; both now invoke it by name.
- Split the `backlog` skill into `backlog-modify` (plan/normalize authoring modes plus the `BACKLOG.md` format spec) and `backlog-audit` (the verdict/audit mode, which edits `BACKLOG.md` directly rather than filing findings) — every caller across `claude-code/skills/`, `AGENTS.md`, `README.md`, and `settings.json` updated to match.

### Fixed
- `git_guard.py`: `PASSTHROUGH_VALUE_FLAGS["xargs"]` only recognized short-form value flags, so a deliberately constructed flag value colliding with a `MONITORED_COMMANDS` name (e.g. `xargs --delimiter git git push --force ...`) could hide the real wrapped command from scanning — same shape as the earlier `time` bug. `xargs`'s long-form value flags are now recognized too.

## [0.34.0] — 2026-08-28

### Added
- `readme` skill gains a Features section (`##2`) and an optional table of contents, both sourced from the new `templates/project/README.md.tmpl` reference skeleton.
- Documented an "Outdated dependencies" convention (`go list -u -m all`, `npm outdated`, `uv pip list --outdated`) alongside the existing CVE-scanner convention in `standards-go`, `standards-typescript`, `standards-python`.

### Changed
- `changelog`'s "Tag sync" step and `workflow-complete`'s P4 now create and push the release tag themselves (`git tag -a` + `git push origin <tag>`) instead of just handing the user a ready-to-run command — tagging a release is now an agent action, same as opening and labeling its PR.
- Renamed `audit-*` skills to `assess-*` (`audit-bugs` → `assess-bugs`, etc.) and `audit-readme` to `readme-audit`, across `AGENTS.md`, `settings.json`, `workflow-implementer`, and every skill/doc that referenced the old names — "audit" implied scoring against a fixed schema, while these skills' actual output is an open-ended judgement call, which "assess" states plainly.

### Fixed
- `claude-code/settings.json` listed `Bash(git tag:*)` in both `allow` and `deny` — deny silently won, so the agent could never actually create a release tag despite `git_guard.py`'s own logic already permitting non-destructive tag creation. Removed the stale `deny` entry.
- `AGENTS.md` falsely claimed git hooks are absent from this repo; it now describes the real `pre-commit`/`pre-push` dispatchers under `scripts/hooks/git/`, installed via `install.sh`.
- `git_guard.py`: `time --output sh <cmd>` (or any `PASSTHROUGH_WRAPPERS` flag whose unrecognized value collided with a `MONITORED_COMMANDS` name) let the guard misidentify the flag's value as the wrapped command, silently skipping its scan of the real inner command. `time`'s long-form `--format`/`--output` flags are now recognized as value-taking, closing that path.
- `README.md`'s "Workflow skills, all free" list had drifted to 16 entries against 32 actual free skills; refreshed to match.

## [0.33.0] — 2026-08-27

### Added
- `standards-api-design` — new API shape/contract conventions skill (resource naming, schema/pagination/error-shape conventions, idempotency-key and versioning design), sibling to `standards-api-security`'s security-only scope.
- `git-pr-review`, `git-pr-comment`, `git-issue-review` — new skills covering merge-readiness review, PR status comments, and open-issue triage, none of which existed before this band.
- "Security standards" sections (language-specific insecure-usage patterns) added to `standards-go`, `standards-python`, `standards-typescript`, `standards-java`, `standards-c`, `standards-dotnet`, `standards-shell`, `standards-csharp` (parked `.off`), for `audit-security` to pull from.

### Changed
- `changelog` merges `write-changelog` + `audit-changelog` (`write`/`audit` modes), same shape as the prior `backlog` merge.
- `standards-design-ui`/`standards-security-ui` renamed to `standards-ui-design`/`standards-ui-security`; `standards-security-api` renamed to `standards-api-security`; `rules-merge-conflicts` renamed to `git-merge-conflict`; `write-repo` renamed to `repo-init`; `config-accessibility-output` renamed to `config-accessibility`.
- `git-pr` split into `git-pr-create` (open/edit) + `git-pr-review` + `git-pr-comment`; `git-issue` split into `git-issue-create` + `git-issue-review`.
- `rules-source-control` and `standards-git` folded into `git-pr-create`'s new "Git lifecycle" section (branch/push/merge/protected-ref conventions merged with what `git_guard.py` actually enforces for each), since it's the skill that walks branch → commit → push → PR.
- Each `standards-<language>` skill's Comment discipline section now carries the full banned/allowed comment-template contract inline, instead of pointing at a shared `rules-commenting` skill.
- `audit-testing` now detects and loads every language manifest present in a repo, not just one; `audit-security` now also loads whichever `standards-<language>` skill(s) touched files trigger.

### Removed
- `write-changelog`, `audit-changelog` — folded into `changelog`.
- `rules-source-control`, `standards-git` — folded into `git-pr-create`.
- `git-pr`, `git-issue` — split into their `-create`/`-review`/`-comment` successors.
- `rules-commenting` — inlined into each `standards-<language>` skill.

### Known gap
- `AGENTS.md`'s `rules-<topic>` naming bucket now has no example skill (its sole example, `rules-commenting`, was removed) — left as-is pending a human decision on whether the bucket stays documented with no live example or a future skill should fill it.

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
