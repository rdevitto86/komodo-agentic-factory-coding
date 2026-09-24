# Changelog

Notable changes to komodo-agentic-factory-coding, formerly komodo-agentic-toolkit-coding and, in turn, komodo-agentic-coding-assembly-line and komodo-agentic-factory-code. Format follows Keep a Changelog; versions follow SemVer.

## 1.1.0 — 2026-09-24

- **TG-03.24** The local reviewer earns its seat (2 task(s))

## 1.0.1 — 2026-09-24

- **TG-03.23** A builder's worktree cannot push (1 task(s))
- **TG-03.22** The guard holds the forge, the gate, and every interpreter (5 task(s))
- **TG-03.20** The guard holds its own denials, and the docs match the line (5 task(s))

## 1.0.0 — 2026-09-24

The first release: feature-complete, with the three proofs recorded below, the last one real groups run with no change to the line. Everything below it is the prototype, kept as history: the 0.x experiments and the Python orchestrator, whose four releases are renumbered `1.0.0-alpha.1` through `1.0.0-alpha.4`. None was ever released for use.

- **TG-03.19** The local switch never reports a probe it skipped (1 task(s))
- **TG-03.18** Claude carries every tier until local is opted in (1 task(s))
- **TG-03.16** A stray worktree is a note, not a failure (1 task(s))
- **TG-03.17** A spawn never cuts its own worktree (1 task(s))
- **TG-03.15** The guard reads a safety mode (1 task(s))
- **TG-03.14** The doctor names a worktree the line did not cut (1 task(s))
- **TG-03.13** Prune settles every merged run, not only the last (1 task(s))
- **TG-03.12** The local mount's own paths are tested (1 task(s))
- **TG-03.11** A pipe through a filter still feeds a shell (1 task(s))
- **TG-03.10** The gate lints what a commit carries (1 task(s))
- **TG-03.9** A local machine carries a station (2 task(s))
- **TG-03.8** The line plans what it is handed (18 task(s))
- **TG-03.7** The sanity pass: safety, correctness, portability (27 task(s))
- **TG-03.6** The gate and the exit test (11 task(s))
- **TG-03.5** The repo layer and the local machines (9 task(s))

### Proof: the run skill drives a group

`komodo run TG-03.8 --budget 30m`, headless on Claude, carrying the one payload task `TSK-03.8.19`. It shipped as #148, from run `TG-03.8-1790212792` in `.komodo/line.jsonl`.

| Station | Seconds | Tokens in | Tokens out | Turns |
|---|---|---|---|---|
| brief | — | 4,177 | — | — |
| build (Sonnet) | 27.5 | 27,242 | 1,099 | 8 |
| close | 6.9 | — | — | — |
| QC | 7.7 | — | — | — |
| review | 31.2 | 40,166 | 1,857 | 4 |
| ship | 0.9 | — | — | — |

Each station ran in the order `komodo step` gave, with 0 repairs. The bar was not met on the first pass: three launches stopped on four headless-only bugs, each fixed before the next launch.

- **#145:** ship tests inherited the run's credential scrub.
- **#146:** tasks shipped earlier were replanned into a late wave.
- **#147:** a worktree's git hook looked for `bin/` in the worktree.
- **#149:** `step` reissued ship while a handoff was pending.

### Proof: a local machine carries a station

`komodo run TG-03.9 --budget 30m`, headless on Claude, with `local_reviewer` set in `~/.komodo/config.json`. `step` routed the review to `komodo machine`, so no hosted model read the diff. It shipped as #152, from run `TG-03.9-1790214162`, on the first launch with no fix needed.

| Station | Machine | Seconds | Tokens in | Tokens out | Turns |
|---|---|---|---|---|---|
| build | claude/sonnet | 71.6 | 50,992 | 3,369 | 17 |
| close | — | 9.1 | — | — | — |
| QC | — | 8.1 | — | — | — |
| review | ollama/qwen2.5-coder:3b | 5.1 | 3,018 | 63 | 1 |
| ship | — | 12.9 | — | — | — |

The review returned schema-valid JSON with 0 findings. Ship handed off once and the launcher pushed it. The Codex exit test is parked until that host has an account.

### Proof: real groups ran with no change to the line

Four groups ran `komodo run <group> --budget 30m`, headless on Claude, with the local reviewer on. Each ran every station in `step` order with no hand edit to the line or the run. Each shipped a PR that touched only its declared files plus `BACKLOG.md` and `CHANGELOG.md`.

| Group | PR | Build seconds | Build tokens in / out | Repairs | Review tokens in / out |
|---|---|---|---|---|---|
| TG-03.12 | #159 | 101.4 | 67,821 / 8,610 | 0 | 5,619 / 62 |
| TG-03.13 | #161 | 196.6 + 62.5 | 198,723 / 25,098 | 1, comments | 6,254 / 57 |
| TG-03.17 | #167 | 184.7 | 154,861 / 16,649 | 0 | 6,694 / 176 |
| TG-03.16 | #168 | 90.0 | 68,578 / 8,096 | 0 | 6,072 / 132 |

The runs are `TG-03.12-1790258917`, `TG-03.13-1790260778`, `TG-03.17-1790262942`, and `TG-03.16-1790263367` in `.komodo/line.jsonl`. The TG-03.13 repair was the line's own: close caught a comment lint failure and rebriefed the builder.

Three other runs did not count. On TG-03.11 and TG-03.15 the driver passed an isolation option to its builder spawn, so it ported the diff by hand. TG-03.17 makes the guard refuse that spawn. On TG-03.14 the new doctor check correctly blocked on that stray worktree, and the close was rerun by hand once it was removed.

Two gaps stay open past 1.0.0. The local 3B reviewer approves almost everything: it missed a real bug in #161. TG-03.17 also shipped matching the wrong spawn tool name, fixed by a one-line commit after the run.

### The line

`komodo`, one static Go binary with no dependency outside the standard library, is the conveyor: intake, brief, close, diff, report, lint, tag, release check, guard, install, doctor, machine, gate, run. A task moves through intake (`komodo next`), the brief device (`komodo brief`), a build machine, close, QC, a diff device, a review machine, and ship, two model calls per task and nothing else in the loop. A role names a tier, light, standard, or heavy, never a model; a profile maps tiers to a machine for one host, and a mount carries a brief to that machine or, for Ollama, calls it directly. Everything a model reads — rules, roles, skills, policy, standards — is markdown; the binary and the mounts under `internal/mount/` are the only Go a model never sees. One guard hook denies exactly four things: a write to a critical branch, a path outside the task's worktree, host or toolkit config, and a commit trailer; everything else inside a worktree is unrestricted. `komodo gate` — vet, test, doctor, the guard table, the binary rebuild — runs locally on pre-commit and pre-push; nothing runs on GitHub.

### Removed

- The Python orchestrator and everything it rebuilt on top of the host: worker spawning, its own worktree management, its own hook and permission model, headless mode, `komodo.json` and `komodo/config.py`. The prototype is preserved whole at the tag `prototype-final`.
- The merger role; QC stops on a conflict and hands it to a human instead of a role resolving it.
- `komodo/briefs/` and `komodo/standards/` as separate directories: a role file now carries its own JSON schema beside it, and a standard is a skill triggered by its own frontmatter globs.
- The MCP server the prototype ran at `127.0.0.1:8000`; `install` removes its entry and no facet ships an `mcp.json` that does anything in 1.0.
- Any GitHub Actions workflow; no `.github/workflows` directory exists and nothing bills CI minutes.

### What replaced it

- A profile per host (`claude`, `hybrid`, `codex`, `local`) that self-selects from the installed mount, a plan probe read from the host's own config file, and whether Ollama answers, with plan overlays for pacing and ceilings.
- `internal/mount/claude` and `internal/mount/codex` render agents, skills, and the guard onto each host from the same roles and skills; `internal/mount/ollama` is the binary acting as its own mount, posting a brief straight to Ollama's chat endpoint for read-only roles.
- A repo layer under `.komodo/`: context by glob, additive standards and skills, `commands.json` for verify and compile, additive policy, and facets (`aws`, `gcp`, `azure`, `postgres`, `github-actions`) selected by `komodo detect` or named by a task.
- The ledger, `.komodo/line.jsonl` and `.komodo/adhoc.jsonl`, stamped by every station with no model in the loop, read back by `komodo report` and `komodo metrics`.

### The swap proofs

`internal/line/swap_test.go` proves each hot-swap point with no code change and no restart: a profile row change moves a station to another machine and `komodo step` names it; a skill body change under `komodo/skills` or `.komodo/skills` reaches the next brief and the next project render; a facet added by `.komodo/facets` or a task's `facets` key reaches the standards slot, the profile slot, and the render; a `commands.json` change replaces verify at QC. MCP is the fifth point and is deferred: the same test asserts a facet's `mcp.json`, when present, changes nothing in V2.

### The readiness pass, fixed

- Every station command runs under a wall clock in its own process group: `done_when` under the task's `timeout` key (default 10 minutes), the compile and verify gates, the toolkit gate, and `after_publish`. A hung child is killed, never waited on.
- The local model is no longer a name compiled into the binary. `OLLAMA_MODEL`, then the overlay's `local_model`, then the first model the server lists, so `komodo machine` never posts to a model that is not pulled.
- A running local server no longer takes the reviewer. The `hybrid` profile keeps review on the host's heavy tier unless the overlay says `local_reviewer`, and a brief past `local_window` (default 32768 tokens) falls back to the remote tier with the reason in `komodo step`.
- `komodo brief` reads a repair from the task's own worktree, where the failed attempt's edits sit, not from the group branch.
- A context anchor that names no heading is reported by `komodo lint` and named in the brief, instead of sending the whole file.
- A task appended to the open group mid-run joins a wave after the pinned ones instead of never running.
- `max_parallel` is wave capacity inside the planner, so an overflow task shares its next wave with the tasks that became ready.
- `komodo next --start` refuses to cut a group while another is open and unshipped; `--force` overrides.
- `komodo brief` refuses a task whose directories overlap a closed, unmerged task branch of the open run.
- The guard refuses a force push, `--force-with-lease`, and a `+refspec` on every branch; the rules say the same. Six allowed rows were added so the table stays balanced.
- The headless ship files the minor findings and runs `after_publish` after the credentialed push, as the in-session ship does.
- `komodo report` reads the run's own group after it ships instead of planning the next one.
- The review diff is resolved from the same ref the group was cut from, through one resolver, and a mount names its own events file.
- The comment lint sees a doc comment above a C# attribute.
- The README states what the guard is not: a sandbox. The hard boundaries are the forge's ruleset and the credential scrub.
- A trailer fed through `git commit -F -` from a heredoc is refused; `checkout -B main` and `switch -C main` are moves onto a critical ref; `checkout main -- file` is a restore, not a switch.
- A `komodo/` directory holding none of the toolkit's entries never replaces the embedded toolkit.
- The run lock is created exclusively, so two launchers starting together cannot both take it.
- One spec shape: `docs/spec/architecture.md`, `system-design.md`, and an optional `prd.md`, plain headings, every section in exactly one file; the SDD and PRD templates are retired and the planner reads the files by path.
- The ledger archives the previous run as `line.<run>.jsonl` instead of truncating it; the brief and review stations stamp their own rows, so every station's seconds are on record.
- The local machine is reached through the registry alone: `mount.LocalMachine()` carries its probe, model, window, and one call, its name is a vendor the doctor checks, and nothing outside `internal/mount/` names it.
- `--git-dir=.git` and `--work-tree=.` name this checkout, whose branch the guard tracks; only another repository's paths are refused.

### The readiness pass, changed

- Token accounting on Claude Code sums the spawned agents' own transcripts that were handed the task's brief, never the session driving the line; cache reads are a separate `tokens_cached` field, and the build stamp names the tier, provider, and model the profile resolved. Numbers before this change are the driving session's and are not comparable.
- `komodo run` on Claude Code bypasses permission prompts, which a headless session cannot answer, and drives on the standard tier's model; the guard hook runs in every mode and stays the wall. The launcher puts `bin/` first on PATH so `komodo` resolves.
- The overlay renames a tier for this host with `"models": {"heavy": "sonnet"}`, which the agents and the profile both read.
- `komodo doctor --remote` audits the forge's branch rulesets through `gh` and reports any active one that reaches past the default branch.
- The reviewer names the files a clip marker omitted as unreviewed.
- Suggested language defaults, in the universal rules and each language standard: Zig for embedded, C++ for robotics and modules, Rust for routers and nodes, Go for web and cloud, Python for AI/ML, TypeScript with Vue or Svelte for UIs, and C only for a C-only vendor SDK or a measured hot path. Firmware rules move from `standards-cpp` into a language-neutral `standards-embedded`, and `standards-cpp` becomes the C++ standard it was named for.
- A session lists only the skills its repo can use. Both hosts select standards by the repo's own files, the render prunes a toolkit skill it no longer selects, and the toolkit's own shipped trees never select a standard for itself. In this repo, always-on skill descriptions fell from 37 skills and about 836 tokens to 6 skills and 146.

## [1.0.0-alpha.4] — 2026-09-21

Formerly 1.3.0, the last of the Python orchestrator. It spawned the host as a worker and rebuilt what the host ships: worktrees, parallel agents, hooks with deny, headless mode. By this release every open backlog item was about keeping the orchestrator safe from itself, and it was abandoned, not finished, in favour of 1.0.0: one static binary as the assembly line, markdown as everything a model reads, one guard, and a model mounted per host. It is preserved whole at the tag `prototype-final`, the last commit on `main` before the repo was cleared. The repo was renamed from `komodo-agentic-toolkit-coding` to `komodo-agentic-coding-assembly-line`, then `komodo-agentic-factory-code`, then `komodo-agentic-factory-coding`, all within the same week.

### Added
- The harness reads the logged-in Claude account once per run from `claude auth status` and derives its own limits from it. A `claude.ai` subscription is metered in tokens, so `--max-budget-usd` is dropped from the worker command line entirely; an API key, Bedrock or Vertex keeps it, and an undetected account keeps it too. The email and org id the command also returns are never read
- Turn caps are solved from the measured cost law rather than written down: builder input tracks `1400 × turns²`, so a tier's cap is `sqrt(plan_budget × tier_share / 1400)`. Pro and unknown resolve to roughly today's numbers, Max to nearly double. An explicitly set cap still wins
- The plan sets a model ceiling. Pro, Team and unknown cap at Sonnet; Max and Enterprise reach Opus. The Claude adapter renders session agents from the same resolution, so a Pro machine never renders an Opus agent. `account.model_ceiling: false` turns it off
- Every worker result carries the newest `rate_limit_event` from the CLI stream, and the run state keeps the last one. Before each wave the pipeline logs a usage window past `rate_limit.warn_at` and stops cleanly past `rate_limit.pause_at`, naming the window and when it resets, instead of burning turns into a refusal
- `komodo status` prints the detected plan, how it is metered, the scaled worker timeouts, and one row per role with its model, effort, turn cap and dollar cap. `--json` carries the same report under `limits`

### Removed
- `komodo.json`. The file was never load-bearing: this repo's copy held fifteen keys, all fifteen identical to the defaults in `komodo/config.py`, and its only real read was the git guard's `protected` list, whose built-in fallback is the same list. Defaults now come from `komodo/config.py` alone, and a machine that wants to override something writes `.komodo/config.json`, which the toolkit owns and gitignores. `templates/project/komodo.json.tmpl` is gone, so a new repo is never handed one
- The instruction to "set base in komodo.json" from the base-branch error. It now names what it looked for and points at `--base`, so nothing tells a reader that work is blocked on a missing config file

### Changed
- The Claude worker runs under `--output-format stream-json` through `Popen` instead of `subprocess.run` with a single JSON envelope. A worker killed by the timeout now reports the turns and tokens it actually burned; it previously recorded 0 turns and $0.00
- `worker_timeout_s` is no longer flat. It scales with the bytes behind the files a task names and with the plan's headroom, capped by the new `worker_timeout_max_s`. The `done_when` gate gets the same scaled timeout
- `context.file_chars` is a total ceiling of 120000 rather than a 24000 pool divided by the file count. The new `context.per_file_chars` gives each file 10000 characters, so a builder is no longer shown less of its own code the more files its task names
- A worker stopped by a ceiling now says which ceiling bound and what it had spent when it did, so the next raise is not a guess
- The repair attempt is handed the diff the first attempt left in the worktree, instead of re-deriving it from the error string alone

## [1.0.0-alpha.3] — 2026-09-18

Formerly 1.2.0.

### Changed
- A task's status vocabulary is `REFINEMENT`, `READY`, `IN_PROGRESS`, `BLOCKED`, `DONE`. `READY` replaces `TODO`, and `REFINEMENT` holds open work the harness never picks up: `next_group` and the pipeline select on the new `Task.ready`, and `tasks lint` stops demanding `files` or `done_when` until a task is promoted out of refinement. Parsing maps a legacy `TODO` token to `READY`, so an unmigrated backlog still runs

### Removed
- The `TODO.md` entry in `.gitignore`, left over from the project template that retired the file

## [1.0.0-alpha.2] — 2026-09-17

Formerly 1.1.0.

### Added
- The turn-end summary splits `## ⚠️ Flagged Changes` into `## 📌 Callouts` and `## ⚠️ Warnings`, so a filed review finding no longer reads as something going wrong; run notes are the only warnings
- `python3 -m komodo pr label --auto` derives a PR's labels from its commit type and the `komodo.json` label map, the same way the pipeline labels a PR it opens, so a PR opened by hand no longer goes up bare
- New `python3 -m komodo release check`: a read-only release-integrity gate, wired into `make verify`, failing on a changelog entry with no tag, a tag with no changelog entry, and a date-separator mismatch between headings
- The session hooks ship as one compiled Go binary, `komodo-hooks`, with a subcommand per hook. `context_injector.py` is ported to Go beside the guard, and `scripts/build-hooks.py` cross-compiles both for darwin, linux, and windows on amd64 and arm64
- The Git hooks join them: `komodo-hooks precommit` and `komodo-hooks prepush` port `pre-commit.py` and `pre-push.py`, so a commit or a push is guarded on a machine with no Python at all. The sh stubs pick the binary for the running machine and fall back to the Python only when no committed target matches
- komodo tasks plan validates every proposed done_when by running it once in a scratch worktree before appending

### Fixed
- Changelog writes carry a preservation invariant: `render.assert_preserved` refuses any `_write_changelog` or `_cut_version` write that drops a released version heading, which is how 0.46.5's Security entry was overwritten in place and lost. That entry is restored here from `v0.46.5`
- Preflight tags every untagged changelog version, not only the newest, so a gap below the top heading (0.50.0 and 0.51.0 here, tagged over by v1.0.0) is no longer invisible. A heading or blockquote reading `never released` is skipped
- Preflight tags a merged version no tag points at, and `komodo doctor` fails on a protected branch when the newest version is untagged
- Cover render.py and the CLI with tests
- Close the files doctor opens, so verify stops printing ResourceWarnings
- Remove the dead conditional in the builder commit path
- Detect an unrunnable done_when on Windows, where cmd.exe does not use exit 126 or 127

### Removed
- `## [Unreleased]`, everywhere. The staging section is gone from `render.py`, `pipeline.py`, `doctor.py`, the `release` command, both session hooks, and the project template. `render.infer_bump`, `render.cut_release`, `render.has_unreleased_entries`, and `komodo release --bump` went with it; a version is declared in the backlog, never inferred
- `.github/workflows/verify.yml`, and with it GitHub Actions from this repo. The same `python3 scripts/verify.py` runs on the developer machine through the `pre-push` hook, which refuses the push when it fails

### Changed
- The Go source and the prebuilt binaries moved from `komodo/adapters/claude/hooks/` to `komodo/hooks/`, since one binary now carries the Git hooks as well as the session hooks and no longer belongs to the Claude adapter. Protected-branch matching, the trailer pattern, and repo-root discovery are written once in `repo.go` and shared by all four subcommands, replacing a second and third hand-rolled copy in the two Python Git hooks
- `komodo install` requires Python only for the hooks that are still scripts. Every platform with a committed binary installs without an interpreter, and the install log names which implementation each hook got instead of falling back silently
- publish drops completed tasks from BACKLOG.md instead of marking them DONE; git history and the changelog hold what shipped
- A task group declares `version: x.y.z` in `BACKLOG.md`, and `komodo tasks lint` refuses a group without one. Close-out copies that version into the changelog heading in the same commit as the code, so the changelog and the git tag can no longer disagree

## [1.0.0-alpha.1] — 2026-09-16

Formerly 1.0.0.

The harness rebuilt from scratch as a standalone orchestrator. The 0.x prose state machine, its text firewall, and the comment `apply` path are gone; what replaces them is code with the same intent and a fraction of the cost.

### Added
- `komodo/`, a stdlib-only Python package run as `python3 -m komodo`. `run` takes a task group through preflight, branch, parallel build waves in worktrees, a compile gate per wave, one verify, one review, changelog and backlog update, push, and PR. `status --prune`, `tasks lint|list|migrate|add|plan`, `comments check`, `hooks install`, `install`, `doctor`, `pr threads|label|comment|reply|sync|respond`, and `release` cover the rest of the line. Every phase persists to `.komodo/runs/<id>/state.json`; `--resume` continues an unfinished run; `--dry-run` prints waves, briefs, and per-section token estimates without spawning.
- A task grammar for `BACKLOG.md`: a fenced yaml block under each task with `files`, `done_when`, `depends_on`, `context`, `owner`, and `type`, and `mode: single|parallel` per group. `komodo/yamlite.py` parses the subset with no dependency; `tasks migrate` converts the 0.x table shape best-effort and reports what needs a human.
- Directory ownership for parallelism: tasks in disjoint directories share a wave, same-directory tasks serialize, dependencies order the waves. The orchestrator reruns every `done_when` itself; a worker's word is not the proof.
- Two worker providers behind one `Brief` in, `Result` out contract: `claude -p` with model, effort, turn cap, budget cap, tool set, and JSON schema set from the role spec, and an Ollama adapter for summarizing. Two profiles, `fast` and `thinking`, in `komodo.json` with `.komodo/local.json` as the personal overlay.
- Credential isolation: workers run with no GitHub token, an empty credential helper, SSH in batch mode with no identity, and an unauthenticated `gh`. `komodo/gitops.py` is the only pusher and refuses protected refs, force, amend, trailers, and merges into a protected branch in code.
- Python git hooks under `komodo/hooks/` with two-line sh stubs Git dispatches on every platform: pre-commit refuses a protected branch and a trailer, checks gofmt, and lints comments on staged lines; pre-push refuses protected refs, deletes, and non-fast-forward updates, then runs the repo verify gate.
- One reviewer pass per group carrying bug, security, test-gap, simplify, narrative-comment, and undocumented-nonobvious classes with a severity scale. Floor findings get one repair; the rest are filed to the backlog by code. The same pass scores the diff's blast radius on a six-tier scale, measuring fan-out with the dependency command the language standard names, and the score lands in the run report and the PR body.
- `komodo doctor`: dangling backticked paths and `/skill` names, personal keys in the settings policy, hook commands naming missing files, skill frontmatter drift, stale worktrees and merged branches. Runs inside `scripts/verify.py`.
- `komodo/rules/` and `komodo/roles/`: the universal rules, the backlog grammar, the CLI guide, and one file per role carrying a tier, an access class, and both a worker and a session output contract. Workers and interactive agents read the same body. `komodo/adapters/claude/` renders `~/.claude` from them; no Claude-specific directory is checked in.
- Profiles map tiers (`light`, `standard`, `heavy`) to a provider, model, and effort, and roles declare a tier. A `local` profile points every tier at Ollama.
- `komodo/standards/`: 33 rules-only files, one per language or domain, injected into a worker by the extensions and directories its task touches and clipped to a cap. Every language the 0.x skills covered is carried over, including the ones that shipped disabled (Rust, Zig, Swift, Kotlin, C, C++, C#, Java, .NET, Azure, GCP, hardware); there is no off state because an unused file costs nothing.
- Mandatory accessibility rules for every human-facing output in `claude-code/AGENTS.md`, and the same density caps applied in code by `komodo/render.py` to reports and PR bodies.
- A CI matrix (Ubuntu, macOS, Windows, plus Python 3.9) running `python scripts/verify.py`.
- Root `LICENSE` (MIT, copyright 2026 R. DeVitto) and `SECURITY.md` — the repo previously stated no terms at all while its installer symlinks the clone into `~/.claude`. `SECURITY.md` names GitHub private vulnerability reporting as the channel, states that every file under `claude-code/hooks/` runs as a `PreToolUse`, `PostToolUse`, or `SessionStart` command on every tool call, that a symlink install moves every session on its next start unless `--ref` pinned it, and that `--ref` and `CODEOWNERS` are the existing mitigations. A band-review correction narrowed the symlink claim: `scripts/install.py` falls back to `copytree`/`copy2` when `os.symlink` raises on Windows without Developer Mode or when `--force-copy` is chosen, and a copy install propagates nothing until the installer is re-run.

### Security
- Read-only git is now enforced per agent instead of only stated in prose. `git_guard.py`'s reviewer-only identity gate generalized into one `AGENT_READ_ONLY_GIT_SUBCOMMANDS` table keyed on six identities, denying by default any subcommand outside a listed agent's set — `builder`, `tester`, `scout`, `researcher`, and `architect` each previously carried "nothing enforces this" in their own file, and now do not. The orchestrator path (a payload carrying no agent identity) stays untouched, since the workflow loop's own commits depend on it. `rev-parse` is granted to all five. `scripts/test_hooks.py` grew from 324 to 339 cases.
- Band review found three live bypasses in that generalization, all closed: `git diff --output=PATH` is a file-write primitive whose deny had been reviewer-only, so three agents holding no `Write`/`Edit` tool at all could write any path, hooks included, through an allowlisted subcommand; a leading `GIT_EXTERNAL_DIFF=` environment assignment and a `-c diff.external=` config option each execute an arbitrary command through an allowlisted subcommand, and those denies had also been reviewer-only.
- Stated the honest limit rather than an overclaim: the guard denies a direct git invocation by identity and is not a sandbox. `builder` and `tester` must run arbitrary Bash for their `Done when` commands and the repo's verify target, so an interpreter hop such as `python3 -c` reaches git regardless — the five agent files and `AGENTS.md` now say so directly, rather than replacing an honest "nothing enforces this" with a claim that overstates the enforcement.

### Changed
- The Claude Code layer is rendered, not maintained. `komodo install` produces a 40-line `AGENTS.md`, one agent per session role with model and effort from the active profile, three procedure skills from `komodo/rules/`, and one thin `standards-*` pointer per rules file, every skill `name-only`, keeping the always-on listing near 900 tokens against a 1,500 budget.
- `komodo/adapters/claude/settings.policy.json` replaces the tracked `settings.json`. It carries only `permissions` and `hooks`; `skillOverrides` is derived at render time; `komodo install` merges those keys into `~/.claude/settings.json` and keeps every personal key.
- The comment rule: a doc line on every public function, one on a private function only when it is long or has more than one return, silence elsewhere. The lint keeps `EXTERNAL_REF`, `NAME_ECHO`, `OVER_LINES`, `STACKED`, `MALFORMED_MARKER`, and `FUNC_UNDOCUMENTED`, adds `OVER_WORDS` and `NARRATIVE`, ignores `#` inside Python string literals, and runs in pre-commit on staged lines.
- `scripts/verify.py` runs the unittest suite, `scripts/validate.py`, the comment lint, and `komodo doctor`. `scripts/validate.py` checks frontmatter keys, the token budget, agent profiles, and brief template pairs.
- Python floor is 3.9.

### Removed
- `git_guard.py` (1,376 lines), `verify_gate.py`, `auto_format.py`, `comments.py apply` and its `PostToolUse` hook, `lib/comment_rules.py`, and `scripts/test_hooks.py`. The guarantees moved into `gitops.py`, the worker environment, and the Python git hooks; the lint moved to `komodo/comments.py`.
- The `workflow-*`, `backlog-*`, `assess-*`, `changelog-*`, `readme-*`, `git-*`, `work-state-map`, `write-comments`, `repo-assess`, `config-accessibility`, `standards-worklog`, `adr`, `prd`, `sdd`, and `runbook` skills, 43 in all, and every `evals/` directory. The pipeline they described is `komodo/pipeline.py`; the accessibility rules are always-on in `AGENTS.md`.
- `claude-code/` itself, `scripts/install.py`, `scripts/release.py`, `scripts/evals.py`, `scripts/work_state.py`, `scripts/test_install.py`, the shell dispatchers under `scripts/hooks/git/`, `templates/briefs/`, `templates/skills/`, the `go` and `node` Makefile templates, the ADR and runbook templates, the README template, and the `pm` agent (its job is `komodo tasks plan` with the `planner` role). `SECURITY.md` shrank to the reporting channel; the install-grants text lives in `docs/architecture.md`.
- `bridges/komodo-bridge/`. Its two prompt files and README moved next to the bridge server they belong to, under the local `.komodo/bridge/agents/` deploy, which loads `agents/<name>/agent.md` relative to itself. The harness never depended on the bridge; `komodo/workers/ollama.py` talks to Ollama directly.
- 40 backlog items that only concerned the deleted machinery. The V1.1 epic carries forward the bridge `num_ctx` limit, CI secret scanning, and the tests the new PR actions still owe.

## [0.51.0] — 2026-09-16

### Added
- `FUNC_UNDOCUMENTED`, which demands a comment on every function declaration — public and private, in every language the lint resolves a family for, not Go alone. Four exemptions carry the design: a test path, a one-statement body, a bodyless declaration, and a generated file. A Python docstring or a JSDoc block satisfies it outright rather than earning a second comment above it. `RET_ARITY_3` and `RET_BOOL_DISCRIMINANT` stay Go-only and outrank it. Detection is a stricter sibling of the line cap's detector — permissive is fine when the cost is granting a function literal two lines, and wrong when the cost is a finding on `go func() {`.
- `EXTERNAL_REF`, which refuses a comment citing a version number, a `PRD`/`SDD`/`ADR`/ticket, or the conversation that produced it — `notFoundGet bool // simulates forge-sdk-go v0.36.0+ ...` is the case it was built from. `apply` refuses the same text, so the sanctioned write path cannot land one either. `RFC` is deliberately not on the list: "RFC 3339 timestamp" describes the code, and a rule that cannot tell a protocol citation from a project-management one is wrong more often than right.
- `OVER_LINES`, a comment-block line cap read off the declaration the block sits above: two lines for a function, one for a `var`, `const`, `type`, or statement. A machine directive inside the run does not count against it, the leading file header stays exempt up to `HEADER_MAX_LINES`, and `apply` refuses a proposal that would push the run it lands in over the cap — including a single line spliced above a comment that was already there, which every check the validator had used to pass.

- `verify` now runs as a `fail-fast: false` matrix over `ubuntu-latest`, `macos-latest` and `windows-latest`, plus a `python:3.7` container leg pinning the declared floor. Windows has been a first-class platform since `scripts/install.py` grew its copy fallback — `docs/windows-install.md` documents it and `scripts/test_install.py` covers it — and had never once run in CI, which is how the `build_settings` defect below reached a release. The gate is invoked as `python scripts/verify.py` rather than `make verify`, since `make` is absent on a stock Windows runner and the Python entry point already leads the gate-resolution order for that reason. The floor leg is a container because `actions/setup-python` publishes no 3.7 build for Ubuntu 24.04, and the `ubuntu-22.04` label that used to carry one entered failing brownouts on 2026-09-17.
- `scripts/test_install.py`'s 22 cases are now a fourth check in `scripts/verify.py`, with a matching `Makefile` target beside the existing three. The suite had run nowhere automatic — not in CI, not on a `builder` Stop — so the entire `--ref` and installer guard surface was unprotected against regression. The gate measures ~11s with it wired in, against `KOMODO_VERIFY_TIMEOUT`'s 300s default.

### Fixed
- `build_settings` resolved a hook command's script with `command.endswith(name + ".py")`, which cannot match the one registered command carrying a subcommand — `python3 ~/.claude/hooks/comments.py hook`. That entry fell through the rewrite and kept a literal `~`, which a hook command does not expand, so on Windows and anywhere `python3` is not on `PATH` — precisely the cases `settings_strategy` picks the generate path for — the `PostToolUse` comment lint installed dead and never ran. The resolver now scans `shlex` tokens for the script and passes every trailing argument through, so the subcommand survives. `I2` was tightened from a whole-file tilde grep to a per-command walk of `settings["hooks"]`, since the grep caught this but could not name which command was wrong.
- Six cases in `scripts/test_install.py` create a real `os.symlink` or assert a POSIX path shape, so the new `windows-latest` leg would have been red on every pull request for reasons that say nothing about the platform. They now skip on Windows and report the skip as its own outcome with a reason, counted in the summary line beside passed and failed — a skip printing as a pass would let a platform leg go green while proving nothing, which is the same class of defect as the one above.
- The comment doctrine reached almost nobody who writes comments. `write-comments` has no `paths:` and is `name-only` in `skillOverrides`, so it never auto-loads and is barely discoverable; `comments.py hook` was registered only in `builder.md`'s frontmatter, so a primary session editing code got no feedback at all while `auto_format.py` ran globally; and `builder` — the path AGENTS.md names as the author inside the loop — carried the whole policy compressed into one paragraph, pointing at a `context: fork` skill it has no instruction to read. The hook is now registered in `settings.json` too, and a new `standards-comments` skill carries the rules path-gated onto every source extension the lint knows, so it loads for any agent touching code. It is the 28th path-gated skill and costs nothing always-on; the budget is unchanged at 1278.
- Five language skills still described a `PreToolUse` guard deleted several releases ago — "a non-compliant comment prompts the user for approval before the write lands" — and four of them opened with "You must strictly limit code comments", the opposite of the current rule. `standards-shell`, `standards-dotnet`, `standards-java`, `standards-database`, `standards-python` and `standards-typescript` now defer to `standards-comments` rather than restating it, and `standards-vue`, `standards-svelte` and `standards-react` stopped recommending the `WHY:` marker prefix, which `is_plain_body` has rejected since the taxonomy moved to implicit types.
- `builder` can no longer close a task with comment findings outstanding. "Re-run `check` until it exits 0, **or** every remaining finding is one you are listing as skipped with a reason" gave a fork at the end of a long task a sanctioned way to skip the work. A `MISSING` on a function it wrote is now never skippable; only a pre-existing `INVALID` may be left, named in `## Notes`.

### Changed
- `STACKED` stopped doing two jobs. It was applied per comment line with `ADJACENT_WINDOW` at 2, so the second line of any block was already a finding and the effective cap was one line everywhere outside a file header — right in spirit, illegible in its message, and wrong for the one case that wants two. It now governs only the distance between two *distinct* blocks; lines inside one block are `OVER_LINES`' business. A header run past `HEADER_MAX_LINES` still loses its exemption, now reported as `OVER_LINES` at the line the exemption ran out on.
- `write-comments` now carries two defaults instead of one. "Write nothing" was the cause of both complaints at once: it produced justification essays where a comment did land, and left functions undocumented where one should have. A function declaration is documented by default; everything else — a statement, a branch, a `var`, a struct field — stays silent by default. The restatement ban is scoped to the second case, since saying what a function does is the job in the first. `DOC` also lost its exported-only gate, because refusing name-first on unexported declarations forces a non-idiomatic phrasing on half a Go codebase once every function owes a comment.
- `write-comments`' bar is what the code does, as it stands. It previously asked for "a discovered constraint" and "a rejected alternative" while banning "what this function does" as narrative, which is a brief for a justification essay with the description filed off — and that is what came back. Restatement, hypotheticals ("can only mean a programmer mistake"), call-site reasoning ("every call site above passes a non-empty prefix"), and point-in-time context ("raised from 200 after the audit") are now named bans, the nine template types are unchanged, and the mandatory discriminant-return comment survives because what a return value discriminates is a standing property of the code. `builder`'s Comments-last step carries the same bar.

## [0.50.0] — 2026-09-15

### Added
- The first skill eval suites, so skill quality stops being unmeasured. `scripts/evals.py` is a stdlib-only, on-demand runner with a `--list` that invokes no model, and `workflow-loop`, `backlog-modify`, `git-pr-create`, `write-comments` and `assess-bugs` each carry one case and one grader under their own `evals/` directory, built against the `claude plugin eval` interface. Deliberately outside `verify`, the `Makefile` and CI: the gate is 9.2 s and runs on every dirty `builder` Stop, so model-calling evals would change what that gate is. No baseline has been run yet and no score is recorded anywhere — that step is outstanding, not done, and an invented number would be worse than an absent one.
- `scripts/validate.py` now fails a skill that is model-invocation-disabled, path-ungated, argument-hint-less and named by no sibling — the exact condition that let two skills go dead unnoticed — and fails any agent that declares no `maxTurns`. A new drift check owns the installed `settings.json`'s verdict, reporting when a generated copy no longer matches what `build_settings` would produce; it is a no-op where that file is a live symlink.
- One brief template per role under `templates/briefs/`, and a stop-on-missing-slot rule in every agent body. A brief was described in prose in three places and validated nowhere, so an omitted slot was only found by the fork guessing or stopping. A fork now returns naming every missing slot and writes nothing — that return replaces its whole output template, since a brief that cannot be executed has no findings, no options, and no queue to report. Not every slot binds every role: `Done when` is required for a builder and a tester, optional for a reviewer whose completion is its own output contract, and absent for scout. Every fork skill's `argument-hint` names the full set its agent will require, so a caller cannot be blocked by a convention it was never told.
- A fast path for a change that is decided but trivial. Entry is checked, not judged — a file count, a line count, and a path exclusion list, run as commands — and it skips planning and consolidation only: verification and review run unchanged. The exclusion list refuses outright for the hook surface, the agent definitions, the verify gate and every script it runs, the git-hook dispatchers, the installers, and the always-loaded directive files, because a change able to weaken its own verification is not a fast path.
- Parallel execution of provably disjoint tasks. `pm` returns a files-touched manifest per task, and two tasks run at once only when their manifests are present and disjoint — an intersection, a missing manifest, or a guess all mean serial, so parallelism is opt-in on proof rather than on assumption. Each parallel task runs in its own worktree and the orchestrator merges before the band gate. Read-only fan-out needs neither manifest nor worktree and is always parallel; that half was free all along.
- `scripts/work_state.py` and the `work-state-map` skill give one view over what is planned, open, blocked, and shipped — the two halves that otherwise live in `BACKLOG.md` and `CHANGELOG.md` read end to end. The parser is stdlib-only with no install step, emits epics, task groups, and releases as coordinate-free JSON, and the skill renders it as Mermaid with no library. Releases are a separate spine rather than linked per task group, because a closed-out task is deleted from the backlog and survives only as changelog prose with no identifier to match on — a fuzzy title match would be invention, so none is drawn.
- UI standards split by platform instead of by concern. `standards-ui-web` replaces `standards-ui-design` and `standards-ui-security`, carrying both halves as sections so `assess-security` reads the Security half the way it already reads a language skill's. `standards-ui-mobile` and `standards-ui-desktop` join it, each with the same two halves. Mobile's Design half is native-UI only; its Security half applies to any native manifest, React Native and Flutter projects included, because permissions, deep-link registration, and the screen-capture flags are native-shell concerns whatever renders above them.
- Every `standards-<lang>` Toolchain section names the command that language's own toolchain ships for listing internal dependencies — and says so plainly where the toolchain ships nothing, rather than reaching for a third-party tool. `assess-change-risk` states when to run it and how the reverse-import set maps to a tier, so blast radius is no longer scored with no import data behind it.
- Two roles join the agent roster. `architect` weighs a design question and returns options, their trade-offs, and a recommendation — it never decides, because it cannot see the constraints that live outside the codebase. `tester` writes tests against an interface that already exists and never touches the code under test; its path scoping is stated prose, not a lock, so running it beside `builder` still needs `isolation: worktree`.
- The `verify` target in both scaffolded Makefiles (`templates/go/`, `templates/node/`) now runs `comments.py check`, so the enforcement path the design decisions document describes exists in every repo this toolkit creates. It runs last, so a formatting, test, or build failure still surfaces first.
- `scripts/hooks/git/install.sh` gains its first regression coverage, in `scripts/test-install.sh` — stale `--status`, stale install, and the already-installed and orphaned `.git/hooks` paths as anchors.

### Changed
- The missing-brief-slot rule is stated once instead of seven times. Its agent-independent half moved into `claude-code/AGENTS.md`, which already reaches every custom agent and forked skill; each agent keeps only its own required-slot list and return shape, which genuinely differ by role. Read-only git stayed per-agent on purpose — the subcommand lists are not identical, and `reviewer`'s is enforced by the hook rather than held by its own prose. The always-on budget moved 1136 to 1278 tokens against a 2000 limit, which is the deliberate price of the hoist: the rule had already drifted in `reviewer.md`, and a rule with seven copies has no owner.
- `standards-swift` and `standards-kotlin` are disabled the visible way. Both carried `disable-model-invocation: true` with no `paths:`, which reads as a live skill but is reachable by nobody; they are now `SKILL.md.off`, matching the seven skills already disabled that way. Neither was turned on, and `standards-ui-mobile` still covers `.swift` and `.kt` files.
- The Bash allowlist no longer carries entries that grant nothing. `sed`, `cp`, `mv` and `tee` are denied by `git_guard.py` for every guarded target regardless, and `cd` and `true` are auto-allowed by the CLI unconditionally; `for`, `test`, `source` and `export` stay because they still grant something. `curl` was narrowed to six per-host rules and then restored within the same band — permission rules are prefix string matches over raw command text, so `curl https://github.com/x -o <path>` satisfied the pin and the rules constrained only where a command started. Host pinning is not expressible here; egress control belongs in the sandbox's `deniedDomains` or in the guard.
- `workflow-loop` is the machine again, not the machine plus its reference material. The file had grown to 4745 tokens against a 5000-token compaction re-attach cap, and two attempts at trimming sentences had netted nothing, because the shape was the problem: four documents shared one file, and only one of the two entry hatches is reachable in any given run. The fast path moves to `ways/fast.md` and loads only when the task asks for it, delegation and the required brief slots move to `delegation.md` and load once per run, and the open hatch collapses into the dispatch line that was already deciding between them. Both new files sit inside the skill directory, so they install wherever the skill does — the failure mode of pointing at something that does not install was already found once. No rule was deleted and no reasoning thinned; the result is 3874 tokens.
- The four places that each restated a delegate's required brief slots now defer to the one table in `delegation.md`. They were written inline because the fuller templates live under `templates/briefs/`, which does not install into a target repo; the new table does, so the duplication has nothing left to buy.
- The toolkit's own gate and test surface are Python, so they run on a machine with only `python3` and `git` — no Git Bash, no `make`. `validate.sh`, `test-hooks.sh`, `test-install.sh`, `release.sh`, and `setup.sh` are gone, replaced by `scripts/validate.py`, `scripts/test_hooks.py`, `scripts/test_install.py`, `scripts/release.py`, and an `install.py` that absorbed the installer rather than being a second one beside it. `verify_gate.py` discovers `.claude/verify.py` then `scripts/verify.py` ahead of the shell and `make` targets, invoked as interpreter plus path so no executable bit is needed — and `make verify` is now a one-line wrapper, so what `verify` means is defined once. The git-hook dispatchers stay shell, because git runs a `core.hooksPath` hook through its own bundled shell; that decision is recorded with its falsifier.
- `standards-shell` states when a script must be Python rather than shell, and that a repo's verify gate must be invocable on every platform the repo claims to support. `standards-cicd` covers runner-matrix portability. Both are already `paths:`-gated, so neither costs anything in the always-on budget.
- The repo's full verification suite runs once per band rather than once per builder fork. Each task's own `Done when` commands are unchanged — the task's proof was never the redundancy. The deferral is an out-of-tree marker keyed by the git common dir, so it cannot be committed into a repo's history and is visible from inside every worktree; absent, malformed, expired, unkeyable, or past its thirty-minute cap all run the gate, because the safe default is the redundant check. Two concurrent sessions sharing one checkout can still cross, which the design notes record as a known limitation rather than a solved problem.
- The output contracts now obey the show-don't-tell rule they were governed by. `config-accessibility` gains a conditional visual-first rule — a dependency edge between entities, or more than a handful of them, triggers a diagram, and below that the table alone is correct and complete — and says where the diagram goes, distinguishing inline Mermaid from a published artifact. `workflow-decompose`, the eleven `assess-*` skills, and the `reviewer` and `pm` agents name a diagram form alongside their table. The diagram is additive in every case, so a reader on a surface that renders nothing loses nothing.
- The agent roster is named for roles instead of workflow phases: `workflow-implementer` is now `builder`, `workflow-planner` is now `pm`, and `engineering` is now `researcher`. `reviewer` and `scout` already named roles and are unchanged. Every agent body now describes the role's standing boundaries rather than the phase that invokes it, so an agent used for ad-hoc work is no longer misdescribed by its own name. `verify_gate.py`'s `Stop` registration and the `comments.py` `PostToolUse` hook moved onto `builder` with the rename — they live in the agent file, so leaving them behind would have disarmed the gate silently.
- A review finding now has to clear a stated evidence bar — a trigger, the path it reaches, and an observable effect, each anchored to a cited `file:line` — rather than only being forbidden from being invented. The reviewer agent gains a `Considered and dismissed` section so a near-miss keeps its reasoning without inflating the findings count, and an empty findings table is stated to be a successful review. `assess-bugs`, `assess-testing`, `assess-vulnerabilities`, and `assess-performance` each carry the matching bar.
- The reviewer agent runs on the stronger model. Precision under adversarial reading is the one thing that role exists for, so it is the wrong place to spend a cost saving. All three finder forks run as that agent and move with it.
- The reviewer's output contract splits by owner: the agent owns the section structure and the rules, and the invoking skill owns the findings table's columns and its `Sev` scale. Previously both declared a "mandatory" template and the two disagreed.
- A session starting in a repo with no verify gate now gets an explicit warning naming what is not running — the repo's own checks, and `comments.py check` with them — rather than a neutral status line that read the same as the released version and backlog tally beside it. Output for a repo that does declare a gate is unchanged.

### Fixed
- `AGENTS.md` said `git_guard.py` fails closed. It fails open — the crash handler exits 0 and denies only when the command matches a hardcoded destructive pattern — so the always-loaded project doc was asserting a safety posture the code does not keep. The handler and its four regression cases were already right; only the prose was wrong, including a second stale reference further down the same file.
- `claude-code/AGENTS.md` described the guard's reach as `.py`/`.json`/`.md` and three write verbs. It actually covers every extension the comment lint knows plus its own document set, against every write verb it recognises. Restated by the rule rather than by a list, since the list is what drifted.
- `builder` had no turn ceiling, alone in a roster where every other agent declares one, and is the only agent with a blocking `Stop` hook. Its brakes do not substitute: three byte-identical failures or a warning at six never fire for a fork alternating between two different failures. Now 100, above `reviewer`'s 80, because a builder writes, runs its own `Done when` commands and lints on top.
- On Windows the installer always generates `settings.json` rather than symlinking it, so the resolved interpreter can be baked into each hook command — but the re-sync notice fired only when a symlink was refused. A Developer Mode user got live symlinks for skills, agents and hooks beside a silently frozen permission file, with nothing printed. The installer now reports the cause, a deliberate generate gets its own message naming that file, and the drift check catches the same divergence after the fact.
- `scripts/hooks/git/install.sh --status` could not tell a `core.hooksPath` pointing somewhere deliberate from one pointing at a directory that no longer exists, so a repo running zero git hooks looked normal from the one command meant to audit exactly that. The path is now classified, a stale one is marked, and installing over one notes that hooks had not been running.

## [0.49.0] — 2026-09-10

### Added
- `BACKLOG.md`'s format gains two optional elements: a `User Story` block, and a checkbox `Acceptance Criteria` list — the one place a checkbox is now sanctioned in the file. A `Category` column (`[Impl]`/`[UnitTest]`/`[IntegTest]`/`[E2E]`/`[Audit]`) is added to the Subtask table, which also lets the old standalone "every task group carries its own `Tests:` task" rule retire as redundant.
- The four standing closeout tasks (Security review, Bug sweep, Code smell, Performance) move out of `Cross-Cutting` into their own per-epic `Quality Assurance & Epic Hardening` task group, gated by a `Trigger` bullet that fires once every functional task group in the epic reaches `[DONE]` — not on every band's changelog write, as the prior wording implied.
- A `[DONE]` task carrying any unchecked Acceptance Criteria box is not swept — both `backlog-audit`'s periodic pass and `workflow-consolidate`'s routine per-band deletion now check this. `workflow-implement` ticks a box once the subtask proving it goes green.

### Changed
- The old "no checkboxes, anywhere" and "no standalone Acceptance Criteria list" rules are gone, replaced by the narrower rule above — checkboxes are still never sanctioned for a subtask, a task's own status, or a `Blocked by`/`Owner` field.

### Fixed
- `templates/project/BACKLOG.md.tmpl` referenced nonexistent `/audit-security`/`/audit-bugs`/`/audit-simplify` skills; corrected to `/assess-*`. Its example test commands were JS-specific (`npm run test:*`); genericized.

## [0.48.0] — 2026-09-10

### Added
- `setup.sh --ref <tag>` pins an install to a release tag. Without it the symlinks in `~/.claude` track whatever the clone has checked out, so an upstream sync moved every engineer's next session with no staging step and no rollback; `--ref` detaches the clone at the tag before linking, so an install moves only when its consumer re-runs `setup.sh` with a different ref. The is-a-git-repo, ref-resolves, and clean-tree guards all run before any mutation — under `--dry-run` too, so a bad ref fails the preview without touching `HEAD` or a symlink. With no `--ref`, behavior is byte-for-byte unchanged.
- Root `CODEOWNERS` over `claude-code/AGENTS.md`, `claude-code/settings.json`, and `claude-code/hooks/` — the three paths that reach every session in every project on the next start, with no staging environment in between.

### Fixed
- `README.md` described a blocking comment hook that does not exist. `git_guard.py` denies at `PreToolUse`, before the command runs; comments are the opposite shape — `comments.py hook` reports on a write without blocking, and `comments.py check` is what fails the `verify` target. The always-on budget figure was also stale (1,045 against an actual 1,129), and is now a rounded value plus a pointer to `scripts/validate.sh`, which prints the exact number.

### Security
- Two bypasses of the new `--ref` guard, found at band review and closed in the same band. An empty `--ref=` — a wrapper whose tag lookup returned nothing — fell through every `[ -n "$REF" ]` gate and silently performed the unpinned install the flag exists to prevent. A ref name beginning with `-` reached `git checkout --detach "$REF"` as a bare argument, where git parses it as an option; `--` cannot close that one, since before a checkout argument it introduces a pathspec rather than a revision, so the shape is rejected up front and the detach now takes the SHA `rev-parse` already resolved instead of the name the caller supplied.

## [0.47.1] — 2026-09-10

### Security
- `reviewer`'s Bash surface is now deny-by-default: `git_guard.py` allows only read-only `git log`/`diff`/`show`/`status`/`blame`/`ls-files` (no global flag, no `--output`, no leading env-var assignment) and denies everything else outright, closing the whole class of bypass a prior series of comments.py-specific pattern fixes chased one signature at a time — including a live-confirmed arbitrary-code-execution path via `GIT_EXTERNAL_DIFF`.

## [0.47.0] — 2026-09-10

### Added
- New `git-branching-strategy` skill — when a change belongs on one short-lived branch off main versus a longer-lived feature branch of stacked PRs, cross-referencing `git-pr-create` for mechanics.
- New `standards-zig` skill — Zig memory/allocator, error-handling, comptime, build-system, testing, and C-interop conventions, structured like `standards-aws`.

### Changed
- `claude-code/AGENTS.md`'s atomic-write rule for a live `hooks/` file now names the Edit/Write tool as the sanctioned path, since `git_guard.py` already blocks the shell `mv` sequence the rule previously prescribed.

## [0.46.5] — 2026-09-10

### Security
- `reviewer`'s Bash surface is now deny-by-default: `git_guard.py` allows only read-only `git log`/`diff`/`show`/`status`/`blame`/`ls-files` (no global flag, no `--output`, no leading env-var assignment) and denies everything else outright, closing the whole class of bypass a prior series of comments.py-specific pattern fixes chased one signature at a time — including a live-confirmed arbitrary-code-execution path via `GIT_EXTERNAL_DIFF`.

## [0.46.4] — 2026-09-10

### Changed
- `AGENTS.md`'s no-scope-expansion rule now explicitly covers formatting/lint reflow of untouched lines — drive-by reflow of a pre-existing line is out of scope even when the file is already open for another reason, since a shared file may carry another engineer's in-flight edit to that line.
- `git_guard.py`'s `repo_root_of()` now delegates its subprocess-call-and-except core to `lib.git.repo_root()` instead of duplicating it, closing the drift that let the same `TypeError` gap sit unfixed here after `lib/git.py` had already received the fix.

### Fixed
- `git_guard.py`'s `repo_root_of()` now also catches `TypeError`, matching the fix `lib/git.py`'s `repo_root()` already received — a non-str/bytes/PathLike `cwd` no longer crashes uncaught in this fail-closed-by-design hook.
- Root `AGENTS.md`'s hook-test-count comment updated from 244 to 259, matching `scripts/test-hooks.sh`'s actual case count.

## [0.46.3] — 2026-09-10

### Fixed
- `lib/git.py`'s `repo_root()` now also catches `TypeError`, so a non-str/bytes/PathLike `cwd` (e.g. from a malformed hook payload) fails open deliberately instead of depending on whichever caller's outer exception handler happened to rescue it.
- `standards-go/SKILL.md`'s two "modernize" transform bullets no longer hardcode a Go version number as their gate condition — both now phrase it relative to `go.mod`'s floor, consistent with the file's own stated convention.

## [0.46.2] — 2026-09-10

### Changed
- `git_guard.py`'s `extract_substitutions` no longer hand-rolls its own second copy of `split_segments`'s quote/comment/boundary state machine — both now share one `classify_shell_char` helper, and the two duplicated `$()`-capture call sites inside `extract_substitutions` collapsed to one.

### Fixed
- `git_guard.py`'s `segment_wants_reparse` no longer treats `command -v`/`command -V` (existence/type checks that never execute their argument) the same as `command eval` (which does) — fixes a false-positive deny on a harmless existence check.
- `git_guard.py`'s `env`/`time`/`nohup`/`xargs`/`command` recursion lost shell quoting on rejoin, so a wrapped `sh -c "multi word"` argument split apart on re-tokenize and only its first word was scanned — recursion now round-trips through `shlex.quote` to preserve the original token boundary.

### Security
- `git_guard.py`'s `env` wrapper handling only recursed on a literal `-c` token, but real `env` has no `-c` flag — `env <any guarded command>` bypassed every pattern the file checks, not only the reviewer-write case that first surfaced it. Fixed by composing the file's existing generic flag-stripper with its `VAR=val` walk instead of a narrow hand-rolled `-i`-only check.
- `git_guard.py` also treated `env -S`/`--split-string` as a discardable value flag; its argument is actually the wrapped command (the same role `-c` plays for `sh`/`bash`) and was silently discarded rather than scanned — now shell-split and recursively scanned like `sh -c`'s argument.

## [0.46.1] — 2026-09-10

### Added
- `workflow-loop`'s P2.3 band review now also dispatches `/assess-performance` conditionally, whenever a touched path is performance-sensitive (a hot loop, a changed complexity class, a new query/index) — restores coverage for the "Performance" standing closeout story that P2.2–P2.4's restructure (0.46.0) had dropped. Unlike the three forked finders, it runs inline and self-files, matching the contract `/assess-code-quality` already relies on.

### Changed
- `workflow-loop/SKILL.md`'s P2.3 now states the severity-floor rule and round-cap thresholds once, deferring to `ways/sdlc.md` by reference instead of restating them — the two copies had no mechanism keeping them in sync.
- `comments.py`'s `run_check` and `hook_findings` now share one `collect_findings` helper instead of independently building `MISSING`/`INVALID` finding dicts; a resulting double-sort (each per-file batch sorted twice) is fixed — `run_check`'s own final sort covers the multi-file case (`collect_files` walks the filesystem with no ordering guarantee), so the per-file sort moved to `hook_findings`, its only caller with no outer sort of its own.
- README.md, AGENTS.md, and `docs/design-decisions.md` reconciled against the whole `TG-01.4` band: the always-on token count (1,045, was stale at 949), the agent list (`reviewer` was missing), the hooks table (`comments.py hook` was missing a row), the test-case count (244, was stale at 160), the Mermaid diagram (a `backlog-audit`-at-P1 node that was never accurate and is now doubly wrong since `backlog-audit` left P2.4 too), and the mid-loop skill list (`assess-performance` added, `/repo-init`/`/readme` corrected to their current names).

### Security
- A harness-attested-token mechanism was built and self-tested as a structural fix for the risk-accepted `comments.py apply` gap 0.46.0 documented, then reverted after two independent bypasses were reproduced against the actual code: `env VAR=val cmd` bypasses a shell `readonly` guard by constructing the child process's environment directly, and the token's state file is an ordinary, world-readable file `reviewer`'s `Read` tool reads without ever touching `git_guard.py`'s Bash-only `PreToolUse` hook. The risk-accepted status from 0.46.0 stands; the attempt and its root cause are recorded in `BACKLOG.md` (`SUB-01.4.9.5`) so a future attempt doesn't rediscover the same break.

## [0.46.0] — 2026-09-10

### Added
- `comments.py` gained a hook subcommand: `PostToolUse` feedback on the just-touched file, reported as `additionalContext`, always exits 0. `workflow-implementer.md` registers the hook and adds a Comments-last craft step using `comments.py check`/`apply`; `write-comments` is now the manual/repair path instead of a downstream P3 fork.
- `workflow-consolidate` now clears a satisfied `[BLOCKED]` Recheck and confirms each shipped task's `Done when` commands before deleting it, band by band, at closeout; `backlog-audit` is repositioned as a full-file sweep a user types or a session runs on staleness, no longer invoked mid-loop.
- `verify_gate.py` now blocks with the configured limit named when `KOMODO_VERIFY_TIMEOUT` is hit (previously exited 0 silently), skips the verify command entirely on a dirty tree whose only paths are records-only (`BACKLOG.md`/`docs/BACKLOG.md`/`CHANGELOG.md`/`README.md`), and stops a stuck fork at 3 consecutive identical failure hashes (exits 0 with a `systemMessage`) instead of grinding toward Claude Code's 8-consecutive-block cutoff.
- `reviewer.md` and `workflow-implementer.md` now run at `effort: high`, matching `settings.json`'s sonnet/opus default — both had been silently running at `medium` because agent frontmatter overrides `modelSettings`. Every read-only agent gained a `maxTurns` cap sized to its job (`reviewer`: 80, `workflow-planner`: 50, `engineering`: 40, `scout`: 20); `workflow-implementer` gets none, since `verify_gate.py`'s `Stop` hook already bounds it.

### Changed
- `workflow-loop`'s P2–P3 phases rebuilt around a band review: P2.2/P2.3/P2.4 restructured into verify+commit / once-per-band review with a severity floor / changelog-only closeout, replacing per-task review and the P2.4 `assess-*` repeat. Removed the inaccurate `isolation: worktree` dispatch/merge-back instructions (the Skill tool has no `isolation` parameter) and stripped P3 of the `/write-comments` call and the retired removed-comments ledger.
- `reviewer` is now read-only: `Edit` dropped from its tools list and its `## Filed` output section removed — `assess-bugs`/`assess-security`/`assess-simplify`/`assess-code-conventions` return findings only, and the caller files them. `reviewer_guard.py` and its `settings.json` registration are retired; `git_guard.py`'s `is_guarded_path` now denies every Bash-side write for the `reviewer` agent unconditionally.

### Fixed
- The comments hook subcommand's file-read had re-rooted from the touched file's own git ancestry instead of `args.repo_root`, skipping the existing containment check and reading files unbounded — fixed to pin root to `args.repo_root`, gate every read through `is_within_repo_root`, and cap reads at 1MB.
- `workflow-complete`'s backlog-staleness check used `git log -S'backlog-audit'`, which false-positived on any commit whose diff merely mentions the string "backlog-audit" in prose (an unrelated docs edit, for instance) rather than an actual sweep — fixed to `git log --grep` against commit messages, paired with a documented convention that a `backlog-audit` run names itself in its own commit message.
- `verify_gate.py`'s new records-only skip had an unhandled 5s timeout on its own git-status probe that could silently skip a genuinely dirty tree — fixed so a probe timeout falls through to running verify normally instead of skipping.
- `git_guard.py`'s new reviewer-write deny for `comments.py apply` (see Security) closed four successive bypasses across a whole-band review: a bare interpreter-less invocation, a `python3 -`/stdin-piped script body, a same-content copy under an unrelated basename, and a case-folded path alias — moved from basename text-matching to `os.path.samefile()` identity plus a content-comparison fallback, run unconditionally per segment rather than gated behind a command-name allowlist.
- `workflow-loop/SKILL.md` and `docs/design-decisions.md` carried three stale references to P2.4 running review (`assess-*`) and to `reviewer` filing its own `## Filed` findings, both left behind by this band's earlier restructure — corrected to the current P2.3-only review, reviewer-files-nothing shape.

### Security
- Filed `TSK-01.1.17` (Critical): `git_guard.py`'s `env` wrapper handling only recurses on a literal `-c` token, but `env`'s real syntax (`env cmd args...`) has none — `env <any guarded command>` bypasses every pattern the file checks, not only the reviewer-write case that surfaced it. Pre-existing, unrelated to this band's changes; out of scope here, tracked under the existing `git_guard.py` hardening task group.
- Risk-accepted (not closed): `git_guard.py`'s reviewer-write guard for `comments.py apply` can still be defeated by a functionally-identical copy of the script that differs by even one byte, which passes both the identity check and the exact-match content fallback. Four rounds of fix-and-reloop closed the realistic bypass paths (missing interpreter prefix, stdin piping, basename spoofing, case-folding); this residual gap is a semantic-equivalence question no static command-text or content analysis can decide, the same class of limit this file's `eval`/variable-indirection gap already carries. Tracked as `SUB-01.4.9.4` for a structurally different fix (gating `apply` on something outside command-text analysis entirely) rather than a further pattern-match round.

## [0.45.0] — 2026-09-09

### Added
- `verify_gate.py` now tracks its own approximate consecutive-block streak per repo (keyed off the repo root, cleared on any pass or skip) and appends a warning to the block reason once that streak nears the 8-consecutive-block point where Claude Code stops honoring a `Stop` hook — previously a fork hitting that cutoff went silent with no in-repo signal.
- `standards-go` now documents `errors.AsType[T]` (Go 1.26+) and embedded-field composite-literal flattening (Go 1.27+, `gopls`'s `embedlit`), verified against real Go/gopls release material after an earlier verification pass incorrectly concluded neither existed.
- `scripts/test-hooks.sh` gained a regression suite locking in `reviewer_guard.py`'s fail-open behavior on malformed/malformed-typed input, mirroring `git_guard.py`'s existing coverage.

### Fixed
- `docs/design-decisions.md`'s "no SKILL.md is invisible" enumeration now includes `standards-c`, matching the parked-skill list elsewhere in the same doc.
- `scripts/test-hooks.sh` now pins `cwd` one way (a top-level JSON field) instead of two; the `cd`-into-fixture-dir mechanism is kept only as the dedicated regression test for `git_guard.py`'s `os.getcwd()` fallback.

### Security
- `git_guard.py`'s `extract_substitutions` now only unescapes a `$()` capture's backslash-escaped backticks when the enclosing segment actually re-parses via `eval`/`-c` — closes a real bypass (`eval "$(echo \`git push --force\`)"` previously went undetected) without the over-broad unescape that briefly followed it false-positiving on inert text merely quoting a banned command in escaped backticks.
- `git_guard.py`'s `segment_wants_reparse` now looks past a `command` prefix (with or without its value-less flags) at what it actually runs, so `command eval "$(...)"` re-parses the same way a bare `eval` already did.
- `git_guard.py`'s segment-boundary tracking now advances past a `#`-comment's closing newline before checking for `eval`/`-c` re-parse, closing a bypass where a preceding comment line let a hidden `eval` slip through undetected.
- Risk-accepted (not closed): a `$()` capture containing an escaped backtick, re-parsed via `eval`/`sh -c` reached through variable indirection (`RUN=eval; $RUN "$(...)"`), an alias, or a shell function rather than a literal leading token, is not statically detectable by this line-local scanner — recognizing it would require cross-statement data-flow tracking this scanner doesn't do.

## [0.44.0] — 2026-09-09

### Added
- `scripts/validate.sh` gained a "cross-skill reachability" check: it fails when a skill body invokes a sibling that carries `disable-model-invocation: true` and so cannot actually be reached via the Skill tool.
- `workflow-implement/SKILL.md` now verifies a task's premise before forking — it reads the function/file the task names and confirms the described defect is still present, so a stale backlog entry the repo has already outgrown is reported back instead of implemented.
- `workflow-loop/SKILL.md`'s P2.1 now requires the brief to state the chosen mechanism (and why) whenever a task involves a genuine design choice, rather than leaving it to the implement fork's judgment.
- `claude-code/AGENTS.md` now states that a file under `claude-code/hooks/` is live via symlink the instant it's saved, and requires a nontrivial edit there to go through an atomic temp-file-then-`mv` write.

### Fixed
- `scripts/validate.sh`'s budget pass no longer overstates the always-on token cost by folding in a `paths:`-gated skill's full description; it now excludes that cost (or counts only its name-only cost when `skillOverrides` collapses it), reporting the gated set as a separate figure.
- `repo-assess` can now compute a real 9-category composite score: `assess-readiness`, `assess-code-quality`, and `assess-testing` dropped `disable-model-invocation: true` (the human-decision gate stays on `repo-assess` itself), per the resolution recorded in `docs/design-decisions.md`.
- `scripts/test-hooks.sh`'s `bash_case`/`smoke_case` helpers now pin `cwd` to a fixed fixture repo instead of sending none, so their deny assertions no longer depend on `git_guard.py` falling back to the suite's own invocation cwd.

## [0.43.0] — 2026-09-09

### Added
- Five `standards-*` skills (`standards-go`, `standards-typescript`, `standards-python`, `standards-java`, the parked `standards-c.off`) gained a shared Dependency Inversion Principle statement and wrapping/magic-literal/guard-clause conventions, each phrased to the language's own idiom.
- `standards-go` gained a const/var zero-cost distinction, a sentinel-identity-comparison rule, a hard 120-col `lll` lint gate (`templates/go/.golangci.yaml`) with a documented 90-col soft wrap threshold, magic-number rule extensions for string literals/cross-package placement/const-block grouping, a repo-wide-sweep `-count=1` verification methodology, a full-inlining default for single-call-site helpers, and a testable-logging `Logger` interface pattern.
- New `assess-code-conventions` skill: a standalone, user-invoked style/formatting finder scoped to judgment calls no linter/formatter can mechanically decide; wired into `assess-code-quality` as a standing input.
- New `reviewer_guard.py` hook makes the `reviewer` agent's "edit only `BACKLOG.md`" boundary mechanical instead of prose-only, for both `Edit`/`Write` and (via a `git_guard.py` extension) `Bash` writes; hardened against leaf- and ancestor-directory symlink bypasses and a fail-open gap on an unresolvable repo root.
- `.github/workflows/verify.yml`: this repo's own `make verify` now runs in CI on pull requests and pushes to `main`, least-privilege `contents: read`.
- `setup.sh` now verifies `python3` resolves and meets the hook layer's floor (3.7) before linking anything.

### Fixed
- `comments.py` no longer defines `KNOWN_TEMPLATE_TYPES` twice; its default (changed-lines) check now folds in untracked-but-not-ignored files as wholly-changed, so a brand-new file is linted without needing `--all`.
- `comments.py check --all` no longer flags a file's leading shebang/header comment block for `STEP_MARKER`/`STACKED` (capped at 30 lines so unbounded narrative padding disguised as a header is still caught), and `BANNER_OUTSIDE_TEST` is now scoped to `.go` files only.

### Changed
- Recorded a naming decision: the `workflow-*` skill prefix stays as-is rather than renaming to `harness-*`, since this toolkit is agent configuration riding on top of whichever harness runs it, not a harness in its own right (`docs/design-decisions.md`).

## [0.42.0] — 2026-09-09

### Added
- `workflow-loop`'s P0-P4 phases (and each forked skill call within them) now record silent, session-stated timing — reported only when the user explicitly asks how long something took, never printed by default.

### Changed
- `workflow-loop`'s P2.4 closeout now dispatches `assess-bugs`/`assess-security`/`assess-simplify` in parallel (`isolation: worktree`, since all three write `BACKLOG.md`) instead of serially, matching the pattern already proven for P2.1's parallel task dispatch — the single clearest actionable slowdown a harness audit found in the review loop.

## [0.41.0] — 2026-09-09

### Added
- `workflow-loop`'s P2.3/P2.4 review phases gained a real round-cap: a third straight new Critical/High finding on the same file within one band stops auto-continuing and surfaces the choice (fix now / risk-accept and ship / defer) to the user instead of re-looping — the budget drops to 2 rounds for a file already flagged in `BACKLOG.md` as a hand-rolled parser or security boundary. From round 2 onward, a review call's brief also states its round number and asks for only clear, concrete, reproduced findings, not theoretical edge cases.
- A session-stated retry-counter tally now enforces that cap uniformly across P2.1/P2.3/P2.4 — the orchestrating session states "round N for `<file>`" before each repeat invocation of the same skill against the same file/task, replacing the prose-only "same check failing twice" rule that depended on the session remembering to apply it (`docs/design-decisions.md`: no new state file, tracked in P2.0's existing perpetual-context queue).

## [0.40.9] — 2026-09-09

### Changed
- `git_guard.py`'s three separately hand-rolled `MAX_SCAN_DEPTH` bound checks consolidated into one shared `check_depth()` helper; `repo_root_of` memoized with `functools.lru_cache` so a multi-argument write command (`cp`/`mv`/redirect/`tee`) reuses one `git` subprocess per `cwd` instead of spawning one per source argument.
- `scripts/test-hooks.sh`'s smoke-test cases `S4`/`S6` deduped against `G58`/`G107` (previously byte-identical commands) into distinct representative commands.

## [0.40.8] — 2026-09-09

### Added
- `workflow-decompose` returns a `## Parallel` section naming task sets with no shared file or dependency edge.

### Changed
- `workflow-loop`'s P2.1 dispatches a `workflow-decompose`-confirmed independent task set together with `isolation: worktree` instead of one task at a time, with P2.0/P2.2/P2.3 stating how per-task WIP tracking, verify/review/commit, and worktree merge-back work for that set. P2.4 skips the `assess-bugs`/`assess-security` repeat for a single-task band already cleared at P2.3 with no diff change since.

## [0.40.7] — 2026-09-09

### Changed
- Four model-visible skills (`standards-aws`, `readme-audit`, `git-merge-conflict`, `workflow-loop`) moved to `name-only` in `settings.json`'s `skillOverrides` — each was already reached only by explicit name but was paying its full description in the always-on listing. Dropped two dead `skillOverrides` entries (`workflow-authoring`, no skill directory; `standards-c`, disabled). Always-on budget drops from 1165 to 949 tokens.
- `workflow-loop`'s Guardrails section states: don't re-open a file a fork just wrote — work from its returned `## Filed`/`## Changed` block instead of re-reading the whole file.

## [0.40.6] — 2026-09-09

### Changed
- Renamed `repo-init` to `git-repo-init` and `readme` to `readme-modify` (pairing it with `readme-audit`), and split `changelog` into `changelog-write`/`changelog-audit`, matching the `backlog-modify`/`backlog-audit` precedent — cross-references across ~20 skills, `workflow-implementer.md`, `README.md`, and `settings.json`'s `skillOverrides` updated to match.

## [0.40.5] — 2026-09-09

### Security
- `git_guard.py` closed five more bypass/outage bugs: the redirect-operator regex missed a real file-descriptor redirect (`1>`, `2>`, `9>`, `&>`) into a guarded path; `sed`/`perl`-in-place detection missed a quoted or split-quoted `-i` flag (`"-i"`, `-'i'`) even though the shell passes it through unchanged, now normalized via `shlex`-based word reconstruction before matching; `is_guarded_path` denied a same-named/same-extension write anywhere on the filesystem with no check that the target actually resolved inside the repo, now resolved from the target's own location rather than the caller's `cwd` (closing a second bypass a review round found in the first fix, where a `cwd` outside any git worktree exempted an absolute write into a real guarded file); and `main()`'s catch-all converted any exception — an internal hook bug as much as a genuine policy decision — into the same deny, causing a full command-denial outage on a single unhandled crash. On an internal crash, `main()` now falls back to a minimal, hardcoded check for unambiguously destructive operations (`git push`/`commit`/`rebase`/`reset`/`clean`/`filter-branch`/`filter-repo`, `rm -rf` and its flag-order variants, `sudo`) and still denies those, allowing everything else through rather than either denying every command or allowing every command. A review round found that same fail-open-on-crash path itself reachable by attacker-controlled input (a deeply nested `$(...)` chain, as a sibling command or inside a redirect/`tee`/`cp`/`mv` target word, drove recursion past Python's limit and fell into the crash path): `scan_command`/`scan_segment` and `expand_word`/`resolve_command_output` now share a `MAX_SCAN_DEPTH` bound and deny once exceeded instead of raising. `scripts/test-hooks.sh` gained a fast-fail smoke-test set run before the full case list, plus `G134`–`G143` (202 total, up from 193).

## [0.40.4] — 2026-09-08

### Security
- `git_guard.py`'s redirect/`tee`/`cp`/`mv` guarded-path checks closed a chain of bypasses: a whitespace-bound regex missed a quoted or command-substitution target entirely, and the `sed`/`perl`/`python`-in-place-write and commit-trailer scanners ran against raw unmasked text, false-denying a command that merely quoted one of those patterns as data. A `resolve_guard_target`/`shell_words`/`expand_word` path now fully dequotes (mid-word splits included) and recursively resolves nested or concatenated `$(...)`/backtick substitutions before checking the target, and terminates correctly on an unmatched `)` closing a bare `(...)` subshell without truncating a literal parenthesis inside a real filename. Risk-accepted, not closed, same posture as `[0.37.2]`'s grep/sed/awk/curl gap: a `$(...)`/backtick body whose output is *computed* by the inner command at runtime (e.g. a `python3 -c` one-liner assembling a filename) rather than spelled or `echo`'d literally in its own arguments still evades the check — closing that in general needs either executing the substitution or denying every unrecognized substitution shape outright, both real costs against this hook's usability; mitigated by the toolkit's single-operator, non-hosted threat model. `scripts/test-hooks.sh` gained `G101`–`G124` (184 total, up from 179).

### Fixed
- `standards-go`'s `SCREAMING_SNAKE_CASE` guidance told the model to preserve legacy screaming-case consts as a package's intentional convention, contradicting idiomatic Go (stdlib and every major Go style guide use MixedCaps for exported consts, lowerCamelCase for unexported). Replaced with: never introduce a new `SCREAMING_SNAKE_CASE` const, exported or not; an existing one is grandfathered legacy and a rename-sweep candidate, never a pattern to continue. Also stated the boundary for an explicitly-requested repo-wide casing sweep — rename every exported identifier the repo itself owns, never one owned by an external/SDK package just because a local file references it.

## [0.40.3] — 2026-09-02

### Security
- `git_guard.py`'s redirect/`tee`/`cp`/`mv` guards (`is_guarded_path`, renamed this band from `is_code_path`) now also cover `.json` and `.md`, closing a side door where the `reviewer` agent's `Bash` grant could overwrite `claude-code/settings.json` or `BACKLOG.md` unexamined by `comment_guard.py`'s Edit/Write-only scoping — `settings.json` is the file that registers `comment_guard`/`git_guard` as hooks in the first place. `scripts/test-hooks.sh` gained `G99`/`G100` (199 total, up from 197).

### Added
- `claude-code/hooks/auto_format.py`'s formatter selection and invocation logic was extracted into a `run_formatter(path)` function; `write_comments_validator.py` now calls it after every successful comment splice, so a spliced comment gets the same `gofmt`/`prettier` pass a normal Edit/Write on that file type would already trigger. `scripts/test-hooks.sh` gained `V3`.

## [0.40.2] — 2026-09-01

### Fixed
- `git_guard.py` denied a `grep`/similar command whose pattern text merely mentioned a git verb inside backticks (e.g. a single-quoted pattern containing `` `git log` ``), wrongly reading the quoted text as a real command-substitution boundary. Replaced the quote-blind regex segmenter with a quote/comment/substitution-aware char-by-char scanner.

### Security
- Fixing the above false-positive surfaced and closed four Critical bypasses in `git_guard.py`'s command scanner: a `#` shell comment could swallow the quote-tracking state across a newline and hide a following mutating command; backtick and `$(...)` command substitution weren't recognized as segment boundaries at all, so a mutating `git` command hidden inside either form was invisible to the scanner; single-vs-double-quote suppression semantics were backwards (only single quotes suppress substitution in real bash — double quotes do not); and an old-style backslash-escaped nested backtick could hide a mutating command inside an outer substitution. `scripts/test-hooks.sh` gained cases `G90`–`G98` (196 total, up from 187) covering all of it.

### Changed
- `git_guard.py`'s `extract_substitutions` had its four near-identical backtick/`$(...)` capture-and-mask blocks collapsed into one shared `capture_and_mask` helper, and `find_backtick_end`/`find_paren_end` unified onto one `scan_masked_span` scanner parameterized by a terminator predicate — no behavior change, `scripts/test-hooks.sh`'s `G90`-`G98` cases still pass.

## [0.40.1] — 2026-09-01

### Changed
- `claude-code/skills/config-accessibility/SKILL.md` trimmed from 2,445 to 793 tokens, dropping sections that duplicated `CLAUDE.local.md`'s always-on rules while keeping the turn-end summary schema, density caps, emoji protocol, code-answer format, document typography, and self-check.

### Fixed
- `~/.claude/CLAUDE.local.md`'s dead `config-accessibility-output` skill reference (no such skill existed since a rename) corrected to `config-accessibility`, so the skill actually loads when the local file points to it.

## [0.40.0] — 2026-09-01

### Changed
- `workflow-loop`'s `SKILL.md` trimmed from ~5,077 to ~3,487 tokens (justification prose cut, every rule/table-row/`Ends when` line kept) so the file survives Claude Code's 5,000-token compaction re-attach cap; added a Guardrails bullet to re-read the active `ways/` file after any context compaction, since it loads via `Read`, not skill invocation.

### Added
- `scripts/validate.sh`: a "skill compaction cap" check that fails any `SKILL.md` exceeding 5,000 estimated tokens, reusing the existing base-context-budget check's shared directory walk and token estimator.

## [0.39.1] — 2026-09-01

### Changed
- `git-pr-create`'s P4 read now uses `git diff <base>..HEAD --stat` instead of the full diff, opening a single file's diff only when the stat line and commit messages leave the change genuinely ambiguous — cuts the orchestrator's per-band token cost at publish time.

## [0.39.0] — 2026-09-01

### Added
- `claude-code/agents/reviewer.md`: a new agent that reads a diff cold and files findings to `BACKLOG.md`, edit-restricted to that file only (enforced by a new `comment_guard.py` check, not just agent prose). `assess-bugs`, `assess-security`, and `assess-simplify` now fork into it instead of running in the orchestrator's own window; `backlog-audit` now forks into `workflow-implementer` the same way.
- `claude-code/hooks/lib/git.py`: a shared `repo_root()`, single-sourcing what `comment_guard.py`, `context_injector.py`, and `verify_gate.py` each previously defined separately.

### Changed
- Root `AGENTS.md` cut from ~7,157 to ~1,416 tokens — every "why it was decided this way" paragraph moved verbatim into new `docs/design-decisions.md`, keeping only what a session needs to act.
- `claude-code/AGENTS.md`'s claim that a subagent never inherits it was corrected — every custom agent and every forked skill actually receives it, and the four affected agent bodies (`engineering.md`, `scout.md`, `workflow-implementer.md`) had their genuinely-redundant restatements trimmed while keeping the "read-only git" rule each still needs (`git_guard.py` permits ordinary git verbs globally; it doesn't gate by agent identity).
- 17 bundled/plugin skill descriptions (`claude-api`, `dataviz`, `design`, and others) collapsed to `name-only` in `claude-code/settings.json`'s `skillOverrides`; `enableWorkflows` disabled since nothing in this toolkit uses the `Workflow` tool.
- `scripts/validate.sh` now states in its own output that its token total excludes bundled and plugin skills; the same caveat was added to `AGENTS.md`.

### Fixed
- `scripts/validate.sh` printed its bundled-skill NOTE twice (once per branch of the same budget check) — now prints once, unconditionally.

## [0.38.0] — 2026-09-01

### Added
- `write-comments` skill + dedicated agent — the only path that can add a code comment now; drafts proposals and splices them itself via a new deterministic validator, `write_comments_validator.py`, rather than editing files directly.
- `comment_removal_log.py` — a new `PostToolUse` hook that logs every approved comment removal to `.claude/state/removed-comments.jsonl` for later review.
- `workflow-implementer` gained an optional `## Comment Candidates` field for capturing live WHY-context at implementation time instead of writing an inline comment.

### Changed
- `comment_guard.py` now flat-denies any narrative comment addition (banner/`WHY`/`NOTE`/`FIXME`/`HACK`/`TODO`/step-marker) — no ask, no approval path; only a machine directive or the shebang-manual case still passes silently.
- Extracted comment-shape rules out of `comment_guard.py` into a shared module, `claude-code/hooks/lib/comment_rules.py`, so the guard and the new validator share one source of truth.
- `workflow-loop`'s P3 now runs `write-comments` once per band before its consolidate commit, fed the band diff, backlog stories, drafted commit message, and accumulated Comment Candidates, and clears the removal log afterward.
- `AGENTS.md`'s hook documentation and `standards-go`/`standards-python`'s comment-rule sections rewritten to match the new flat-deny behavior, replacing stale text describing the old ask-based guard.

### Fixed
- A same-edit comment removal previously short-circuited `comment_guard.py`'s deny checks, letting a narrative comment addition land unevaluated whenever the same edit also removed an existing comment — the single most common real-world case this redesign was meant to close.
- `write_comments_validator.py` rejected proposals whose `file` path resolved outside the target repo root (a `../`-escaping or absolute path could otherwise have been written to).

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

