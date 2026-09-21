# komodo-agentic-toolkit-coding

Komodo's agent toolkit: one set of rules, roles, standards, skills, and guardrails that any agent host renders and runs. The host is the factory; Komodo is the assembly line. Today the factory is Claude Code with Ollama beside it. The day it is something else, the move is one adapter file.

**Status: V1.5 is planned, not built.** This README is the plan. V1, the Python orchestrator under `komodo/`, is what runs today and builds V1.5 until the run skill lands; the demolition group at the end of the roadmap removes it. The work is in `BACKLOG.md`.

## Vision

A Komodo developer clones one repo, runs one install, and every session and every unattended run on their machine carries the same rules, the same standards, the same roles, and the same guardrails. Work enters as tasks in `BACKLOG.md` and leaves as reviewed pull requests, with a model called exactly twice per task: once to build, once to review. Everything between those two calls is deterministic code, identical on every host and every operating system.

Three commitments hold it together:

1. **The line is code; the loop is the host.** Picking the wave, assembling a brief, validating a result, rerunning the acceptance commands, merging a worktree, denying a command: standard-library Python. Spawning agents, giving each a fresh context, running hooks, headless mode: the host's job, never rebuilt here.
2. **Nothing outside the adapters knows the host.** Rules, roles, skills, and policy are three open formats. `komodo doctor` fails the build on a vendor name, a host tool, or a host path anywhere else.
3. **Context is budgeted, not hoped for.** Always-on context stays under 1500 tokens. Every slot injected into a builder has a character cap and a visible cut marker. A session sees a standard as a one-line pointer; a brief carries the clipped copy.
4. **Nothing of V1 is lost by accident.** The V1 coverage table at the end of this README maps every V1 capability to the task that carries it, or says it was dropped and why.

Windows is a requirement, not a port: every hook, the install, and the launcher run there with no shell, no symlinks, and no build step.

## Core features

### One neutral source, rendered per host

Everything Komodo says to an agent lives once under `komodo/`: the universal rules in `AGENTS.md`, one file per role with its tier and tools in frontmatter and its brief template as the body, one `SKILL.md` per standard and per workflow, and one `policy.json` for what no agent may do. An adapter of about 100 lines renders that source into a host's own layout: agents, skills, hooks, and settings for Claude Code, or TOML agents and a hooks file for Codex. `python3 -m komodo install --host claude` is the whole switch, and the same command with another host name is the whole migration. Two developers on two operating systems run the same install and get byte-identical rules.

### The assembly line in code

`komodo tasks` is the line. `next` prints the next ready group as JSON with its waves and dependencies, skipping tasks that already hold a valid result on disk, which is how a run resumes. `brief` fills a role's template, writes the scope file, and creates the worktree. `close` validates the builder's JSON against the role schema, reruns the task's `done_when` commands, runs the comment lint, and flips the status token; with `--wave` it merges worktrees in order and stops on conflict, and with `--group` it commits, pushes, and opens the PR. `diff` and `report` feed the reviewer and the human. None of these calls a model.

| Station | Who | What |
|---|---|---|
| Intake | `tasks next --json` | The next READY group: tasks, dependencies, waves by directory, type, version. Tasks with a valid result on disk are skipped, which is resume |
| Brief | `tasks brief <task>` | Fills the role template from the slots, writes the brief and the scope file, creates the worktree |
| Build | builder role, one per task in the wave | Reads its brief path; owns the scope; writes its result JSON |
| Close | `tasks close <task>` | Validates the result against the role schema, reruns `done_when`, runs the comment lint, flips the status. A failure writes the failure slot for one repair |
| Wave | `tasks close --wave` | Merges the wave's worktrees in order, stops on conflict, runs the repo's verify |
| Review | reviewer role, fresh context | Reads only `tasks diff`; on another tier or vendor when the profile says so. One pass, one repair |
| Publish | `tasks close --group` | Commit on `<type>/<name>`, push, PR, changelog line, status DONE |
| Report | `tasks report` | Per-task seconds, turns, tokens when the host reports them, findings; in the accessibility format |

### The run skill and the headless launcher

The `run` skill is about ten lines under 800 tokens: next, spawn one builder per task in the wave, close, spawn the reviewer, close the group, publish. The session judges only what to spawn and what a result says, so there is no prose state machine for it to re-read after compaction, which is what made the 0.x skill loop slow. `komodo run <group>` wraps the same skill in the host's non-interactive mode for unattended runs, with push credentials stripped from the environment first. The plan's first proof runs one group under V1 and under the skill and records wall time and tokens before anything is deleted.

### Context injection with budgets

A builder never guesses what it needs; `tasks brief` injects it. The slots are the task block, the repo's own `AGENTS.md`, repo context files matched to the task's files, the task's context anchors resolved to sections, the files themselves, the standards for the extensions and role, the acceptance commands, and on a repair the failed output plus the previous attempt's diff. Every slot has a cap in `config.py` and is clipped head-and-tail with a marker the model can see, so a brief is the same size on a large repo as on a small one. The reviewer gets the diff, the group's tasks, and the standards the diff touches, and nothing from the builder's transcript.

| Slot | Source | Default cap |
|---|---|---|
| task block | the task's YAML | none |
| repo rules | the repo's `AGENTS.md`, or a one-line default | 8k chars |
| repo context | `.komodo/context/*.md` whose globs match the task's files | 8k chars |
| context | the task's `context` anchors, resolved to sections | 10k per file, 24k total |
| files | the task's `files`, existing ones read | 10k per file, 24k total |
| standards | by extension and role, from the skills directory plus the repo layer | 6k per standard |
| done when | the task's commands | none |
| failure | the failed command output and the previous attempt's diff, on repair only | 80k |

### The repo layer

A repo may commit a `.komodo/` directory and nothing in it is required. It can also add a skill under `.komodo/skills/`, or append a "Repo overrides" section to a shipped one by using its name, and `komodo install --project` renders those into the host's project directory as gitignored copies. `.komodo/commands.json` names the verify, compile, before-review, and after-publish commands the line runs at those stations, and `.komodo/policy.json` adds protected refs, production markers, and denied patterns the guard merges under the machine floor. A repo can add and append; it can never replace or remove a shipped body or a denial. Context files under `.komodo/context/` carry a glob list in frontmatter and are injected into any task whose files match, so a payroll module's rules reach the builder touching payroll and no one else. `.komodo/standards/` extends a shipped standard or adds one the toolkit never shipped, in the same format with the same trigger. `.komodo/exclude` names shipped standards this repo never loads, with a floor of comments, API security, and SDLC it cannot reach, and doctor fails when an exclusion has drifted away from the code. Precedence is defaults, then a per-machine overlay that can only tighten, then the repo.

### Roles, tiers, and profiles

A role declares a tier, light, standard, or heavy, and never a model. A profile maps each tier to a provider, a model, and an effort for one host, and `komodo install --profile` picks it. The default profile runs Claude Code's three model sizes; `hybrid` sends light roles through the local bridge; `codex` and `local` do the same on Codex, with `local` running every tier on Ollama. Changing what a tier costs is a config edit that touches no role, and a role rendered for a new host reads the same words it read on the old one.

### Plan-aware pacing

The adapter probes the account's plan and its five-hour and seven-day usage windows from the host's own config file, never from a CLI status line that once misreported a Max account as Pro, and returns them in a neutral shape. A plan is a profile overlay: a ceiling per tier, a parallelism cap, a review floor, a diff size under which review is skipped, a repair count, and the window fractions at which to warn and to pause. `tasks next` applies the overlay and, when a window has passed its pause point, prints a wait time instead of a wave, so a run pauses before a wave and never inside one. On a Pro plan light roles prefer the local bridge; on Max the defaults hold; with no probe the conservative overlay applies and the run still starts.

### Release and tags

`komodo release check` audits the changelog against the tags and exits non-zero on drift, read-only. `komodo tasks tag` tags every changelog version no tag points at, annotated, and pushes it, on a clean base branch only. The next run's intake calls it, which is V1's preflight tag, so a version is tagged the first time anything runs after the human merges its PR. The guard allows creating and pushing a tag and denies deleting or force-moving one.

### Pull requests

Publish opens the PR with the report as its body: what landed, what blocked, timing, and findings, and it opens as a draft when a task is blocked. The category label comes from the config mapping and the agent label marks authorship, and both are applied only when the repo already defines them. `komodo pr` keeps thin wrappers for threads, label, comment, and reply, and the `respond` skill lets a session answer every unresolved review thread as the responder role, changing code when the reviewer is right. The V1 worker that merged the base in is gone; a conflict is the human's.

### Independent review

Every group gets one review pass from a reviewer with a fresh context that reads only the diff. It never sees the builder's transcript, its brief, or its reasoning, so it cannot inherit the builder's assumptions. The profile may place the reviewer on a different tier or a different vendor than the builder, including a local model, so a review need not share a provider with the build. Findings at or above the floor become one repair pass; the rest are filed to the backlog by code.

### Guardrails

One script, the guard, runs before every shell, edit, and write on every host, and it is the only enforcement point. It reads `policy.json` and denies pushes, commits, merges, and deletes on protected refs, force and amend and history rewrites, co-author and generated-by trailers, recursive deletes outside the worktree, database drops and truncates, infrastructure destroy verbs, and any command naming a production marker. Inside a worktree it also denies an edit or write outside the task's own file list. It fails open on an internal error so one bug never stalls every command, and `komodo guard check` runs a table of at least 60 commands inside verify so a broken guard fails the gate, never a run. The headless launcher strips push credentials on top, which is V1's structural guarantee kept for unattended work.

### Optional git hooks

Two git hooks in standard-library Python exist for a human terminal, since the guard already covers every agent. Pre-commit refuses a protected branch and a trailer and lints comments on staged lines; pre-push refuses a non-fast-forward and runs the repo's verify command under a timeout. `komodo hooks install` points a repo at a copy under that repo, never at the toolkit checkout, so a run that edits the toolkit cannot change the hook mid-run. Nothing installs them by default.

### Dry run and prune

`tasks brief --dry-run` prints every slot's size after clipping and a token estimate, and writes nothing, which is what V1's run dry-run did. `komodo doctor --prune` removes stale worktrees and deletes branches merged into base, which V1's status command did. Both exist so a human can see what a run would cost and clean up after one without a model call.

### Local models

Local models are a profile choice, not a code path. On Claude Code, which cannot run a subagent on Ollama, `komodo bridge` is a stdio MCP server in the standard library that the host spawns on demand; a light role's body calls it, so there is no process to keep alive and nothing to install beyond Ollama. On Codex the `local` profile points every tier at Ollama through the host's own provider setting. Both Komodo machines pull the same models, so a brief produces the same result on either desk.

### Standards as skills

Every language and domain standard is a `SKILL.md` whose description names the extensions and directories that trigger it. In a session the host loads it on demand; in a brief `tasks brief` injects the clipped copy for the extensions the task touches. The files stay between 3 and 8 KB, the validator caps them, and there is no "off" standard: an unused language costs nothing. A standard names no live repo, port, URL, version, or path.

### The accessibility contract

Every human-facing output follows one contract, rendered into the always-on rules: the answer first, one idea per line, five bullets per list, three sentences per paragraph, bold anchors, and a turn-end summary in fixed buckets when something changed. The run report follows the same contract, so a report and a chat reply read the same way. The contract is a rules file, so it travels to every host unchanged.

### Doctor and verify

`komodo doctor` fails on a backticked path or skill name that does not resolve, on a vendor name or host tool outside the adapters, on a rendered layout that has drifted from the source, and on a repo exclusion that no longer matches the code. `scripts/verify.py` is the gate: tests, the always-on token budget, the skill size caps, the comment lint, the guard table, and doctor. Nothing lands without it.

## Roadmap

Nine groups in `BACKLOG.md`, in order. V1 runs the first four; the run skill runs the rest.

| Group | Delivers | Proof |
|---|---|---|
| TG-03.1 Source reshape | Standards as skills, briefs folded into roles with schemas, the policy file, the rules with the accessibility contract, the merger role removed | Tests, no old directories |
| TG-03.2 Guard and inject | The two stdlib hooks, the 60-command table, `guard check` in verify, the Go binaries deleted, the git hooks rewritten in Python and opt-in | Verify runs the table |
| TG-03.3 Profiles, Claude adapter, CLI | Profiles with context caps and plan overlays, the Claude render with seeds and pointer skills, the plan probe, doctor with prune, the CLI | Validate under 1500 tokens |
| TG-03.4 The line in code | `tasks next` with start and pacing, `brief` with dry run, `close` with compile gate, labels, and draft PRs, `diff`, `report`, `tag`, `release check`; resume by results | Tests per command |
| TG-03.5 Run skill and launcher | The ten-line skill, review, backlog, and respond skills, `komodo run` with a wall-clock budget, the PR wrappers, the V1 versus V1.5 timing proof | One group each way, numbers recorded |
| TG-03.6 The repo layer | Context by glob, standard overrides, exclusions with a floor, repo skills with `install --project`, repo commands and additive policy | Tests, doctor |
| TG-03.7 Local models on Claude Code | The stdio bridge, its registration at install, the hybrid profile | Bridge tests with a fake Ollama |
| TG-03.8 Codex and the exit test | The Codex adapter, the portability lint, the local profile, one group under Codex | Zero source changes outside adapters |
| TG-03.9 Demolition and docs | The orchestrator, workers, state, account, and gitops deleted; README and templates rewritten; 1.5.0 | Verify passes with nothing left |

## V1 coverage

Every V1 capability, where it lands, or why it does not.

| V1 capability | V1.5 |
|---|---|
| `run` with waves, worktrees, merge, verify, review, publish | `tasks next`, `brief`, `close`, and the run skill, TG-03.4 and TG-03.5 |
| `run --dry-run` token estimates | `tasks brief --dry-run`, TSK-03.4.2 |
| `run --resume` | Result files on disk, TSK-03.4.1 |
| `mode: single` groups | Honored by `tasks next`, TSK-03.4.1 |
| Compile gate per wave with one repair | `tasks close --wave`, TSK-03.4.3 |
| Verify command discovery order | Kept, overridable by repo commands, TSK-03.4.3 and TSK-03.6.5 |
| Blocked task with note, wave continues | `tasks close`, TSK-03.4.3 |
| Review with severity floor, findings filed to the backlog | `tasks diff` and `tasks close`, TSK-03.4.3 |
| PR body sections, labels from the mapping, draft on block | `tasks close --group`, TSK-03.4.3 |
| Changelog entry per version | `tasks close --group`, TSK-03.4.3 |
| Preflight tag, `release check` | `tasks tag` and `release check`, TSK-03.4.5 |
| Clean-tree check and base resolution | `tasks next --start`, TSK-03.4.1 |
| Report: phases, per-role cost, summary buckets | `tasks report`, TSK-03.4.4; per-phase time becomes per-task time |
| Plan detection, model ceiling, turn caps | Plan probe and overlays, TSK-03.3.4 and TSK-03.4.6; turn caps are the host's |
| Rate-window pause and warn | `tasks next` waits before a wave, TSK-03.4.6 |
| Group wall-clock budget | The launcher, TSK-03.5.2 |
| `status --prune` and `--json` | `doctor --prune` and `--json`, TSK-03.3.5 |
| `tasks lint`, `list`, `add` | Kept, TSK-03.4.1 |
| `tasks plan` worker | The planner role through the backlog skill, TSK-03.5.1 |
| `tasks migrate` | Dropped; the pre-1.0 grammar has no repos left |
| `pr threads`, `label`, `comment`, `reply` | Kept as wrappers, TSK-03.5.4 |
| `pr respond` worker | The respond skill in a session, TSK-03.5.4 |
| `pr sync` worker and the merger role | Dropped; a conflict is the human's |
| `install` with seeds, `--dry-run`, copy on Windows | Kept, TSK-03.3.2; `--host` and `--project` added |
| `doctor`: references, policy leaks, leftovers, roles, changelog | Kept, TSK-03.3.5; portability and repo drift added in TSK-03.8.2 and TSK-03.6.3 |
| `comments check` | Kept, TSK-03.3.3 |
| `hooks install` and `status`, pre-commit and pre-push | Rewritten in Python, opt-in, TSK-03.2.5 |
| Go guard and Go inject | Python guard and inject, TSK-03.2.1 and TSK-03.2.2 |
| `gitops` refusals and the single pusher | The guard's policy and the launcher's scrub, TSK-03.2.1 and TSK-03.5.2 |
| `worker_env` credential scrub | The launcher, TSK-03.5.2 |
| Brief slots with clip and caps | `tasks brief`, TSK-03.4.2 |
| Standards by extension, clipped in briefs, pointers in sessions | TSK-03.1.1 and TSK-03.3.2 |
| Worker JSON validated against a schema | `tasks close`, TSK-03.4.3 |
| Ollama worker and the summarizer | The bridge and the hybrid profile, TG-03.7 |
| Profiles fast, thinking, local | claude, hybrid, codex, local, TSK-03.3.1 |
| Always-on token budget in validate | Kept, with skill caps, TSK-03.3.2 |
| Verify gate | Kept, with the guard table, TSK-03.2.3 |
| Project templates | Kept, TSK-03.9.2; the host rules file is rendered instead of templated |
| Personal overlay seed | Kept, TSK-03.3.2 |
| Per-worker dollar caps and timeouts | Dropped; a subscription never charges them and the host owns turns |
| Repo-level context, standards, exclusions | Built, TG-03.6; V1 planned them and never shipped |

## Setup

Requirements: Python 3.9+, git, `gh` authenticated, and the host CLI on PATH. Ollama is optional.

```bash
git clone <this repo> ~/komodo/ai/komodo-agentic-toolkit-coding
cd ~/komodo/ai/komodo-agentic-toolkit-coding
python3 -m komodo install --host claude       # or --host codex; add --profile hybrid for local light roles
```

The install is a copy. After editing anything under `komodo/`, run it again. `python3 -m komodo doctor` says when you forgot.

## Usage

In any repo with a `BACKLOG.md` in the task grammar:

```bash
/run TG-01.2                                  # in a session: the assembly line on one group
python3 -m komodo run TG-01.2                 # headless, credentials stripped
python3 -m komodo tasks next --json           # what would run, and why
python3 -m komodo tasks lint                  # after every backlog edit
python3 -m komodo doctor                      # references, portability, drift
python3 scripts/verify.py                     # the gate
```

## Layout

| Path | What |
|---|---|
| the rules file and `komodo/rules/` | Universal rules, the accessibility contract, the backlog grammar |
| `komodo/roles/` | One file per role: tier, tools, session flag, return schema, brief template |
| `komodo/skills/` | `run`, `review`, `backlog`, and one `standards-<x>` per language or domain |
| the policy file and `komodo/hooks/` | What no agent may do, and the two scripts that enforce it |
| `komodo/adapters/` | One render per host |
| `komodo/tasks.py` and the line, repo layer, bridge, and launcher modules | The line, the repo layer, the local bridge, the launcher |
| `tests/`, `scripts/verify.py` | The gate |
| `templates/project/` | Starters for a new repo |

